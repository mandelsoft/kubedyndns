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
// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/mandelsoft/kubedyndns/apis/coredns/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// CoreDNSEntryLister helps list CoreDNSEntries.
// All objects returned here must be treated as read-only.
type CoreDNSEntryLister interface {
	// List lists all CoreDNSEntries in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.CoreDNSEntry, err error)
	// CoreDNSEntries returns an object that can list and get CoreDNSEntries.
	CoreDNSEntries(namespace string) CoreDNSEntryNamespaceLister
	CoreDNSEntryListerExpansion
}

// coreDNSEntryLister implements the CoreDNSEntryLister interface.
type coreDNSEntryLister struct {
	indexer cache.Indexer
}

// NewCoreDNSEntryLister returns a new CoreDNSEntryLister.
func NewCoreDNSEntryLister(indexer cache.Indexer) CoreDNSEntryLister {
	return &coreDNSEntryLister{indexer: indexer}
}

// List lists all CoreDNSEntries in the indexer.
func (s *coreDNSEntryLister) List(selector labels.Selector) (ret []*v1alpha1.CoreDNSEntry, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.CoreDNSEntry))
	})
	return ret, err
}

// CoreDNSEntries returns an object that can list and get CoreDNSEntries.
func (s *coreDNSEntryLister) CoreDNSEntries(namespace string) CoreDNSEntryNamespaceLister {
	return coreDNSEntryNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// CoreDNSEntryNamespaceLister helps list and get CoreDNSEntries.
// All objects returned here must be treated as read-only.
type CoreDNSEntryNamespaceLister interface {
	// List lists all CoreDNSEntries in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.CoreDNSEntry, err error)
	// Get retrieves the CoreDNSEntry from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.CoreDNSEntry, error)
	CoreDNSEntryNamespaceListerExpansion
}

// coreDNSEntryNamespaceLister implements the CoreDNSEntryNamespaceLister
// interface.
type coreDNSEntryNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all CoreDNSEntries in the indexer for a given namespace.
func (s coreDNSEntryNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.CoreDNSEntry, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.CoreDNSEntry))
	})
	return ret, err
}

// Get retrieves the CoreDNSEntry from the indexer for a given namespace and name.
func (s coreDNSEntryNamespaceLister) Get(name string) (*v1alpha1.CoreDNSEntry, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("corednsentry"), name)
	}
	return obj.(*v1alpha1.CoreDNSEntry), nil
}
