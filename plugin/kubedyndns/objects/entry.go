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
	"context"
	"fmt"
	"net"
	"reflect"
	"slices"
	"strings"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/etcd/msg"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	api "github.com/mandelsoft/kubedyndns/apis/coredns/v1alpha1"
	clientapi "github.com/mandelsoft/kubedyndns/client/clientset/versioned"
	"github.com/miekg/dns"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/coredns/coredns/plugin/kubernetes/object"
)

var Log clog.P

// Entry is a stripped down api.CoreDNSEntry with only the items we need for CoreDNS.
type Entry struct {
	Version   string
	Name      string
	Namespace string
	ZoneRef   string
	Valid     bool
	Ttl       uint32
	DNSNames  []string

	A     []string
	AAAA  []string
	CNAME string

	Text    []string
	NS      []string
	Service *api.ServiceSpec

	*object.Empty
}

func normalizeHost(host, zone string) string {
	if !strings.HasSuffix(host, ".") {
		return plugin.Name(host).Normalize() + zone
	}
	return host
}

func normalizeRecords(recs []api.SRVRecord, zone string) []api.SRVRecord {
	r := make([]api.SRVRecord, len(recs))

	for i, v := range recs {
		n := v
		n.Host = normalizeHost(n.Host, zone)
		r[i] = n
	}
	return r
}

// ToEntry returns a client specific converter for converting an api.CoreDNSEntry to a *Entry.
func ToEntry(ctx context.Context, client clientapi.Interface) func(obj meta.Object) (meta.Object, error) {
	return func(obj meta.Object) (meta.Object, error) {
		e, ok := obj.(*api.CoreDNSEntry)
		if !ok {
			return nil, fmt.Errorf("unexpected object %v", obj)
		}
		s := &Entry{
			Version:   e.GetResourceVersion(),
			Name:      e.GetName(),
			Namespace: e.GetNamespace(),
			ZoneRef:   e.Spec.ZoneRef,
		}

		for _, n := range e.Spec.DNSNames {
			fmt.Printf("cache %q\n", plugin.Name(n).Normalize())
			s.DNSNames = append(s.DNSNames, plugin.Name(n).Normalize())
		}

		var err error
		var hosts []string
		for _, ips := range e.Spec.A {
			ip := net.ParseIP(ips)
			if ip == nil || ip.To4() == nil {
				err = fmt.Errorf("invalid ipv4 address %q", ips)
			} else {
				hosts = append(hosts, ips)
			}
		}
		s.A, hosts = hosts, nil

		for _, ips := range e.Spec.AAAA {
			ip := net.ParseIP(ips)
			if ip == nil || ip.To4() != nil {
				err = fmt.Errorf("invalid ipv6 address %q", ips)
			} else {
				hosts = append(hosts, ips)
			}
		}
		s.AAAA, hosts = hosts, nil

		if len(e.Spec.CNAME) > 0 {
			s.CNAME = e.Spec.CNAME
		}

		set(&s.Text, e.Spec.TXT)
		if e.Spec.SRV != nil {
			s.Service = &api.ServiceSpec{Service: e.Spec.SRV.Service}
			set(&s.Service.Records, slices.Clone(e.Spec.SRV.Records))
		}

		if len(e.Spec.DNSNames) == 0 {
			err = fmt.Errorf("at least one DNS name is required")
		}
		if len(e.Spec.A) == 0 && len(e.Spec.AAAA) == 0 && len(e.Spec.CNAME) == 0 && len(e.Spec.NS) == 0 && (e.Spec.SRV == nil || len(e.Spec.SRV.Records) == 0) {
			err = fmt.Errorf("no record defined")
		}
		if e.Spec.SRV != nil {
			if len(e.Spec.SRV.Records) != 0 && len(e.Spec.SRV.Service) == 0 {
				err = fmt.Errorf("service name required for SRV record")
			}
			for i, r := range e.Spec.SRV.Records {
				if r.Protocol != "TCP" && r.Protocol != "UDP" {
					err = fmt.Errorf("invalid protocol %q for SRV record %d", r.Protocol, i)
				}
				if r.Port <= 0 {
					err = fmt.Errorf("invalid port for SRV record %d", i)
				}
				if len(r.Host) == 0 {
					err = fmt.Errorf("host missing for SRV record %d", i)
				}
			}
		}
		if err != nil {
			s.Valid = false
			if e.Status.Message != err.Error() || e.Status.State != "Invalid" {
				e.Status.Message = err.Error()
				e.Status.State = "Invalid"
				_, err = client.CorednsV1alpha1().CoreDNSEntries(e.Namespace).UpdateStatus(ctx, e, meta.UpdateOptions{})
			} else {
				err = nil
			}
		} else {
			s.Valid = true
			if e.Status.Message != "" || e.Status.State != "Ok" {
				e.Status.Message = ""
				e.Status.State = "Ok"
				_, err = client.CorednsV1alpha1().CoreDNSEntries(e.Namespace).UpdateStatus(ctx, e, meta.UpdateOptions{})
			}
		}
		if err != nil {
			Log.Errorf("error updating entry status %s/%s: %s", e.Namespace, e.Name, err)
		}
		*e = api.CoreDNSEntry{}

		return s, nil
	}
}

var _ runtime.Object = &Entry{}

// DeepCopyObject implements the ObjectKind interface.
func (s *Entry) DeepCopyObject() runtime.Object {
	s1 := &Entry{
		Version:   s.Version,
		Name:      s.Name,
		Namespace: s.Namespace,
	}
	set(&s1.DNSNames, s.DNSNames)
	set(&s1.A, s.A)
	set(&s1.AAAA, s.AAAA)
	set(&s1.Text, s.Text)
	s1.CNAME = s.CNAME
	if s.Service.Service != "" {
		s1.Service.Service = s.Service.Service
		set(&s1.Service.Records, s.Service.Records)
	}
	return s1
}

// Equal checks if the update to an entry is something
// that matters to us or if they are effectively equivalent.
func (e *Entry) Equal(b *Entry) bool {
	if e == nil || b == nil {
		return false
	}

	if !slices.Equal(e.DNSNames, b.DNSNames) {
		return false
	}
	if !slices.Equal(e.A, b.A) {
		return false
	}
	if !slices.Equal(e.AAAA, b.AAAA) {
		return false
	}
	if !slices.Equal(e.Text, b.Text) {
		return false
	}
	if e.Service != nil || b.Service != nil {
		if e.Service != b.Service && e.Service.Service != b.Service.Service {
			return false
		}
	}

	if e.CNAME != b.CNAME {
		return false
	}
	if e.Service != nil && e.Service.Service != "" {
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

func (s *Entry) serviceForHosts(defttl uint32, hosts ...string) []msg.Service {
	var result []msg.Service
	for _, h := range hosts {
		result = append(result, msg.Service{
			Host: h,
			Port: -1,
			Mail: false,
			TTL:  DefTTL(s.Ttl, defttl),
			Key:  coredns,
		})
	}
	return result
}

func (s *Entry) Services(t uint16, p string, defttl uint32, zone string) []msg.Service {
	if !s.Valid {
		return nil
	}
	var result []msg.Service
	switch t {
	case dns.TypeANY:
		result = s.serviceForHosts(defttl, s.A...)
		result = append(result, s.serviceForHosts(defttl, s.AAAA...)...)
		result = append(result, s.serviceForHosts(defttl, s.CNAME)...)
		result = append(result, s.Services(dns.TypeTXT, p, defttl, zone)...)
		result = append(result, s.Services(dns.TypeSRV, p, defttl, zone)...)
	case dns.TypeA:
		result = s.serviceForHosts(defttl, s.A...)
	case dns.TypeAAAA:
		result = s.serviceForHosts(defttl, s.AAAA...)
	case dns.TypeCNAME:
		result = s.serviceForHosts(defttl, s.CNAME)
	case dns.TypeTXT:
		for _, h := range s.Text {
			result = append(result, msg.Service{
				Text: h,
				Port: -1,
				Mail: false,
				TTL:  DefTTL(s.Ttl, defttl),
				Key:  coredns,
			})
		}
	case dns.TypeSRV:
		if s.Service.Service != "" {
			for _, h := range s.Service.Records {
				if h.Protocol == p {
					result = append(result, msg.Service{
						Host:     normalizeHost(h.Host, zone),
						Port:     h.Port,
						Priority: h.Priority,
						Weight:   h.Weight,
						Mail:     false,
						TTL:      DefTTL(s.Ttl, defttl),
						Key:      coredns,
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

func (s *Entry) MatchType(t uint16) bool {
	switch t {
	case dns.TypeANY:
		return s.MatchType(dns.TypeA) || s.MatchType(dns.TypeAAAA) || s.MatchType(dns.TypeCNAME) || s.MatchType(dns.TypeTXT) || s.MatchType(dns.TypeSRV) || s.MatchType(dns.TypeNS)
	case dns.TypeA:
		return len(s.A) > 0
	case dns.TypeAAAA:
		return len(s.AAAA) > 0
	case dns.TypeCNAME:
		return s.CNAME != ""
	case dns.TypeTXT:
		return len(s.Text) > 0
	case dns.TypeSRV:
		return s.Service != nil
	case dns.TypeNS:
		return len(s.NS) > 0
	case dns.TypeMX:
		return false
	}
	return false
}

func set[E any](dst *[]E, src []E) {
	*dst = slices.Clone(src)
}

func setx(dst interface{}, src interface{}) {
	dv := reflect.ValueOf(dst)
	if dv.Kind() != reflect.Ptr || dv.Type().Elem().Kind() != reflect.Slice {
		panic(fmt.Sprintf("invalid slice target %T", dst))
	}
	sv := reflect.ValueOf(src)
	for sv.Kind() == reflect.Ptr {
		sv = sv.Elem()
	}
	if sv.Kind() != reflect.Slice && sv.Kind() != reflect.Array {
		panic(fmt.Sprintf("invalid slice source %T", src))
	}
	slice := reflect.New(reflect.SliceOf(dv.Type().Elem().Elem())).Elem()
	for i := 0; i < sv.Len(); i++ {
		slice = reflect.Append(slice, sv.Index(i))
	}
	dv.Elem().Set(slice)
}

func DefTTL(ttl, def uint32) uint32 {
	if ttl == 0 {
		return def
	}
	return ttl
}

const coredns = "c" // used as a fake key prefix in msg.Service
