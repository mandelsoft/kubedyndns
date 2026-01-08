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

package kubedyndns

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/plugin/pkg/upstream"
	"github.com/pkg/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"  // pull this in here, because we want it excluded if plugin.cfg doesn't have k8s
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc" // pull this in here, because we want it excluded if plugin.cfg doesn't have k8s
	"k8s.io/client-go/tools/cache"

	// _ "k8s.io/client-go/plugin/pkg/client/auth/openstack" // pull this in here, because we want it excluded if plugin.cfg doesn't have k8s
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"

	"github.com/mandelsoft/kubedyndns/plugin/kubedyndns/objects"
)

const pluginName = "kubedyndns"

var Log = clog.NewWithPlugin(pluginName)

func init() { objects.Log = Log; plugin.Register(pluginName, setup) }

func setup(c *caddy.Controller) error {
	Log.Infof("setup kubedyndns plugin")
	klog.SetOutput(os.Stdout)

	ks, err := parse(c)
	if err != nil {
		return plugin.Error(pluginName, err)
	}

	// link plugins into processing chain
	for i := range ks {
		k := ks[i]

		if i == len(ks)-1 {
			// last forward: point next to next plugin
			dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
				k.Next = next
				return k
			})
		} else {
			// middle forward: point next to next forward
			nextForward := ks[i+1]
			dnsserver.GetConfig(c).AddPlugin(func(plugin.Handler) plugin.Handler {
				k.Next = nextForward
				return k
			})
		}
	}

	err = ks[0].InitKubeCache(context.Background())
	if err != nil {
		return plugin.Error(pluginName, err)
	}

	ks[0].RegisterKubeCache(c)

	// get locally bound addresses
	c.OnStartup(func() error {
		localIPs := boundIPs(c)
		for i := range ks {
			ks[i].localIPs = localIPs
		}
		return nil
	})

	return nil
}

// RegisterKubeCache registers KubeCache start and stop functions with Caddy
func (k *KubeDynDNS) RegisterKubeCache(c *caddy.Controller) {
	c.OnStartup(func() error {
		go k.APIConn.Run()

		timeout := time.After(5 * time.Second)
		ticker := time.NewTicker(100 * time.Millisecond)
		for {
			select {
			case <-ticker.C:
				if k.APIConn.HasSynced() {
					return nil
				}
			case <-timeout:
				return nil
			}
		}
	})

	c.OnShutdown(func() error {
		return k.APIConn.Stop()
	})
}

func parse(c *caddy.Controller) ([]*KubeDynDNS, error) {
	var (
		kc *K8SConfig
		r  []*KubeDynDNS
	)

	singleton := ""
	i := 0
	for c.Next() {
		i++

		k8s, err := ParseStanza(c, kc)
		if err != nil {
			return nil, err
		}
		if len(k8s.namespaces) == 0 {
			singleton = "using at least one instance for all namespaces"
		}
		kc = k8s.assureK8SConfig()
		k8s.filtered = k8s.Mode == MODE_FILTER
		r = append(r, k8s)
	}
	if singleton != "" && len(r) > 1 {
		return nil, fmt.Errorf("this plugin can only be used once per Server Block: %s", singleton)
	}
	return r, nil
}

// ParseStanza parses a kubedyndns stanza
func ParseStanza(c *caddy.Controller, kc *K8SConfig) (*KubeDynDNS, error) {
	var err error

	k8s := New([]string{""})

	zones := c.RemainingArgs()

	if len(zones) != 0 {
		k8s.Zones = zones
		for i := 0; i < len(k8s.Zones); i++ {
			hosts := plugin.Host(k8s.Zones[i]).NormalizeExact()
			if hosts == nil {
				return nil, fmt.Errorf("no hosts found in zone %s", k8s.Zones[i])
			}
			k8s.Zones[i] = hosts[0]
		}
	} else {
		k8s.Zones = make([]string, len(c.ServerBlockKeys))
		for i := 0; i < len(c.ServerBlockKeys); i++ {
			hosts := plugin.Host(c.ServerBlockKeys[i]).NormalizeExact()
			if hosts == nil {
				return nil, fmt.Errorf("no hosts found in zone %s", k8s.Zones[i])
			}
			k8s.Zones[i] = hosts[0]
		}
	}

	for _, z := range k8s.Zones {
		if dnsutil.IsReverse("."+z) > 0 {
			continue
		}
		k8s.ServedZones = append(k8s.ServedZones, z)
	}

	k8s.Upstream = upstream.New()

	for c.NextBlock() {
		switch c.Val() {
		case "mode":
			args := c.RemainingArgs()
			if len(args) > 0 {
				if len(args) > 1 {
					return nil, fmt.Errorf("Multiple modes not possible")
				}
				switch args[0] {
				case MODE_FILTER, MODE_SUBDOMAINS, MODE_PRIMARY:
					k8s.Mode = args[0]
				default:
					return nil, fmt.Errorf("Invalid mode %q, use %s, %s or %s", args[0], MODE_FILTER, MODE_SUBDOMAINS, MODE_PRIMARY)
				}
				continue
			}
			return nil, c.ArgErr()
		case "zoneobject":
			args := c.RemainingArgs()
			if len(args) > 0 {
				if len(args) != 1 {
					return nil, c.Errf("Zone Object requires name")
				}
				k8s.zoneObject = args[0]
				continue
			}
			return nil, c.ArgErr()
		case "namespaces":
			args := c.RemainingArgs()
			if len(args) > 0 {
				for _, a := range args {
					k8s.namespaces[a] = struct{}{}
				}
				continue
			}
			return nil, c.ArgErr()
		case "endpoint":
			args := c.RemainingArgs()
			if len(args) > 0 {
				// Multiple endpoints are deprecated but still could be specified,
				// only the first one be used, though
				k8s.assureK8SConfig().APIServerList = args
				if len(args) > 1 {
					return nil, c.Errf("Multiple endpoints not possible")
				}
				continue
			}
			return nil, c.ArgErr()
		case "token":
			args := c.RemainingArgs()
			switch len(args) {
			case 2:
				k8s.assureK8SConfig().APICertAuth = args[1]
				k8s.assureK8SConfig().APIToken = args[0]
			case 1:
				if isDirectory(args[0]) {
					if fileExists(filepath.Join(args[0], "token")) {
						k8s.assureK8SConfig().APIToken = filepath.Join(args[0], "token")
					} else {
						return nil, c.Errf("no token file found")
					}
					if fileExists(filepath.Join(args[0], "ca.crt")) {
						k8s.assureK8SConfig().APICertAuth = filepath.Join(args[0], "ca.crt")
					}
				} else {
					k8s.assureK8SConfig().APIToken = args[0]
				}
			default:
				return nil, c.ArgErr()
			}
		case "tls": // cert key cacertfile
			args := c.RemainingArgs()
			if len(args) == 3 {
				kc := k8s.assureK8SConfig()
				kc.APIClientCert, kc.APIClientKey, kc.APICertAuth = args[0], args[1], args[2]
				continue
			}
			return nil, c.ArgErr()
		case "labels":
			args := c.RemainingArgs()
			if len(args) > 0 {
				labelSelectorString := strings.Join(args, " ")
				ls, err := meta.ParseToLabelSelector(labelSelectorString)
				if err != nil {
					return nil, c.Errf("unable to parse label selector value: '%v': %v", labelSelectorString, err)
				}
				k8s.assureK8SConfig().labelSelector = ls
				continue
			}
			return nil, c.ArgErr()
		case "fallthrough":
			k8s.Fall.SetZonesFromArgs(c.RemainingArgs())
		case "ttl":
			args := c.RemainingArgs()
			if len(args) == 0 {
				return nil, c.ArgErr()
			}
			t, err := strconv.Atoi(args[0])
			if err != nil {
				return nil, err
			}
			if t < 0 || t > 3600 {
				return nil, c.Errf("ttl must be in range [0, 3600]: %d", t)
			}
			k8s.ttl = uint32(t)
		case "kubeconfig":
			args := c.RemainingArgs()
			override := &clientcmd.ConfigOverrides{}
			switch len(args) {
			case 2:
				override.CurrentContext = args[1]
			case 1:
			default:
				return nil, c.ArgErr()
			}
			config := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
				&clientcmd.ClientConfigLoadingRules{ExplicitPath: args[0]},
				override,
			)
			k8s.assureK8SConfig().ClientConfig = config
			continue
		case "transitive":
			args := c.RemainingArgs()
			switch len(args) {
			case 0:
				k8s.transitive = true
			case 1:
				k8s.transitive, err = strconv.ParseBool(args[0])
				if err != nil {
					return nil, err
				}
			default:
				return nil, c.ArgErr()
			}
		case "slave":
			args := c.RemainingArgs()
			switch len(args) {
			case 0:
				k8s.slave = true
			case 1:
				k8s.slave, err = strconv.ParseBool(args[0])
				if err != nil {
					return nil, err
				}
			default:
				return nil, c.ArgErr()
			}
		default:
			return nil, c.Errf("unknown property '%s'", c.Val())
		}
	}

	if k8s.Mode == MODE_PRIMARY {
		if k8s.zoneObject == "" {
			return nil, c.Errf("zoneObject required for mode %q", k8s.Mode)
		}

		if len(k8s.namespaces) != 1 {
			return nil, c.Errf("one namespace required for zoneObject for mode %q", k8s.Mode)
		}

		ns := ""
		for n := range k8s.namespaces {
			ns = n
		}

		k8s.zoneRef = &cache.ObjectName{Name: k8s.zoneObject, Namespace: ns}
		k8s.k8s.Namespaces[k8s.zoneRef.Namespace] = struct{}{}

	} else {
		if k8s.zoneObject != "" {
			return nil, c.Errf("zoneObject requires mode %q", MODE_PRIMARY)
		}
	}

	if kc != nil && k8s.k8s != nil {
		return nil, c.Errf("kubernetes config is possible only once at first instance in server block")
	}

	k8s.assureK8SConfig().Namespaces[k8s.zoneRef.Namespace] = struct{}{}

	if k8s.Mode != MODE_FILTER {
		if len(k8s.ServedZones) != 1 {
			return nil, c.Errf("Mode %s requires one served zone as base domain", k8s.Mode)
		}
	}
	return k8s, nil
}

func isDirectory(path string) bool {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false
	}

	return fileInfo.IsDir()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true // File exists
	}
	if errors.Is(err, os.ErrNotExist) {
		return false // File specifically does not exist
	}
	// The file might exist, but we have a permission error
	// or another issue. In most cases, you treat this as false.
	return false
}
