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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type HostedZoneList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata
	// More info: http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HostedZone `json:"items"`
}

// HostedZone describes a a DNS Hosted Zone configured in a namespace.
// +kubebuilder:storageversion
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced,shortName=chs,path=hostedzones,singular=hostedzone
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name=Domain,JSONPath=".spec.domainName",type=string
// +kubebuilder:printcolumn:name=Parent,JSONPath=".spec.parentRef",type=string
// +kubebuilder:printcolumn:name=EMail,JSONPath=".spec.email",type=string
// +kubebuilder:printcolumn:name=NameServer,JSONPath=".status.nameServers",type=string
// +kubebuilder:printcolumn:name=State,JSONPath=".status.state",type=string
// +kubebuilder:printcolumn:name=Serial,JSONPath=".spec.serial",type=string,priority=1
// +kubebuilder:printcolumn:name=Refresh,JSONPath=".spec.refresh",type=string,priority=1
// +kubebuilder:printcolumn:name=Retry,JSONPath=".spec.retry",type=string,priority=1
// +kubebuilder:printcolumn:name=Expire,JSONPath=".spec.expire",type=string,priority=1
// +kubebuilder:printcolumn:name=MinTTL,JSONPath=".spec.minimumTTL",type=string,priority=1
// +kubebuilder:printcolumn:name=Message,JSONPath=".status.message",type=string,priority=1
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type HostedZone struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec HostedZoneSpec `json:"spec"`
	// +optional
	Status HostedZoneStatus `json:"status"`
}

// HostedZoneSpec is  the specification for a dns hostedzone object
type HostedZoneSpec struct {
	// DomainName is the name of the domain of the hosted zone.
	DomainName string `json:"domainName"`

	// EMail address of admins.
	EMail string `json:"email"`

	// Refresh is the interval for secondaries to query to updates
	Refresh int `json:"refresh"`

	// Retry time to repeat refresh.
	Retry int `json:"retry"`

	// Expire is the maximal validity interval.
	Expire int `json:"expire"`

	// MinmumTTL is the minimal live time.
	MinimumTTL int `json:"minimumTTL"`

	// ParantRef is the name if a local hosted zone resource it is linked to.
	// +optional
	ParentRef string `json:"parentRef"`
}

// HostedZoneStatus describes the statuso a hostedzone.
type HostedZoneStatus struct {
	// State of the dns entry object
	// +optional
	State string `json:"state,omitempty"`
	// Error message in case of an invalid entry
	// +optional
	Message string `json:"message,omitempty"`

	// NameServers is a list of wname servers for the hosted zone.
	// +optional
	NameServers []string `json:"nameServers"`
}
