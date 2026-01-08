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
	meta2 "k8s.io/apimachinery/pkg/api/meta"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"

	api "github.com/mandelsoft/kubedyndns/apis/coredns/v1alpha1"
	clientapi "github.com/mandelsoft/kubedyndns/client/clientset/versioned"

	"github.com/coredns/coredns/plugin/kubernetes/object"
)

// Zone is a stripped down api.HostedZone with only the items we need for CoreDNS.
type Zone struct {
	Plain     bool
	Version   string
	Name      string
	Namespace string
	Error     error

	*api.HostedZoneSpec

	Status *api.HostedZoneStatus

	*object.Empty
}

func (z *Zone) GetType() string {
	return TYPE_ZONE
}

func (z *Zone) String() string {
	if z == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%s/%s[%v]", z.Namespace, z.Name, z.DomainNames)
}

// ToZone returns a client specific converter for converting an api.HostedZone to a *Zone.
func ToZone(ctx context.Context, client clientapi.Interface, transitive bool, slave bool) func(obj meta.Object) (meta.Object, error) {
	return func(obj meta.Object) (meta.Object, error) {
		e, ok := obj.(*api.HostedZone)
		if !ok {
			return nil, fmt.Errorf("unexpected object %v", obj)
		}
		s := &Zone{
			Plain:          !slave && IsPlain(e.Status.Conditions),
			Version:        e.GetResourceVersion(),
			Name:           e.GetName(),
			Namespace:      e.GetNamespace(),
			HostedZoneSpec: e.Spec.DeepCopy(),
			Status:         e.Status.DeepCopy(),
		}

		for _, n := range e.Status.NameServers {
			fmt.Printf("cache zone %q\n", plugin.Name(n).Normalize())
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
		if !transitive {
			if e.Spec.ParentRef != "" {
				err = fmt.Errorf("nested zones not supported in non-transitive mode")
			}
		}
		s.Error = err
		*e = api.HostedZone{}

		return s, nil
	}
}

func IsPlain(conditions []meta.Condition) bool {
	// check for plain mode.
	// This means the controller is explictly managed and not by an aaS controller
	// managing additional conditions
	plain := true
	for _, c := range conditions {
		if c.Type != api.ServerConditionType {
			plain = false
			break
		}
	}
	return plain
}

func (z *Zone) UpdateStatus(ctx context.Context, client clientapi.Interface) (bool, error) {
	var o api.HostedZone

	if z.Plain {
		Log.Infof("using plain mode for %d conditions", len(z.Status.Conditions))
	}

	mod := false
	if z.Plain {
		if len(o.Status.Conditions) > 0 {
			o.Status.Conditions = nil
			mod = true
		}
	}

	// In plain mode the status is managed directly. otherwise
	// a server condition is managed, which is the handled by the aaS controller
	// to determine an aggregated object state.
	o.ResourceVersion = z.GetResourceVersion()
	o.Name = z.GetName()
	o.Namespace = z.GetNamespace()
	z.Status.DeepCopyInto(&o.Status)
	if z.Error != nil {
		if z.Plain {
			if o.Status.Message != z.Error.Error() || o.Status.State != "Invalid" {
				o.Status.Message = z.Error.Error()
				o.Status.State = "Invalid"
				mod = true
			}
		} else {
			mod = meta2.SetStatusCondition(&o.Status.Conditions, meta.Condition{
				Type:               api.ServerConditionType,
				Status:             meta.ConditionFalse,
				ObservedGeneration: o.ObjectMeta.Generation,
				Reason:             api.ReasonServerValidationFailure,
				Message:            z.Error.Error(),
			})
		}
	} else {
		if z.Plain {
			if o.Status.Message != "zone is served" || o.Status.State != "Ok" {
				o.Status.Message = "zone is served"
				o.Status.State = "Ok"
				mod = true
			}
		} else {
			mod = meta2.SetStatusCondition(&o.Status.Conditions, meta.Condition{
				Type:               api.ServerConditionType,
				Status:             meta.ConditionTrue,
				ObservedGeneration: o.ObjectMeta.Generation,
				Reason:             api.ReasonServerActive,
				Message:            "hosted zone served",
			})
		}
	}
	if mod {
		_, err := client.CorednsV1alpha1().HostedZones(o.Namespace).UpdateStatus(ctx, &o, meta.UpdateOptions{})
		if err != nil {
			Log.Errorf("error updating zone status %s/%s: %s", o.Namespace, o.Name, err)
		} else {
			Log.Infof("zone status %s/%s updated: %#v", o.Namespace, o.Name, o.Status)
		}
		return mod, err
	}
	return mod, nil
}

var _ runtime.Object = &Zone{}

// DeepCopyObject implements the ObjectKind interface.
func (z *Zone) DeepCopyObject() runtime.Object {
	s1 := &Zone{
		Version:   z.Version,
		Name:      z.Name,
		Namespace: z.Namespace,
	}
	s1.Status = z.Status.DeepCopy()
	s1.HostedZoneSpec = z.HostedZoneSpec.DeepCopy()
	return s1
}

// Equal checks if the update to an entry is something
// that matters to us or if they are effectively equivalent.
func (z *Zone) Equal(b *Zone) bool {
	if z == nil || b == nil {
		return false
	}

	if len(z.Status.NameServers) != len(b.Status.NameServers) {
		return false
	}

	// we should be able to rely on
	// these being sorted and able to be compared
	// they are supposed to be in a canonical format
	if !sets.NewString(z.Status.NameServers...).Equal(sets.NewString(b.Status.NameServers...)) {
		return false
	}

	if !z.HostedZoneSpec.Equal(b.HostedZoneSpec) {
		return false
	}
	if z.Status.State != b.Status.State {
		return false
	}
	return true
}

// GetNamespace implements the metav1.Object interface.
func (z *Zone) GetNamespace() string { return z.Namespace }

// SetNamespace implements the metav1.Object interface.
func (z *Zone) SetNamespace(namespace string) {}

// GetName implements the metav1.Object interface.
func (z *Zone) GetName() string { return z.Name }

// SetName implements the metav1.Object interface.
func (z *Zone) SetName(name string) {}

// GetResourceVersion implements the metav1.Object interface.
func (z *Zone) GetResourceVersion() string { return z.Version }

// SetResourceVersion implements the metav1.Object interface.
func (z *Zone) SetResourceVersion(version string) { z.Version = version }
