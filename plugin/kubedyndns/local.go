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

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
)

// boundIPs returns the list of non-loopback IPs that CoreDNS is bound to
func boundIPs(c *caddy.Controller) (ips []net.IP) {
	conf := dnsserver.GetConfig(c)
	hosts := conf.ListenHosts
	if hosts == nil || hosts[0] == "" {
		hosts = nil
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			return nil
		}
		for _, addr := range addrs {
			hosts = append(hosts, addr.String())
		}
	}
	for _, host := range hosts {
		ip, _, _ := net.ParseCIDR(host)
		ip4 := ip.To4()
		if ip4 != nil && !ip4.IsLoopback() {
			ips = append(ips, ip4)
			continue
		}
		ip6 := ip.To16()
		if ip6 != nil && !ip6.IsLoopback() {
			ips = append(ips, ip6)
		}
	}
	return ips
}
