/*
 * Copyright 2021 Mandelsoft. All rights reserved.
 *  This file is licensed under the Apache Software License, v. 2 except as noted
 *  otherwise in the LICENSE file
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *       http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

// Package kubernetes provides the kubernetes backend.
package kubedyndns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/fall"
	"github.com/coredns/coredns/plugin/pkg/upstream"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/cert"

	clientapi "github.com/mandelsoft/kubedyndns/client/clientset/versioned"
)

const MODE_FILTER = "FilterByZones"
const MODE_SUBDOMAINS = "Subdomains"
const MODE_PRIMARY = "Primary"

type K8SConfig struct {
	APIServerList []string
	APIToken      string
	APICertAuth   string
	APIClientCert string
	APIClientKey  string
	ClientConfig  clientcmd.ClientConfig
	Namespaces    map[string]struct{}
	// Label handling.
	labelSelector *meta.LabelSelector
	selector      labels.Selector
}

// KubeDynDNS implements a plugin that connects to a Kubernetes cluster.
type KubeDynDNS struct {
	Next        plugin.Handler
	Mode        string
	Zones       []string
	transitive  bool
	ServedZones []string
	Upstream    *upstream.Upstream
	APIConn     Controller
	Fall        fall.F
	ttl         uint32
	k8s         *K8SConfig
	controlOpts
	localIPs []net.IP
}

// New returns a initialized Kubernetes. It default interfaceAddrFunc to return 127.0.0.1. All other
// values default to their zero value, primaryZoneIndex will thus point to the first zone.
func New(zones []string) *KubeDynDNS {
	k := new(KubeDynDNS)
	k.Zones = zones
	k.ttl = defaultTTL
	k.Mode = MODE_FILTER
	k.namespaces = make(map[string]struct{})
	return k
}

func (k *KubeDynDNS) assureK8SConfig() *K8SConfig {
	if k.k8s == nil {
		k.k8s = &K8SConfig{
			Namespaces: make(map[string]struct{}),
		}
	}
	return k.k8s
}

const (
	// DNSSchemaVersion is the schema version: https://github.com/kubernetes/dns/blob/master/docs/specification.md
	DNSSchemaVersion = "1.1.0"
	// defaultTTL to apply to all answers.
	defaultTTL = 10
)

var (
	errNoItems        = errors.New("no items found")
	errNsNotExposed   = errors.New("namespace is not exposed")
	errInvalidRequest = errors.New("invalid query name")
)

func (k *K8SConfig) getClientConfig() (*rest.Config, error) {
	log.Infof("retrieving Kubernetes client config")
	if k != nil {
		if k.ClientConfig != nil && k.APIToken != "" {
			return nil, fmt.Errorf("only API token or kubeconfig")
		}
		if k.APIToken != "" && len(k.APIServerList) == 0 {
			return nil, fmt.Errorf("API token requires API server")
		}
	}

	if k != nil && k.ClientConfig != nil {
		log.Infof("using explicit config")
		return k.ClientConfig.ClientConfig()
	}
	loadingRules := &clientcmd.ClientConfigLoadingRules{}
	overrides := &clientcmd.ConfigOverrides{}
	clusterinfo := clientcmdapi.Cluster{}
	authinfo := clientcmdapi.AuthInfo{}

	// Connect to API from in cluster
	if k == nil || len(k.APIServerList) == 0 {
		log.Infof("using in-cluster config")
		cc, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
		// cc.ContentType = "application/vnd.kubernetes.protobuf"
		return cc, err
	}

	if k.APIToken != "" {
		cfg, err := TokenConfig(k.APIServerList[0], k.APIToken, k.APICertAuth)
		if err != nil {
			return nil, err
		}
		return cfg, nil
	}
	// Connect to API from out of cluster
	// Only the first one is used. We will deprecate multiple endpoints later.
	clusterinfo.Server = k.APIServerList[0]

	if len(k.APICertAuth) > 0 {
		clusterinfo.CertificateAuthority = k.APICertAuth
	}
	if len(k.APIClientCert) > 0 {
		authinfo.ClientCertificate = k.APIClientCert
	}
	if len(k.APIClientKey) > 0 {
		authinfo.ClientKey = k.APIClientKey
	}

	overrides.ClusterInfo = clusterinfo
	overrides.AuthInfo = authinfo
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)

	cc, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	cc.ContentType = "application/vnd.kubernetes.protobuf"
	return cc, err

}

// InitKubeCache initializes a new Kubernetes cache.
func (k *KubeDynDNS) InitKubeCache(ctx context.Context) (err error) {
	config, err := k.k8s.getClientConfig()
	if err != nil {
		return err
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes notification controller: %q", err)
	}
	apiClient, err := clientapi.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes notification controller: %q", err)
	}

	if k.k8s.labelSelector != nil {
		var selector labels.Selector
		selector, err = meta.LabelSelectorAsSelector(k.k8s.labelSelector)
		if err != nil {
			return fmt.Errorf("unable to create Selector for LabelSelector '%s': %q", k.k8s.labelSelector, err)
		}
		k.k8s.selector = selector
	}

	log.Infof("using mode %s: %v", k.Mode, k.ServedZones)
	k.APIConn = newController(ctx, kubeClient, apiClient, k.controlOpts)

	return err
}

func (k *KubeDynDNS) TTL(ttl uint32) uint32 {
	if ttl > 0 {
		return ttl
	}
	if k.ttl > 0 {
		return k.ttl
	}
	return 300
}

// match checks if a and b are equal taking wildcards into account.
func match(a, b string) bool {
	if wildcard(a) {
		return true
	}
	if wildcard(b) {
		return true
	}
	return strings.EqualFold(a, b)
}

// wildcard checks whether s contains a wildcard value defined as "*" or "any".
func wildcard(s string) bool {
	return s == "*" || s == "any"
}

const coredns = "c" // used as a fake key prefix in msg.Service

// BackendError writes an error response to the client.
func (k *KubeDynDNS) BackendError(ctx context.Context, zi *ZoneInfo, rcode int, state request.Request, err error) (int, error) {
	m := new(dns.Msg)
	m.SetRcode(state.Req, rcode)
	m.Authoritative = true
	m.Ns = k.SOA(ctx, zi, state)

	state.W.WriteMsg(m)
	// Return success as the rcode to signal we have written to the client.
	return dns.RcodeSuccess, err
}

////////////////////////////////////////////////////////////////////////////////
// general backend methods

// IsNameError implements the ServiceBackend interface.
func (k *KubeDynDNS) IsNameError(err error) bool {
	return err == errNoItems || err == errNsNotExposed || err == errInvalidRequest
}

// Serial return the SOA serial.
func (k *KubeDynDNS) Serial(state request.Request) uint32 { return uint32(k.APIConn.Modified()) }

// MinTTL returns the minimal TTL.
func (k *KubeDynDNS) MinTTL(state request.Request) uint32 { return k.ttl }

func TokenConfig(server, tokenFile, rootCAFile string) (*rest.Config, error) {
	const (
		def_tokenFile  = "/var/run/secrets/kubernetes.io/serviceaccount/token"
		def_rootCAFile = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	)

	if tokenFile == "" {
		tokenFile = def_tokenFile
	}
	if rootCAFile == "" {
		rootCAFile = def_rootCAFile
	}

	token, err := os.ReadFile(tokenFile)
	if err != nil {
		return nil, err
	}

	tlsClientConfig := rest.TLSClientConfig{}

	if _, err := cert.NewPool(rootCAFile); err != nil {
		return nil, err
	} else {
		tlsClientConfig.CAFile = rootCAFile
	}

	return &rest.Config{
		// TODO: switch to using cluster DNS.
		Host:            server,
		TLSClientConfig: tlsClientConfig,
		BearerToken:     string(token),
		BearerTokenFile: tokenFile,
	}, nil
}
