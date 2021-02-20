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
	version   string
	name      string
	namespace string
	ttl       uint32
	index     []string
	hosts     []string
	services  []api.SRVRecord

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
		version:   e.GetResourceVersion(),
		name:      e.GetName(),
		namespace: e.GetNamespace(),
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
	if len(e.Spec.CNAME) > 0 {
		hosts = append(hosts, plugin.Name(e.Spec.CNAME).Normalize())
	}
	s.hosts = hosts
	copy(s.services, e.Spec.SRV)

	*e = api.CoreDNSEntry{}

	return s, nil
}

var _ runtime.Object = &Entry{}

// DeepCopyObject implements the ObjectKind interface.
func (s *Entry) DeepCopyObject() runtime.Object {
	s1 := &Entry{
		version:   s.version,
		name:      s.name,
		namespace: s.namespace,
	}
	copy(s1.index, s.index)
	copy(s1.hosts, s.hosts)
	copy(s1.services, s.services)
	return s1
}

func (s *Entry) Services(t uint16) []msg.Service {
	var result []msg.Service
	switch t {
	case dns.TypeA, dns.TypeAAAA, dns.TypeCNAME:
		for _, h := range s.hosts {
			result = append(result, msg.Service{
				Host: h,
				Port: -1,
				Mail: false,
				TTL:  s.ttl,
			})
		}
	case dns.TypeSRV:
		for _, h := range s.services {
			result = append(result, msg.Service{
				Host:     h.Target,
				Port:     h.Port,
				Priority: h.Priority,
				Weight:   h.Weight,
				Mail:     false,
				TTL:      s.ttl,
			})
		}
	}
	return result
}

// GetNamespace implements the metav1.Object interface.
func (s *Entry) GetNamespace() string { return s.namespace }

// SetNamespace implements the metav1.Object interface.
func (s *Entry) SetNamespace(namespace string) {}

// GetName implements the metav1.Object interface.
func (s *Entry) GetName() string { return s.name }

// SetName implements the metav1.Object interface.
func (s *Entry) SetName(name string) {}

// GetResourceVersion implements the metav1.Object interface.
func (s *Entry) GetResourceVersion() string { return s.version }

// SetResourceVersion implements the metav1.Object interface.
func (s *Entry) SetResourceVersion(version string) {}

// Index returns copy of index
func (s *Entry) Index() []string {
	return s.index[:]
}

// Equivalent checks if the update to an entry is something
// that matters to us or if they are effectively equivalent.
func (a *Entry) Equivalent(b *Entry) bool {
	if a == nil || b == nil {
		return false
	}

	if len(a.index) != len(b.index) {
		return false
	}
	if len(a.hosts) != len(b.hosts) {
		return false
	}
	if len(a.services) != len(b.services) {
		return false
	}

	if !sets.NewString(a.index...).Equal(sets.NewString(b.index...)) {
		return false
	}
	if !sets.NewString(a.hosts...).Equal(sets.NewString(b.hosts...)) {
		return false
	}
	// we should be able to rely on
	// these being sorted and able to be compared
	// they are supposed to be in a canonical format
	for i, sa := range a.services {
		if sa != b.services[i] {
			return false
		}
	}
	return true
}