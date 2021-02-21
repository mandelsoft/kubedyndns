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
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coredns/coredns/plugin/kubernetes/object"

	clientapi "github.com/mandelsoft/kubedyndns/client/clientset/versioned"
	"github.com/mandelsoft/kubedyndns/plugin/kubedyndns/objects"

	corev1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const (
	DNSIndex = "dns"
	IPIndex  = "ip"
)

type Controller interface {
	EntryList() []*objects.Entry
	EntryIndex(string) []*objects.Entry

	GetNamespaceByName(string) (*corev1.Namespace, error)

	Run()
	HasSynced() bool
	Stop() error

	// Modified returns the timestamp of the most recent changes
	Modified() int64
}

type controller struct {
	// Modified tracks timestamp of the most recent changes
	// It needs to be first because it is guaranteed to be 8-byte
	// aligned ( we use sync.LoadAtomic with this )
	modified int64

	kubeclient kubernetes.Interface
	client     clientapi.Interface

	selector          labels.Selector
	namespaceSelector labels.Selector

	entryController cache.Controller
	nsController    cache.Controller

	entryLister cache.Indexer
	nsLister    cache.Store

	// stopLock is used to enforce only a single call to Stop is active.
	// Needed because we allow stopping through an http endpoint and
	// allowing concurrent stoppers leads to stack traces.
	stopLock sync.Mutex
	shutdown bool
	stopCh   chan struct{}

	zones []string
}

type controlOpts struct {
	// Label handling.
	labelSelector          *meta.LabelSelector
	selector               labels.Selector
	namespaceLabelSelector *meta.LabelSelector
	namespaceSelector      labels.Selector

	zones            []string
	endpointNameMode bool
}

// newController creates a controller for CoreDNS.
func newController(ctx context.Context, kubeClient kubernetes.Interface, client clientapi.Interface, opts controlOpts) *controller {
	cntr := controller{
		kubeclient:        kubeClient,
		client:            client,
		selector:          opts.selector,
		namespaceSelector: opts.namespaceSelector,
		stopCh:            make(chan struct{}),
		zones:             opts.zones,
	}

	cntr.entryLister, cntr.entryController = object.NewIndexerInformer(
		&cache.ListWatch{
			ListFunc:  entryListFunc(ctx, cntr.client, corev1.NamespaceAll, cntr.selector),
			WatchFunc: entryWatchFunc(ctx, cntr.client, corev1.NamespaceAll, cntr.selector),
		},
		&corev1.Service{},
		cache.ResourceEventHandlerFuncs{AddFunc: cntr.Add, UpdateFunc: cntr.Update, DeleteFunc: cntr.Delete},
		cache.Indexers{DNSIndex: entryDNSIndexFunc},
		object.DefaultProcessor(objects.ToEntry(ctx, cntr.client), nil),
	)

	cntr.nsLister, cntr.nsController = cache.NewInformer(
		&cache.ListWatch{
			ListFunc:  namespaceListFunc(ctx, cntr.kubeclient, cntr.namespaceSelector),
			WatchFunc: namespaceWatchFunc(ctx, cntr.kubeclient, cntr.namespaceSelector),
		},
		&corev1.Namespace{},
		defaultResyncPeriod,
		cache.ResourceEventHandlerFuncs{})

	return &cntr
}

func entryDNSIndexFunc(obj interface{}) ([]string, error) {
	e, ok := obj.(*objects.Entry)
	if !ok {
		return nil, errObj
	}
	return e.Index, nil
}

func entryListFunc(ctx context.Context, c clientapi.Interface, ns string, s labels.Selector) func(meta.ListOptions) (runtime.Object, error) {
	return func(opts meta.ListOptions) (runtime.Object, error) {
		if s != nil {
			opts.LabelSelector = s.String()
		}
		return c.CorednsV1alpha1().CoreDNSEntries(ns).List(ctx, opts)
	}
}

func namespaceListFunc(ctx context.Context, c kubernetes.Interface, s labels.Selector) func(meta.ListOptions) (runtime.Object, error) {
	return func(opts meta.ListOptions) (runtime.Object, error) {
		if s != nil {
			opts.LabelSelector = s.String()
		}
		return c.CoreV1().Namespaces().List(ctx, opts)
	}
}

func entryWatchFunc(ctx context.Context, c clientapi.Interface, ns string, s labels.Selector) func(options meta.ListOptions) (watch.Interface, error) {
	return func(options meta.ListOptions) (watch.Interface, error) {
		if s != nil {
			options.LabelSelector = s.String()
		}
		return c.CorednsV1alpha1().CoreDNSEntries(ns).Watch(ctx, options)
	}
}

func namespaceWatchFunc(ctx context.Context, c kubernetes.Interface, s labels.Selector) func(options meta.ListOptions) (watch.Interface, error) {
	return func(options meta.ListOptions) (watch.Interface, error) {
		if s != nil {
			options.LabelSelector = s.String()
		}
		return c.CoreV1().Namespaces().Watch(ctx, options)
	}
}

// Stop stops the  controller.
func (cntr *controller) Stop() error {
	cntr.stopLock.Lock()
	defer cntr.stopLock.Unlock()

	// Only try draining the workqueue if we haven't already.
	if !cntr.shutdown {
		close(cntr.stopCh)
		cntr.shutdown = true

		return nil
	}

	return fmt.Errorf("shutdown already in progress")
}

// Run starts the controller.
func (cntr *controller) Run() {
	go cntr.entryController.Run(cntr.stopCh)
	go cntr.nsController.Run(cntr.stopCh)
	<-cntr.stopCh
}

// HasSynced calls on all controllers.
func (cntr *controller) HasSynced() bool {
	a := cntr.entryController.HasSynced()
	b := cntr.nsController.HasSynced()
	return a && b
}

func (cntr *controller) EntryList() (entries []*objects.Entry) {
	os := cntr.entryLister.List()
	for _, o := range os {
		s, ok := o.(*objects.Entry)
		if !ok {
			continue
		}
		entries = append(entries, s)
	}
	return entries
}

func (cntr *controller) EntryIndex(idx string) (entries []*objects.Entry) {
	os, err := cntr.entryLister.ByIndex(DNSIndex, idx)
	if err != nil {
		return nil
	}
	for _, o := range os {
		s, ok := o.(*objects.Entry)
		if !ok {
			continue
		}
		entries = append(entries, s)
	}
	return entries
}

// GetNamespaceByName returns the namespace by name. If nothing is found an error is returned.
func (cntr *controller) GetNamespaceByName(name string) (*corev1.Namespace, error) {
	os := cntr.nsLister.List()
	for _, o := range os {
		ns, ok := o.(*corev1.Namespace)
		if !ok {
			continue
		}
		if name == ns.ObjectMeta.Name {
			return ns, nil
		}
	}
	return nil, fmt.Errorf("namespace not found")
}

func (cntr *controller) Add(obj interface{})               { cntr.updateModifed() }
func (cntr *controller) Delete(obj interface{})            { cntr.updateModifed() }
func (cntr *controller) Update(oldObj, newObj interface{}) { cntr.detectChanges(oldObj, newObj) }

// detectChanges detects changes in objects, and updates the modified timestamp
func (cntr *controller) detectChanges(oldObj, newObj interface{}) {
	// If both objects have the same resource version, they are identical.
	if newObj != nil && oldObj != nil && (oldObj.(meta.Object).GetResourceVersion() == newObj.(meta.Object).GetResourceVersion()) {
		return
	}
	obj := newObj
	if obj == nil {
		obj = oldObj
	}
	switch ob := obj.(type) {
	case *objects.Entry:
		if !(oldObj.(*objects.Entry).Equal(newObj.(*objects.Entry))) {
			cntr.updateModifed()
		}
	default:
		log.Warningf("Updates for %T not supported.", ob)
	}
}

// subsetsEquivalent checks if two endpoint subsets are significantly equivalent
// I.e. that they have the same ready addresses, host names, ports (including protocol
// and service names for SRV)
func subsetsEquivalent(sa, sb object.EndpointSubset) bool {
	if len(sa.Addresses) != len(sb.Addresses) {
		return false
	}
	if len(sa.Ports) != len(sb.Ports) {
		return false
	}

	// in Addresses and Ports, we should be able to rely on
	// these being sorted and able to be compared
	// they are supposed to be in a canonical format
	for addr, aaddr := range sa.Addresses {
		baddr := sb.Addresses[addr]
		if aaddr.IP != baddr.IP {
			return false
		}
		if aaddr.Hostname != baddr.Hostname {
			return false
		}
	}

	for port, aport := range sa.Ports {
		bport := sb.Ports[port]
		if aport.Name != bport.Name {
			return false
		}
		if aport.Port != bport.Port {
			return false
		}
		if aport.Protocol != bport.Protocol {
			return false
		}
	}
	return true
}

func (cntr *controller) Modified() int64 {
	unix := atomic.LoadInt64(&cntr.modified)
	return unix
}

// updateModified set dns.modified to the current time.
func (cntr *controller) updateModifed() {
	unix := time.Now().Unix()
	atomic.StoreInt64(&cntr.modified, unix)
}

var errObj = errors.New("obj was not of the correct type")

const defaultResyncPeriod = 0
