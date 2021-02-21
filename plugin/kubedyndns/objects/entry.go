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

package objects

import (
	"fmt"
	"net"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/etcd/msg"
	"github.com/miekg/dns"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"

	api "github.com/mandelsoft/kubedyndns/apis/coredns/v1alpha1"

	"github.com/coredns/coredns/plugin/kubernetes/object"
)

// Entry is a stripped down api.CoreDNSEntry with only the items we need for CoreDNS.
type Entry struct {
	Version   string
	Name      string
	Namespace string
	Ttl       uint32
	Index     []string
	Hosts     []string
	Text      []string
	Service   api.ServiceSpec

	*object.Empty
}

// EntryKey returns a string using for the index.
func EntryKey(obj *api.CoreDNSEntry) []string {
	keys := []string{}
	for _, k := range obj.Spec.DNSNames {
		keys = append(keys, plugin.Name(k).Normalize())
	}
	return keys
}

// ToEntry converts an api.Service to a *Service.
func ToEntry(obj meta.Object) (meta.Object, error) {
	e, ok := obj.(*api.CoreDNSEntry)
	if !ok {
		return nil, fmt.Errorf("unexpected object %v", obj)
	}
	s := &Entry{
		Version:   e.GetResourceVersion(),
		Name:      e.GetName(),
		Namespace: e.GetNamespace(),
	}

	var hosts []string
	for _, ips := range e.Spec.A {
		ip := net.ParseIP(ips)
		if ip != nil && ip.To4() != nil {
			hosts = append(hosts, ips)
		}
	}
	for _, ips := range e.Spec.AAAA {
		ip := net.ParseIP(ips)
		if ip != nil && ip.To4() == nil {
			hosts = append(hosts, ips)
		}
	}
	copy(s.Text, e.Spec.TXT)
	if len(e.Spec.CNAME) > 0 {
		hosts = append(hosts, plugin.Name(e.Spec.CNAME).Normalize())
	}
	s.Hosts = hosts
	if e.Spec.SRV != nil {
		s.Service.Service = e.Spec.SRV.Service
		copy(s.Service.Records, e.Spec.SRV.Records)
	}

	*e = api.CoreDNSEntry{}

	return s, nil
}

var _ runtime.Object = &Entry{}

// DeepCopyObject implements the ObjectKind interface.
func (s *Entry) DeepCopyObject() runtime.Object {
	s1 := &Entry{
		Version:   s.Version,
		Name:      s.Name,
		Namespace: s.Namespace,
	}
	copy(s1.Index, s.Index)
	copy(s1.Hosts, s.Hosts)
	copy(s1.Text, s.Text)
	if s.Service.Service != "" {
		s1.Service.Service = s.Service.Service
		copy(s1.Service.Records, s.Service.Records)
	}
	return s1
}

// Equaö checks if the update to an entry is something
// that matters to us or if they are effectively equivalent.
func (e *Entry) Equal(b *Entry) bool {
	if e == nil || b == nil {
		return false
	}

	if len(e.Index) != len(b.Index) {
		return false
	}
	if len(e.Hosts) != len(b.Hosts) {
		return false
	}
	if len(e.Text) != len(b.Text) {
		return false
	}
	if e.Service.Service != b.Service.Service {
		return false
	}
	// we should be able to rely on
	// these being sorted and able to be compared
	// they are supposed to be in a canonical format
	if !sets.NewString(e.Index...).Equal(sets.NewString(b.Index...)) {
		return false
	}
	if !sets.NewString(e.Hosts...).Equal(sets.NewString(b.Hosts...)) {
		return false
	}
	if !sets.NewString(e.Text...).Equal(sets.NewString(b.Text...)) {
		return false
	}
	if e.Service.Service != "" {
		if len(e.Service.Records) != len(b.Service.Records) {
			return false
		}
		for i, sa := range e.Service.Records {
			if sa != b.Service.Records[i] {
				return false
			}
		}
	}
	return true
}

func (s *Entry) Services(t uint16, p string) []msg.Service {
	var result []msg.Service
	switch t {
	case dns.TypeA, dns.TypeAAAA, dns.TypeCNAME:
		for _, h := range s.Hosts {
			result = append(result, msg.Service{
				Host: h,
				Port: -1,
				Mail: false,
				TTL:  s.Ttl,
			})
		}
	case dns.TypeTXT:
		for _, h := range s.Text {
			result = append(result, msg.Service{
				Host: h,
				Port: -1,
				Mail: false,
				TTL:  s.Ttl,
			})
		}
	case dns.TypeSRV:
		if s.Service.Service != "" {
			for _, h := range s.Service.Records {
				if h.Protocol == p {
					result = append(result, msg.Service{
						Host:     h.Host,
						Port:     h.Port,
						Priority: h.Priority,
						Weight:   h.Weight,
						Mail:     false,
						TTL:      s.Ttl,
					})
				}
			}
		}
	}
	return result
}

// GetNamespace implements the metav1.Object interface.
func (s *Entry) GetNamespace() string { return s.Namespace }

// SetNamespace implements the metav1.Object interface.
func (s *Entry) SetNamespace(namespace string) {}

// GetName implements the metav1.Object interface.
func (s *Entry) GetName() string { return s.Name }

// SetName implements the metav1.Object interface.
func (s *Entry) SetName(name string) {}

// GetResourceVersion implements the metav1.Object interface.

func (s *Entry) GetResourceVersion() string { return s.Version }

// SetResourceVersion implements the metav1.Object interface.
func (s *Entry) SetResourceVersion(version string) {}