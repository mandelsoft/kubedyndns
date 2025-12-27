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

package kubedyndns

import (
	"fmt"
	"slices"

	"github.com/mandelsoft/kubedyndns/plugin/kubedyndns/objects"
	"k8s.io/client-go/tools/cache"
)

func (cntr *controller) reconcileEntry(key cache.ObjectName, no int) error {
	o, ok, err := cntr.entryLister.GetByKey(key.String())
	if err != nil || !ok {
		if !ok {
			Log.Infof("entry %q has been deleted", key)
		}
		return err
	}
	e := o.(*objects.Entry)

	var names []string
	var zone string

	if e.ZoneRef != "" {
		if cntr.zoneRef == nil || cntr.zoneRef.Namespace != e.Namespace {
			return nil
		}
		zn := cache.NewObjectName(key.Namespace, e.ZoneRef)

		o, ok, err := cntr.zoneLister.GetByKey(zn.String())
		if err != nil {
			return err
		}
		if !ok {
			return e.UpdateStatus(cntr.ctx, cntr.client, "", nil, fmt.Errorf("no root zone found"))
		}
		z := o.(*objects.Zone)

		names = slices.Clone(e.DNSNames)
		ok, root, err := cntr.responsibleForZoneObject(z, &names)
		Log.Infof("responsible: %v, root: %q, error: %v", ok, root, err)
		if err != nil {
			return err
		}
		if root == nil {
			return e.UpdateStatus(cntr.ctx, cntr.client, "", nil, fmt.Errorf("no root zone found"))
		}

		if !ok {
			// remove old zone responsibility
			if e.Status.RootZone != "" && e.Status.RootZone == cntr.zoneRef.Name {
				if root == nil {
					err = fmt.Errorf("no root zone found")
				} else {
					err = fmt.Errorf("responsibility lost")
				}
				return e.UpdateStatus(cntr.ctx, cntr.client, "", nil, err)
			}
			return nil
		}

		Log.Infof("domain names: %v", names)

		if z.Status.State != "Ready" && z.Status.State != "Ok" {
			Log.Infof("zone %q state is %q", z.Name, z.Status.State)
			return e.UpdateStatus(cntr.ctx, cntr.client, zone, names, fmt.Errorf("zone failure: %s", z.Status.Message))
		}
		zone = root.Name
	} else {
		if cntr.zoneRef != nil {
			return nil
		}
		if len(cntr.controlOpts.namespaces) > 0 && !cntr.controlOpts.namespaces.Has(key.Namespace) {
			return nil
		}
	}
	return e.UpdateStatus(cntr.ctx, cntr.client, zone, names, nil)
}

func (cntr *controller) enqueueEntry(key cache.ObjectName) {
	cntr.queue.Add(NewRequestKey(objects.TYPE_ENTRY, key.Namespace, key.Name))
}
