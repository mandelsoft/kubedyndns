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
	"slices"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/request"
	"github.com/mandelsoft/kubedyndns/plugin/kubedyndns/objects"
	"github.com/miekg/dns"
)

func withType(req *request.Request, t uint16) *request.Request {
	if req.Req == nil || len(req.Req.Question) == 0 {
		return req
	}
	n := *req
	in := *(req.Req)
	n.Req = &in
	in.Question = slices.Clone(in.Question)
	in.Question[0].Qtype = t
	return &n
}

// Name implements the Handler interface.
func (k *KubeDynDNS) Name() string {
	return "kubernetes"
}

// ServeDNS implements the plugin.Handler interface.
func (k *KubeDynDNS) ServeDNS(ctx context.Context, w dns.ResponseWriter, in *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: in}

	qname := state.QName()
	zone := plugin.Zones(k.Zones).Matches(qname)
	if zone == "" {
		return plugin.NextOrFailure(k.Name(), k.Next, ctx, w, in)
	}

	var zo *objects.Zone
	if k.zoneRef != nil {
		zo = k.APIConn.GetZone(*k.zoneRef)
	}

	var zones []string
	if zone == "." {
		zones = zo.DomainNames
	} else {
		for _, z := range zo.DomainNames {
			zones = append(zones, z+zone)
		}
	}

	if zo != nil {
		zone = plugin.Zones(zones).Matches(qname)
	}
	if zone == "" {
		return plugin.NextOrFailure(k.Name(), k.Next, ctx, w, in)
	}

	zone = qname[len(qname)-len(zone):] // maintain case of original query
	state.Zone = zone

	var (
		records []dns.RR
		auth    []dns.RR
		extra   []dns.RR
		err     error
	)

	zi := NewZoneInfo(zone, zo)

	dz, rs, zn := k.findZone(zi, qname)
	if rs != nil {
		for _, r := range rs {
			for _, s := range r.NS {
				auth = append(auth, k.NS(s, qname, r.Ttl)...)
			}
			if len(auth) == 0 {
				auth = append(auth, k.NS("ns."+qname, qname, r.Ttl)...)
			}
		}
	} else {
		if dz != nil {
			if zi != dz {
				if k.transitive && !(state.QType() == dns.TypeNS && qname == zn) {
					zi = dz
					state.Zone = zn
				} else {
					for _, s := range dz.Object.Status.NameServers {
						auth = append(auth, k.NS(s, qname, uint32(dz.Object.MinimumTTL))...)
					}
					if len(auth) == 0 {
						auth = append(auth, k.NS("ns."+qname, qname, uint32(dz.Object.MinimumTTL))...)
					}
				}
			}
		}
		if len(auth) == 0 {
			// we need the ZoneInfo in the Records method, but it cannot be passed
			// through the intermediate coredns calls.
			// therefore we create a delegate containing this information per request
			// which implements the required plugin.ServiceBackend interface in combination
			// with the general methods of the KubeDynDNS object.
			be := &Backend{zoneInfo: zi, KubeDynDNS: k}
			records, extra, err = be.Handle(ctx, state)
		} else {
			if state.QType() == dns.TypeNS {
				records = auth
				auth = nil
			}
		}
	}

	if k.IsNameError(err) {
		if k.Fall.Through(state.Name()) {
			return plugin.NextOrFailure(k.Name(), k.Next, ctx, w, in)
		}
		if !k.APIConn.HasSynced() {
			// If we haven't synchronized with the kubernetes cluster, return server failure
			return k.BackendError(ctx, zi, dns.RcodeServerFailure, state, nil)
		}
		return k.BackendError(ctx, zi, dns.RcodeNameError, state, nil)
	}
	if err != nil {
		return dns.RcodeServerFailure, err
	}

	m := new(dns.Msg)
	m.SetReply(in)
	m.Authoritative = true
	m.Answer = append(m.Answer, records...)
	m.Extra = append(m.Extra, extra...)
	if len(m.Answer) == 0 {
		if len(auth) > 0 {
			m.Ns = auth
		} else {
			m.Ns = k.SOA(ctx, zi, state)
		}
	}

	w.WriteMsg(m)
	return dns.RcodeSuccess, nil
}

func (k *KubeDynDNS) findZone(zi *ZoneInfo, qname string) (*ZoneInfo, []*objects.Entry, string) {
	qname = dns.Fqdn(qname)
	zn := zi.DomainName
	sub := qname[:len(qname)-len(zn)]
	labels := dns.SplitDomainName(sub)
	cur := zn
	rel := "."
	for i := range labels {
		l := labels[len(labels)-1-i]
		cur = dnsutil.Join(l, cur)
		rel = dnsutil.Join(l, rel)
		Log.Infof("lookup nested zone for %s/%s\n", cur, rel)
		var ns []*objects.Entry
		for _, e := range k.APIConn.EntryDNSIndex(rel) {
			if zi.Match(e.ZoneRef, e) {
				if len(e.NS) != 0 {
					ns = append(ns, e)
					zn = cur
					break
				}
			}
		}
		if ns != nil {
			Log.Infof("found delegated zone for %s: %s<%s>\n", cur, ns[0].Name, rel)
			return nil, ns, cur
		}
		for _, e := range k.APIConn.ZoneDomainIndex(rel) {
			if zi.Match(e.ParentRef, e) {
				Log.Infof("found nested zone for %s: %s<%s>\n", cur, e.Name, rel)
				zn = cur
				rel = "."
				zi = NewZoneInfo(zn, e)
				if !k.transitive {
					return zi, nil, zi.DomainName
				}
				break
			}
		}
	}

	return zi, nil, zi.DomainName
}

func (k *KubeDynDNS) SOA(ctx context.Context, zi *ZoneInfo, state request.Request) []dns.RR {
	if zi != nil {
		ttl := uint32(min(zi.Object.MinimumTTL, 300))
		header := dns.RR_Header{Name: zi.DomainName, Rrtype: dns.TypeSOA, Ttl: ttl, Class: dns.ClassINET}

		nsrv := dnsutil.Join("ns.dns", zi.DomainName)
		if len(zi.Object.Status.NameServers) > 0 {
			nsrv = dns.Fqdn(zi.Object.Status.NameServers[0])
		}
		mbox := dnsutil.Join("hostmaster", zi.DomainName)
		if zi.Object.EMail != "" {
			mbox = zi.Object.EMail
		}
		soa := &dns.SOA{Hdr: header,
			Mbox:    mbox,
			Ns:      nsrv,
			Serial:  k.Serial(state),
			Refresh: uint32(zi.Object.Refresh),
			Retry:   uint32(zi.Object.Retry),
			Expire:  uint32(zi.Object.Expire),
			Minttl:  ttl,
		}
		return []dns.RR{soa}
	} else {
		minTTL := k.MinTTL(state)
		ttl := min(minTTL, uint32(300))

		header := dns.RR_Header{Name: zi.DomainName, Rrtype: dns.TypeSOA, Ttl: ttl, Class: dns.ClassINET}

		Mbox := dnsutil.Join("hostmaster", zi.DomainName)
		Ns := dnsutil.Join("ns.dns", zi.DomainName)

		soa := &dns.SOA{Hdr: header,
			Mbox:    Mbox,
			Ns:      Ns,
			Serial:  k.Serial(state),
			Refresh: 7200,
			Retry:   1800,
			Expire:  86400,
			Minttl:  minTTL,
		}
		return []dns.RR{soa}
	}
}

func (k *KubeDynDNS) NS(nsrv, name string, ttl uint32) []dns.RR {
	return []dns.RR{&dns.NS{Hdr: dns.RR_Header{Name: name, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: k.TTL(ttl)}, Ns: nsrv}}
}
