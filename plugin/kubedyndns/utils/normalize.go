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

package utils

import (
	"github.com/coredns/coredns/plugin"
	"github.com/miekg/dns"
)

func Normalize(n plugin.Host) string {
	names := n.NormalizeExact()
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

// GetParentDomains uses the miekg/dns library to safely iterate
// over the parent domains of a QNAME.
func GetParentDomains(qname string) []string {
	// 1. Normalize the QNAME: miekg/dns requires FQDN (trailing dot) for some functions,
	// but the SplitDomainName function handles the root dot gracefully.
	// We ensure the trailing dot is present for maximum compatibility.
	fqdn := dns.Fqdn(qname)

	parentDomains := make([]string, 0)
	currentDomain := fqdn

	// 2. The SplitDomainName function tells us how many labels the QNAME has.
	// We need to iterate that many times.
	labelCount := dns.CountLabel(fqdn)

	// 3. Iterate from the QNAME down to the TLD.
	// The loop runs 'labelCount' times, removing the left-most label each time.
	for i := 0; i < labelCount; i++ {
		parentDomains = append(parentDomains, currentDomain)

		// dns.NextLabel finds the start of the next label.
		// We use it to cut off the first label (e.g., cut 'www' from 'www.example.com.')
		nextLabelOffset, _ := dns.NextLabel(currentDomain, 0)
		currentDomain = currentDomain[nextLabelOffset:]
	}

	return parentDomains
}
