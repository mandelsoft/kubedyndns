/*
 * Copyright 2025 Mandelsoft. All rights reserved.
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

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/etcd/msg"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/request"
	"github.com/mandelsoft/kubedyndns/plugin/kubedyndns/objects"
	"github.com/miekg/dns"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ZoneInfo struct {
	DomainName string
	Object     *objects.Zone
}

func NewZoneInfo(domain string, zo *objects.Zone) *ZoneInfo {
	return &ZoneInfo{DomainName: domain, Object: zo}
}

func (i *ZoneInfo) Match(ref string, e metav1.Object) bool {
	if i.Object == nil {
		return true
	}
	return e.GetNamespace() == i.Object.Namespace && ref == i.Object.Name
}

type Backend struct {
	*KubeDynDNS
	zoneInfo *ZoneInfo
}

var _ plugin.ServiceBackend = (*Backend)(nil)

func (k *Backend) Handle(ctx context.Context, state request.Request) ([]dns.RR, []dns.RR, error) {
	var (
		err     error
		records []dns.RR
		extra   []dns.RR
	)

	zi := k.zoneInfo

	switch state.QType() {
	case dns.TypeANY:
		var r []dns.RR
		records, _, err = plugin.A(ctx, k, zi.DomainName, *withType(&state, dns.TypeA), nil, plugin.Options{})

		r, _, err = plugin.AAAA(ctx, k, zi.DomainName, *withType(&state, dns.TypeAAAA), nil, plugin.Options{})
		records = append(records, r...)

		r, _, err = plugin.TXT(ctx, k, zi.DomainName, *withType(&state, dns.TypeTXT), nil, plugin.Options{})
		records = append(records, r...)

		r, err = plugin.CNAME(ctx, k, zi.DomainName, *withType(&state, dns.TypeCNAME), plugin.Options{})
		records = append(records, r...)

		r, extra, err = plugin.SRV(ctx, k, zi.DomainName, *withType(&state, dns.TypeSRV), plugin.Options{})
		records = append(records, r...)

		if state.Name() == zi.DomainName {
			r, extra, err = plugin.NS(ctx, k, zi.DomainName, *withType(&state, dns.TypeNS), plugin.Options{})
			records = append(records, r...)
		}

	case dns.TypeA:
		records, _, err = plugin.A(ctx, k, zi.DomainName, state, nil, plugin.Options{})
	case dns.TypeAAAA:
		records, _, err = plugin.AAAA(ctx, k, zi.DomainName, state, nil, plugin.Options{})
	case dns.TypeTXT:
		records, _, err = plugin.TXT(ctx, k, zi.DomainName, state, nil, plugin.Options{})
	case dns.TypeCNAME:
		records, err = plugin.CNAME(ctx, k, zi.DomainName, state, plugin.Options{})
	case dns.TypePTR:
		records, err = plugin.PTR(ctx, k, zi.DomainName, state, plugin.Options{})
	case dns.TypeMX:
		records, extra, err = plugin.MX(ctx, k, zi.DomainName, state, plugin.Options{})
	case dns.TypeSRV:
		records, extra, err = plugin.SRV(ctx, k, zi.DomainName, state, plugin.Options{})
	case dns.TypeSOA:
		records, err = k.SOA(ctx, zi, state), nil

	case dns.TypeNS:
		if state.Name() == zi.DomainName {
			records, extra, err = plugin.NS(ctx, k, zi.DomainName, state, plugin.Options{})
			break
		}
		fallthrough
	default:
		// Do a fake A lookup, so we can distinguish between NODATA and NXDOMAIN
		fake := state.NewWithQuestion(state.QName(), dns.TypeA)
		fake.Zone = state.Zone
		_, _, err = plugin.A(ctx, k, zi.DomainName, fake, nil, plugin.Options{})
	}

	return records, extra, err
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// Services implements the ServiceBackend interface.
func (k *Backend) Services(ctx context.Context, state request.Request, exact bool, opt plugin.Options) (svcs []msg.Service, err error) {

	switch state.QType() {
	case dns.TypeNS:
		// We can only get here if the qname equals the zone, see ServeDNS in handler.go.
		nss := k.nsAddrs(false, state.Zone)
		var svcs []msg.Service
		for _, ns := range nss {
			if ns.Header().Rrtype == dns.TypeA {
				svcs = append(svcs, msg.Service{Host: ns.(*dns.A).A.String(), Key: msg.Path(ns.Header().Name, coredns), TTL: k.ttl})
				continue
			}
			if ns.Header().Rrtype == dns.TypeAAAA {
				svcs = append(svcs, msg.Service{Host: ns.(*dns.AAAA).AAAA.String(), Key: msg.Path(ns.Header().Name, coredns), TTL: k.ttl})
			}
		}
		return svcs, nil
	}

	s, e := k.Records(ctx, state, false)
	return s, e
}

// Records looks up services in kubernetes.
func (k *Backend) Records(ctx context.Context, state request.Request, exact bool) ([]msg.Service, error) {
	r, e := parseRequest(state.Name(), state.Zone)
	if e != nil {
		return nil, e
	}
	if dnsutil.IsReverse(state.Name()) > 0 {
		return nil, errNoItems
	}

	if r.IsServiceRequest() != (state.QType() == dns.TypeSRV) {
		return nil, errNoItems
	}
	services, err := k.findEntries(r, state.QType())
	return services, err
}

// findServices returns the services matching r from the cache.
func (k *Backend) findEntries(r *recordRequest, t uint16) (services []msg.Service, err error) {
	var entries []*objects.Entry

	zi := k.zoneInfo
	if k.filtered {
		entries = k.APIConn.EntryDNSIndex(r.domain + "." + zi.DomainName)
		Log.Infof("find (filtered) %s.%s -> %d entries", r.domain, zi.DomainName, len(entries))
	} else {
		tmp := k.APIConn.EntryDNSIndex(r.domain + ".")
		for _, e := range tmp {
			if zi.Match(e.ZoneRef, e) {
				entries = append(entries, e)
			}
		}
		Log.Infof("find %s. -> %d entries -> %d in %s<%s>", r.domain, len(tmp), len(entries), k.zoneObject, zi.DomainName)
	}
	if len(entries) == 0 {
		return nil, errNoItems
	}

	if r.service != "" && r.service != "any" && r.service != "all" {
		for _, e := range entries {
			if e.Service.Service == r.service {
				for _, s := range e.Services(t, r.protocol, k.ttl, zi.DomainName) {
					services = append(services, s)
				}
			}
		}
	} else {
		for _, e := range entries {
			if e.MatchType(t) {
				services = append(services, e.Services(t, "", k.ttl, zi.DomainName)...)
			}
		}
	}

	return services, nil
}

////////////////////////////////////////////////////////////////////////////////

// Lookup implements the ServiceBackend interface.
func (k *Backend) Lookup(ctx context.Context, state request.Request, name string, typ uint16) (*dns.Msg, error) {
	return k.Upstream.Lookup(ctx, state, name, typ)
}
