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
	"k8s.io/client-go/tools/cache"
)

type FilteringStore struct {
	delegate cache.Store
	filter   func(obj interface{}) bool
}

var _ cache.Store = (*FilteringStore)(nil)

func (f *FilteringStore) Resync() error {
	return f.delegate.Resync()
}

func (f *FilteringStore) Add(obj interface{}) error {
	if !f.filter(obj) {
		return nil // skip object
	}
	return f.delegate.Add(obj)
}

func (f *FilteringStore) Update(obj interface{}) error {
	if !f.filter(obj) {
		return nil // skip object
	}
	return f.delegate.Update(obj)
}

func (f *FilteringStore) Delete(obj interface{}) error {
	if !f.filter(obj) {
		return nil
	}
	return f.delegate.Delete(obj)
}

func (f *FilteringStore) List() []interface{} { return f.delegate.List() }
func (f *FilteringStore) ListKeys() []string  { return f.delegate.ListKeys() }
func (f *FilteringStore) Get(obj interface{}) (interface{}, bool, error) {
	return f.delegate.Get(obj)
}
func (f *FilteringStore) GetByKey(key string) (interface{}, bool, error) {
	return f.delegate.GetByKey(key)
}
func (f *FilteringStore) Replace(list []interface{}, resourceVersion string) error {
	// IMPORTANT: filter list at startup
	var filtered []interface{}
	for _, obj := range list {
		if f.filter(obj) {
			filtered = append(filtered, obj)
		}
	}
	return f.delegate.Replace(filtered, resourceVersion)
}
