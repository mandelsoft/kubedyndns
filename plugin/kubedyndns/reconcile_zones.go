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

	"github.com/mandelsoft/kubedyndns/plugin/kubedyndns/objects"
	"github.com/miekg/dns"
	"k8s.io/client-go/tools/cache"
)

func (cntr *controller) reconcileZone(key cache.ObjectName, no int) error {
	if cntr.zoneRef == nil {
		Log.Error("oops, unexpected zone reconcilation")
		return nil
	}

	o, ok, err := cntr.zoneLister.GetByKey(key.String())
	if err != nil {
		return err
	}
	if !ok {
		Log.Infof("hosted zone %q has been deleted", key)
		// update entries for deleted zone
		cntr.triggerEntriesForZone(key)
		cntr.triggerNestedZones(key)
		return nil
	}

	z := o.(*objects.Zone)

	ok, root, err := cntr.responsibleForZoneObject(z, nil)
	if err == nil && root == "" {
		if z.Error == nil {
			z.Error = fmt.Errorf("no root zone found")
		}
		mod, err := z.UpdateStatus(cntr.ctx, cntr.client)
		if err != nil {
			return err
		}
		if mod {
			cntr.triggerEntriesForZone(key)
		}
	}

	if !ok && root != "" {
		Log.Infof("not responsible for root zone %q for %s", root, key)
	}

	if err != nil || !ok {
		return err
	}

	Log.Infof("responsible for %s", key)

	_, err = z.UpdateStatus(cntr.ctx, cntr.client)
	if err != nil {
		return err
	}

	cntr.triggerEntriesForZone(key)
	cntr.triggerNestedZones(key)
	return nil
}

func (cntr *controller) enqueueZone(key cache.ObjectName) {
	cntr.queue.Add(NewRequestKey(objects.TYPE_ZONE, key.Namespace, key.Name))
}

func (cntr *controller) triggerEntriesForZone(key cache.ObjectName) {
	for _, e := range cntr.EntryZoneIndex(key) {
		k := cache.MetaObjectToName(e)
		Log.Infof("triggering entry %s", k)
		cntr.enqueueEntry(k)
	}
}

func (cntr *controller) triggerNestedZones(key cache.ObjectName) {
	for _, e := range cntr.ZoneParentIndex(key) {
		k := cache.MetaObjectToName(e)
		Log.Infof("triggering nested zone for %s", k)
		cntr.enqueueZone(k)
	}
}

func (cntr *controller) responsibleForZoneObject(z *objects.Zone, names *[]string) (bool, string, error) {
	if cntr.zoneRef == nil || z.Namespace != cntr.zoneRef.Namespace {
		return false, "", nil
	}
	for {
		aggregateNames(z, names)
		if z.Name == cntr.zoneRef.Name {
			return true, cntr.zoneRef.Name, nil
		}

		if z.ParentRef == "" {
			return false, z.Name, nil
		}

		n := cache.NewObjectName(z.Namespace, z.ParentRef)
		o, ok, err := cntr.zoneLister.GetByKey(n.String())
		if err != nil || !ok {
			return false, "", err
		}
		z = o.(*objects.Zone)
	}
}

func aggregateNames(z *objects.Zone, names *[]string) {
	if names == nil {
		return
	}
	var result []string

	for _, n := range *names {
		for _, suf := range z.DomainNames {
			result = append(result, dns.Fqdn(n)+dns.Fqdn(suf))
		}
	}
	*names = result
}
