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
// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	time "time"

	corednsv1alpha1 "github.com/mandelsoft/kubedyndns/apis/coredns/v1alpha1"
	versioned "github.com/mandelsoft/kubedyndns/client/clientset/versioned"
	internalinterfaces "github.com/mandelsoft/kubedyndns/client/informers/externalversions/internalinterfaces"
	v1alpha1 "github.com/mandelsoft/kubedyndns/client/listers/coredns/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// CoreDNSEntryInformer provides access to a shared informer and lister for
// CoreDNSEntries.
type CoreDNSEntryInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.CoreDNSEntryLister
}

type coreDNSEntryInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewCoreDNSEntryInformer constructs a new informer for CoreDNSEntry type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewCoreDNSEntryInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredCoreDNSEntryInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredCoreDNSEntryInformer constructs a new informer for CoreDNSEntry type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredCoreDNSEntryInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.CorednsV1alpha1().CoreDNSEntries(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.CorednsV1alpha1().CoreDNSEntries(namespace).Watch(context.TODO(), options)
			},
		},
		&corednsv1alpha1.CoreDNSEntry{},
		resyncPeriod,
		indexers,
	)
}

func (f *coreDNSEntryInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredCoreDNSEntryInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *coreDNSEntryInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&corednsv1alpha1.CoreDNSEntry{}, f.defaultInformer)
}

func (f *coreDNSEntryInformer) Lister() v1alpha1.CoreDNSEntryLister {
	return v1alpha1.NewCoreDNSEntryLister(f.Informer().GetIndexer())
}