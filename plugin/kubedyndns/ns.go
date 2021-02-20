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
	"net"
	"strings"

	"github.com/miekg/dns"
)

func isDefaultNS(name, zone string) bool {
	return strings.Index(name, defaultNSName) == 0 && strings.Index(name, zone) == len(defaultNSName)
}

// nsAddrs returns the A or AAAA records for the CoreDNS service in the cluster. If the service cannot be found,
// it returns a record for the local address of the machine we're running on.
func (k *KubeDynDNS) nsAddrs(external bool, zone string) []dns.RR {
	var (
		svcNames []string
		svcIPs   []net.IP
	)

	// Find the CoreDNS Endpoints
	for _, localIP := range k.localIPs {
		endpoints := k.APIConn.EpIndexReverse(localIP.String())

		// Collect IPs for all Services of the Endpoints
		for _, endpoint := range endpoints {
			svcs := k.APIConn.SvcIndex(endpoint.Index)
			for _, svc := range svcs {
				if external {
					svcName := strings.Join([]string{svc.Name, svc.Namespace, zone}, ".")
					for _, exIP := range svc.ExternalIPs {
						svcNames = append(svcNames, svcName)
						svcIPs = append(svcIPs, net.ParseIP(exIP))
					}
					continue
				}
				svcName := strings.Join([]string{svc.Name, svc.Namespace, Svc, zone}, ".")
				if svc.Headless() {
					// For a headless service, use the endpoints IPs
					for _, s := range endpoint.Subsets {
						for _, a := range s.Addresses {
							svcNames = append(svcNames, endpointHostname(a, k.opts.endpointNameMode)+"."+svcName)
							svcIPs = append(svcIPs, net.ParseIP(a.IP))
						}
					}
				} else {
					for _, clusterIP := range svc.ClusterIPs {
						svcNames = append(svcNames, svcName)
						svcIPs = append(svcIPs, net.ParseIP(clusterIP))
					}
				}
			}
		}
	}

	// If no local IPs matched any endpoints, use the localIPs directly
	if len(svcIPs) == 0 {
		svcIPs = make([]net.IP, len(k.localIPs))
		svcNames = make([]string, len(k.localIPs))
		for i, localIP := range k.localIPs {
			svcNames[i] = defaultNSName + zone
			svcIPs[i] = localIP
		}
	}

	// Create an RR slice of collected IPs
	rrs := make([]dns.RR, len(svcIPs))
	for i, ip := range svcIPs {
		if ip.To4() == nil {
			rr := new(dns.AAAA)
			rr.Hdr.Class = dns.ClassINET
			rr.Hdr.Rrtype = dns.TypeAAAA
			rr.Hdr.Name = svcNames[i]
			rr.AAAA = ip
			rrs[i] = rr
			continue
		}
		rr := new(dns.A)
		rr.Hdr.Class = dns.ClassINET
		rr.Hdr.Rrtype = dns.TypeA
		rr.Hdr.Name = svcNames[i]
		rr.A = ip
		rrs[i] = rr
	}

	return rrs
}

const defaultNSName = "ns.dns."
