// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
// +kubebuilder:resource:scope=Namespaced,shortName=cdnse,path=corednsentries,singular=coredns
// +kubebuilder:printcolumn:name=Hostname,JSONPath=".spec.hostname",type=string
// +kubebuilder:printcolumn:name=URLScheme,JSONPath=".spec.scheme",type=string
// +kubebuilder:printcolumn:name=Path,JSONPath=".spec.path",type=string
// +kubebuilder:printcolumn:name=Port,JSONPath=".spec.port",type=number
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CoreDNSEntry describes an additional coredns dns entry
type CoreDNSEntry struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec CoreDNSSpec `json:"spec"`
}

// CoreDNSSpec is  the specification for an dns entry object
type CoreDNSSpec struct {
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
	Protocol string `json:"protocol"`
	Priority int    `json:"priority"`
	Weight   int    `json:"weight"`
	Port     int    `json:"port"`
	Host     string `json:"host"`
}
