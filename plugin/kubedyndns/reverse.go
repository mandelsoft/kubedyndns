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

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/etcd/msg"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/request"
)

// Reverse implements the ServiceBackend interface.
func (k *KubeDynDNS) Reverse(ctx context.Context, state request.Request, exact bool, opt plugin.Options) ([]msg.Service, error) {

	ip := dnsutil.ExtractAddressFromReverse(state.Name())
	if ip == "" {
		_, e := k.Records(ctx, state, exact)
		return nil, e
	}

	return nil, nil
}
