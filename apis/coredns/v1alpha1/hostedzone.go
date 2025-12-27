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
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// HostedZoneSpec defines the desired state of HostedZone
type HostedZoneSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// Class is used separate different hosted zone realms managed by different
	// controller sets.
	// It should only be set for root zones (without a parent).
	// +optional
	Class *string `json:"class,omitempty"`

	// Runtime is used to specify the logical runtime to use
	// for deploying the primary DNS server.
	// It should only be set for root zones (without a parent).
	// +optional
	Runtime *string `json:"runtime,omitempty"`

	// DomainNames is a set of domain names for the hosted zone.
	// Formally, for every name a new DNS zone is managed.
	DomainNames []string `json:"domainNames"`

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

type Observed struct {
	// Class already used for implementation.
	Class string `json:"class"`

	// Runtime already used for implementation.
	Runtime string `json:"runtime"`
}

func (o *Observed) Equals(other *Observed) bool {
	if o == nil || other == nil {
		return other == o
	}
	return reflect.DeepEqual(o, other)
}

// HostedZoneStatus defines the observed state of HostedZone.
type HostedZoneStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the HostedZone resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// State of the hosted zone object
	// +optional
	State string `json:"state,omitempty"`

	// Error message in case of an invalid entry
	// +optional
	Message string `json:"message,omitempty"`

	// NameServers is a list of name servers for the hosted zone.
	// +optional
	NameServers []string `json:"nameServers"`

	// Observed provides information about implementation.
	// +optional
	Observed *Observed `json:"observed,omitempty"`
}

// +kubebuilder:storageversion
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=chz,path=hostedzones,singular=hostedzone,categories=dns
// +kubebuilder:printcolumn:name=Domain,JSONPath=".spec.domainNames",type=string
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
// +kubebuilder:rbac:groups=coredns.mandelsoft.org,resources=hostedzones,verbs=get;list;watch;create;update;patch;delete,labels=rbac.authorization.k8s.io/aggregate-to-admin=true

// HostedZone is the Schema for the hostedzones API
type HostedZone struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of HostedZone
	// +required
	Spec HostedZoneSpec `json:"spec"`

	// status defines the observed state of HostedZone
	// +optional
	Status HostedZoneStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HostedZoneList contains a list of HostedZone
type HostedZoneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []HostedZone `json:"items"`
}
