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
	"strings"

	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/miekg/dns"
)

type recordRequest struct {
	// The named service
	// SRV record.
	service string
	// The protocol is usually _udp or _tcp (if set), and comes from the protocol part of a well formed
	// SRV record.
	protocol string
	// The domain nme of the service
	domain string
}

func (r *recordRequest) IsServiceRequest() bool {
	return r.service != ""
}

// parseRequest parses the qname to find all the elements we need for querying k8s. Anything
// that is not parsed will be empty.
// Potential underscores are stripped from _port and _protocol.
func parseRequest(name, zone string) (*recordRequest, error) {
	// 23Possible cases:
	// 1. _service._protocol.<path>.zone
	// 3. <path>.zone

	base, _ := dnsutil.TrimZone(name, zone)
	segs := dns.SplitDomainName(base)
	last := len(segs) - 1
	if last < 0 {
		return nil, errInvalidRequest
	}
	// return NODATA for apex queries
	if segs[0] == "_tcp" || segs[0] == "_upd" {
		return nil, errInvalidRequest
	}

	r := &recordRequest{domain: base}
	for i, s := range segs {
		if s == "_tcp" || s == "_udp" {
			r.service = stripUnderscore(strings.Join(segs[0:i], "."))
			r.domain = strings.Join(segs[i+1:], ".")
			r.protocol = strings.ToUpper(stripUnderscore(s))
			break
		}
	}

	if r.service == "" && r.protocol != "" {
		return nil, errInvalidRequest
	}
	return r, nil
}

// stripUnderscore removes a prefixed underscore from s.
func stripUnderscore(s string) string {
	if s[0] != '_' {
		return s
	}
	return s[1:]
}

// String returns a string representation of r, it just returns all fields concatenated with dots.
// This is mostly used in tests.
func (r *recordRequest) String() string {
	s := r.service
	s += "/" + r.protocol
	s += "/" + r.domain
	return s
}
