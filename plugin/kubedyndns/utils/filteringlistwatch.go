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
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

type ListWatch interface {
	cache.ListerWatcher
	cache.ListerWatcherWithContext
}

type FilteringListWatch struct {
	lw     *cache.ListWatch
	filter func(obj runtime.Object) bool
}

func NewFilteringListWatch(lw *cache.ListWatch, filter func(obj runtime.Object) bool) ListWatch {
	if filter == nil {
		return lw
	}
	return &FilteringListWatch{
		lw, filter,
	}
}

func (f *FilteringListWatch) List(options metav1.ListOptions) (runtime.Object, error) {
	return f.ListWithContext(context.Background(), options)
}

func (f *FilteringListWatch) ListWithContext(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
	list, err := f.lw.ListWithContext(ctx, options)
	if err != nil {
		return nil, err
	}

	filteredItems := []unstructured.Unstructured{}
	for _, item := range list.(*unstructured.UnstructuredList).Items {
		obj := item.DeepCopy()
		if f.filter(obj) {
			filteredItems = append(filteredItems, *obj)
		}
	}

	list.(*unstructured.UnstructuredList).Items = filteredItems
	return list, nil
}

func (f *FilteringListWatch) Watch(options metav1.ListOptions) (watch.Interface, error) {
	return f.WatchWithContext(context.Background(), options)
}

func (f *FilteringListWatch) WatchWithContext(ctx context.Context, options metav1.ListOptions) (watch.Interface, error) {
	w, err := f.lw.WatchWithContext(ctx, options)
	if err != nil {
		return nil, err
	}

	// Wrap the watcher so you can drop events
	return watch.Filter(w, func(event watch.Event) (watch.Event, bool) {
		if event.Type == watch.Bookmark || event.Type == watch.Error {
			return event, true
		}
		if !f.filter(event.Object) {
			// ensure object is deleted
			return watch.Event{watch.Deleted, event.Object}, true
		}
		return event, true
	}), nil
}
