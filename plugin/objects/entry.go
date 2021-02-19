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

	api "github.com/mandelsoft/kubedyndns/apis/coredns/v1alpha1"
	"github.com/coredns/coredns/plugin"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/coredns/coredns/plugin/kubernetes/object"
)

// Entry is a stripped down api.CoreDNSEntry with only the items we need for CoreDNS.
type Entry struct {
	// Don't add new fields to this struct without talking to the CoreDNS maintainers.
	Version      string
	Name         string
	Namespace    string
	Index        []string
	IPs          []string
	CNames       []string

	*object.Empty
}

// EntryKey returns a string using for the index.
func EntryKey(obj *api.CoreDNSEntry) []string {
	keys:=[]string{}
	for _, k := range obj.Spec.DNSNames {
		keys=append(keys,plugin.Name(k).Normalize())
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
		Version:      e.GetResourceVersion(),
		Name:         e.GetName(),
		Namespace:    e.GetNamespace(),
		Index:        EntryKey(e),
	}

	if len(e.Spec.IP) > 0 {
		s.IPs =[]string{e.Spec.IP}
	}

	*e = api.CoreDNSEntry{}

	return s, nil
}

var _ runtime.Object = &Entry{}

// DeepCopyObject implements the ObjectKind interface.
func (s *Entry) DeepCopyObject() runtime.Object {
	s1 := &Entry{
		Version:      s.Version,
		Name:         s.Name,
		Namespace:    s.Namespace,
		Index:        s.Index,
	}
	copy(s1.IPs, s.IPs)
	return s1
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
