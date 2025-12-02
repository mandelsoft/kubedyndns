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
	"k8s.io/client-go/tools/cache"
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

// ServeDNS implements the plugin.Handler interface.
func (k *KubeDynDNS) ServeDNS(ctx context.Context, w dns.ResponseWriter, in *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: in}

	qname := state.QName()
	zone := plugin.Zones(k.Zones).Matches(qname)
	if zone == "" {
		return plugin.NextOrFailure(k.Name(), k.Next, ctx, w, in)
	}
	zo := k.APIConn.GetZone(k.zoneRef)
	zone = qname[len(qname)-len(zone):] // maintain case of original query
	state.Zone = zone

	var (
		records []dns.RR
		auth    []dns.RR
		extra   []dns.RR
		err     error
	)

	dz, rs, zn := k.findZone(zo, qname)
	if rs != nil {
		for _, r := range rs {
			for _, s := range r.NS {
				auth = append(auth, k.NS(s, qname, r.Ttl, state)...)
			}
			if len(auth) == 0 {
				auth = append(auth, k.NS("ns."+qname, qname, r.Ttl, state)...)
			}
		}
	} else {
		if dz != nil {
			if zo != dz {
				if k.transitive && !(state.QType() == dns.TypeNS && qname == zn) {
					zo = dz
				} else {
					for _, s := range dz.NameServers {
						auth = append(auth, k.NS(s, qname, uint32(dz.MinimumTTL), state)...)
					}
					if len(auth) == 0 {
						auth = append(auth, k.NS("ns."+qname, qname, uint32(dz.MinimumTTL), state)...)
					}
				}
			}
		}
		if len(auth) == 0 {
			switch state.QType() {
			case dns.TypeANY:
				var r []dns.RR
				records, _, err = plugin.A(ctx, k, zone, *withType(&state, dns.TypeA), nil, plugin.Options{})

				r, _, err = plugin.AAAA(ctx, k, zone, *withType(&state, dns.TypeAAAA), nil, plugin.Options{})
				records = append(records, r...)

				r, _, err = plugin.TXT(ctx, k, zone, *withType(&state, dns.TypeTXT), nil, plugin.Options{})
				records = append(records, r...)

				r, err = plugin.CNAME(ctx, k, zone, *withType(&state, dns.TypeCNAME), plugin.Options{})
				records = append(records, r...)

				r, extra, err = plugin.SRV(ctx, k, zone, *withType(&state, dns.TypeSRV), plugin.Options{})
				records = append(records, r...)

				if state.Name() == zone {
					r, extra, err = plugin.NS(ctx, k, zone, *withType(&state, dns.TypeNS), plugin.Options{})
					records = append(records, r...)
				}

			case dns.TypeA:
				records, _, err = plugin.A(ctx, k, zone, state, nil, plugin.Options{})
			case dns.TypeAAAA:
				records, _, err = plugin.AAAA(ctx, k, zone, state, nil, plugin.Options{})
			case dns.TypeTXT:
				records, _, err = plugin.TXT(ctx, k, zone, state, nil, plugin.Options{})
			case dns.TypeCNAME:
				records, err = plugin.CNAME(ctx, k, zone, state, plugin.Options{})
			case dns.TypePTR:
				records, err = plugin.PTR(ctx, k, zone, state, plugin.Options{})
			case dns.TypeMX:
				records, extra, err = plugin.MX(ctx, k, zone, state, plugin.Options{})
			case dns.TypeSRV:
				records, extra, err = plugin.SRV(ctx, k, zone, state, plugin.Options{})
			case dns.TypeSOA:
				records, err = k.SOA(ctx, k.findMatchingZone(zo, qname), zone, state), nil

			case dns.TypeNS:
				if state.Name() == zone {
					records, extra, err = plugin.NS(ctx, k, zone, state, plugin.Options{})
					break
				}
				fallthrough
			default:
				// Do a fake A lookup, so we can distinguish between NODATA and NXDOMAIN
				fake := state.NewWithQuestion(state.QName(), dns.TypeA)
				fake.Zone = state.Zone
				_, _, err = plugin.A(ctx, k, zone, fake, nil, plugin.Options{})
			}
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
			return k.BackendError(ctx, zo, zone, dns.RcodeServerFailure, state, nil)
		}
		return k.BackendError(ctx, zo, zone, dns.RcodeNameError, state, nil)
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
			m.Ns = k.SOA(ctx, zo, zone, state)
		}
	}

	w.WriteMsg(m)
	return dns.RcodeSuccess, nil
}

// Name implements the Handler interface.
func (k *KubeDynDNS) Name() string { return "kubernetes" }

func (k *KubeDynDNS) findZone(base *objects.Zone, qname string) (*objects.Zone, []*objects.Entry, string) {
	qname = dns.Fqdn(qname)
	sub := qname[:len(qname)-len(base.DomainName)]
	labels := dns.SplitDomainName(sub)
	cur := base.DomainName
	rel := "."
	for i := range labels {
		l := labels[len(labels)-1-i]
		cur = dnsutil.Join(l, cur)
		rel = dnsutil.Join(l, rel)
		log.Infof("lookup nested zone for %s/%s\n", cur, rel)
		var ns []*objects.Entry
		for _, e := range k.APIConn.EntryDNSIndex(rel) {
			if e.ZoneRef == base.Name && e.Namespace == base.Namespace {
				if len(e.NS) != 0 {
					ns = append(ns, e)
				}
			}
		}
		if ns != nil {
			log.Infof("found delegated zone for %s: %s<%s>\n", cur, ns[0].Name, rel)
			return nil, ns, cur
		}
		for _, e := range k.APIConn.ZoneDomainIndex(rel) {
			if e.ParentRef == base.Name && e.Namespace == base.Namespace {
				log.Infof("found nested zone for %s: %s<%s>\n", cur, e.Name, rel)
				if !k.transitive {
					return e, nil, cur
				}
				rel = "."
				base = e
			}
		}
	}

	return base, nil, cur
}

func (k *KubeDynDNS) findMatchingZone(base *objects.Zone, qname string) *objects.Zone {

	currentDomain := dns.Fqdn(qname)
	labelCount := dns.CountLabel(currentDomain)

	if base == nil {
		return nil
	}
	for i := 0; i < labelCount; i++ {
		zs := k.APIConn.ZoneDomainIndex(currentDomain)
		if z := k.findNested(zs, base); z != nil {
			return z
		}

		// dns.NextLabel finds the start of the next label.
		// We use it to cut off the first label (e.g., cut 'www' from 'www.example.com.')
		nextLabelOffset, _ := dns.NextLabel(currentDomain, 0)
		currentDomain = currentDomain[nextLabelOffset:]
	}
	return nil
}

func (k *KubeDynDNS) findNested(zones []*objects.Zone, base *objects.Zone) *objects.Zone {
	for i, zone := range zones {
		for zone != nil {
			if zone == base || zone.ParentRef == base.Name {
				return zones[i]
			}
			if !dns.IsSubDomain(base.DomainName, zone.DomainName) || zone.ParentRef == "" {
				zone = nil
			} else {
				zone = k.APIConn.GetZone(&cache.ObjectName{zone.Namespace, zone.ParentRef})
			}
		}
	}
	return nil
}

func (k *KubeDynDNS) SOA(ctx context.Context, z *objects.Zone, zone string, state request.Request) []dns.RR {
	if z != nil {
		ttl := uint32(min(z.MinimumTTL, 300))
		header := dns.RR_Header{Name: z.DomainName, Rrtype: dns.TypeSOA, Ttl: ttl, Class: dns.ClassINET}

		nsrv := dnsutil.Join("ns.dns", z.DomainName)
		if len(z.NameServers) > 0 {
			nsrv = dns.Fqdn(z.NameServers[0])
		}
		mbox := dnsutil.Join("hostmaster", z.DomainName)
		if z.EMail != "" {
			mbox = z.EMail
		}
		soa := &dns.SOA{Hdr: header,
			Mbox:    mbox,
			Ns:      nsrv,
			Serial:  k.Serial(state),
			Refresh: uint32(z.Refresh),
			Retry:   uint32(z.Retry),
			Expire:  uint32(z.Expire),
			Minttl:  ttl,
		}
		return []dns.RR{soa}
	} else {
		recs, _ := plugin.SOA(ctx, k, zone, state, plugin.Options{})
		return recs
	}
}

func (k *KubeDynDNS) NS(nsrv, name string, ttl uint32, state request.Request) []dns.RR {
	return []dns.RR{&dns.NS{Hdr: dns.RR_Header{Name: name, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: k.TTL(ttl)}, Ns: nsrv}}
}

// BackendError writes an error response to the client.
func (k *KubeDynDNS) BackendError(ctx context.Context, zo *objects.Zone, zone string, rcode int, state request.Request, err error) (int, error) {
	m := new(dns.Msg)
	m.SetRcode(state.Req, rcode)
	m.Authoritative = true
	m.Ns = k.SOA(ctx, zo, zone, state)

	state.W.WriteMsg(m)
	// Return success as the rcode to signal we have written to the client.
	return dns.RcodeSuccess, err
}
