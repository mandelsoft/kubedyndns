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

type CoreDNSEntryList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata
	// More info: http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CoreDNSEntry `json:"items"`
}

// +kubebuilder:storageversion
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced,shortName=cdnse,path=corednsentries,singular=corednsentry
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name=DNSNames,JSONPath=".spec.dnsNames",type=string
// +kubebuilder:printcolumn:name=ZoneRef,JSONPath=".spec.zoneRef",type=string
// +kubebuilder:printcolumn:name=A,JSONPath=".spec.A",type=string
// +kubebuilder:printcolumn:name=CNAME,JSONPath=".spec.CNAME",type=string
// +kubebuilder:printcolumn:name=SRV,JSONPath=".spec.SRV.service",type=string
// +kubebuilder:printcolumn:name=State,JSONPath=".status.state",type=string
// +kubebuilder:printcolumn:name=Message,JSONPath=".status.message",type=string,priority=1
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CoreDNSEntry describes an additional coredns dns entry
type CoreDNSEntry struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec CoreDNSSpec `json:"spec"`
	// +optional
	Status CoreDNSStatus `json:"status"`
}

// CoreDNSSpec is  the specification for an dns entry object
type CoreDNSSpec struct {
	// ZoneRef is the name of the hosted zone
	// +optional
	ZoneRef string `json:"zoneRef"`

	// DNSNames is a list of DNSNames
	DNSNames []string `json:"dnsNames"`
	// +optional
	A []string `json:"A,omitempty"`
	// +optional
	AAAA []string `json:"AAAA,omitempty"`
	// +optional
	TXT []string `json:"TXT,omitempty"`
	// +optional
	SRV *ServiceSpec `json:"SRV,omitempty"`
	// +optional
	CNAME string `json:"CNAME,omitempty"`
	// +optional
	NS []string `json:"NS,omitempty"`
}

const PROTO_TCP = "TCP"
const PROTO_UDP = "UPD"

// ServiceSpec describes a service's SRV records
type ServiceSpec struct {
	Service string      `json:"service"`
	Records []SRVRecord `json:"records"`
}

// SRVRecord is a service record
type SRVRecord struct {
	// Protocol of the service record (UDP/TCP)
	Protocol string `json:"protocol"`
	// Priority of the service record
	// +optional
	Priority int `json:"priority,omitempty"`
	// Weight of the service record
	// +optional
	Weight int `json:"weight,omitempty"`
	// Port of the service record
	Port int `json:"port"`
	// Target of the service record
	Host string `json:"host"`
}

// CoreDNSStatus describes the status of an entry
type CoreDNSStatus struct {
	// State of the dns entry object
	// +optional
	State string `json:"state,omitempty"`
	// Error message in case of an invalid entry
	// +optional
	Message string `json:"message,omitempty"`
}
