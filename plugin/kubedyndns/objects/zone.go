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
	"net/mail"
	"strings"

	"github.com/coredns/coredns/plugin"
	"github.com/miekg/dns"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"

	api "github.com/mandelsoft/kubedyndns/apis/coredns/v1alpha1"
	clientapi "github.com/mandelsoft/kubedyndns/client/clientset/versioned"

	"github.com/coredns/coredns/plugin/kubernetes/object"
)

// Zone is a stripped down api.HostedZone with only the items we need for CoreDNS.
type Zone struct {
	Version   string
	Name      string
	Namespace string
	Valid     bool

	api.HostedZoneSpec

	NameServers []string

	*object.Empty
}

// ToZone returns a client specific converter for converting an api.HostedZone to a *Zone.
func ToZone(ctx context.Context, client clientapi.Interface) func(obj meta.Object) (meta.Object, error) {
	return func(obj meta.Object) (meta.Object, error) {
		e, ok := obj.(*api.HostedZone)
		if !ok {
			return nil, fmt.Errorf("unexpected object %v", obj)
		}
		s := &Zone{
			Version:        e.GetResourceVersion(),
			Name:           e.GetName(),
			Namespace:      e.GetNamespace(),
			HostedZoneSpec: e.Spec,
		}

		for _, n := range e.Status.NameServers {
			fmt.Printf("cache zone %q\n", plugin.Name(n).Normalize())
			s.NameServers = append(s.NameServers, plugin.Name(n).Normalize())
		}

		s.DomainNames = nil
		for _, n := range e.Spec.DomainNames {
			s.DomainNames = append(s.DomainNames, dns.Fqdn(n))
		}

		var err error
		if s.EMail == "" {
			err = fmt.Errorf("email address requird")
		} else {
			_, err := mail.ParseAddress(s.EMail)
			if err == nil {
				comps := strings.Split(s.EMail, "@")
				s.EMail = dns.Fqdn(strings.Replace(comps[0], ".", "\\.", -1) + "." + comps[1])
			}
		}

		if err != nil {
			s.Valid = false
			if e.Status.Message != err.Error() || e.Status.State != "Invalid" {
				e.Status.Message = err.Error()
				e.Status.State = "Invalid"
				_, err = client.CorednsV1alpha1().HostedZones(e.Namespace).UpdateStatus(ctx, e, meta.UpdateOptions{})
			} else {
				err = nil
			}
		} else {
			s.Valid = true
			if e.Status.Message != "" || e.Status.State != "Ok" {
				e.Status.Message = ""
				e.Status.State = "Ok"
				_, err = client.CorednsV1alpha1().HostedZones(e.Namespace).UpdateStatus(ctx, e, meta.UpdateOptions{})
			}
		}
		if err != nil {
			Log.Errorf("error updating zone status %s/%s: %s", e.Namespace, e.Name, err)
		}
		*e = api.HostedZone{}

		return s, nil
	}
}

var _ runtime.Object = &Zone{}

// DeepCopyObject implements the ObjectKind interface.
func (s *Zone) DeepCopyObject() runtime.Object {
	s1 := &Zone{
		Version:   s.Version,
		Name:      s.Name,
		Namespace: s.Namespace,
	}
	set(&s1.NameServers, s.NameServers)
	s1.HostedZoneSpec = s.HostedZoneSpec
	return s1
}

// Equal checks if the update to an entry is something
// that matters to us or if they are effectively equivalent.
func (e *Zone) Equal(b *Zone) bool {
	if e == nil || b == nil {
		return false
	}

	if len(e.NameServers) != len(b.NameServers) {
		return false
	}

	// we should be able to rely on
	// these being sorted and able to be compared
	// they are supposed to be in a canonical format
	if !sets.NewString(e.NameServers...).Equal(sets.NewString(b.NameServers...)) {
		return false
	}

	if !e.HostedZoneSpec.Equal(&b.HostedZoneSpec) {
		return false
	}
	return true
}

// GetNamespace implements the metav1.Object interface.
func (s *Zone) GetNamespace() string { return s.Namespace }

// SetNamespace implements the metav1.Object interface.
func (s *Zone) SetNamespace(namespace string) {}

// GetName implements the metav1.Object interface.
func (s *Zone) GetName() string { return s.Name }

// SetName implements the metav1.Object interface.
func (s *Zone) SetName(name string) {}

// GetResourceVersion implements the metav1.Object interface.
func (s *Zone) GetResourceVersion() string { return s.Version }

// SetResourceVersion implements the metav1.Object interface.
func (s *Zone) SetResourceVersion(version string) {}
