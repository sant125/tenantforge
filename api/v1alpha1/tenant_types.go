/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// TenantTier defines the level of service for a tenant, which can be "Small", "Medium", or "Large".
type TenantTier string

const (
	TenantTierSmall  TenantTier = "Small"
	TenantTierMedium TenantTier = "Medium"
	TenantTierLarge  TenantTier = "Large"
)

// Define the possible values for NetworkIsolation, which can be "Open" or "Isolated".
type NetworkIsolationLevel string

const (
	NetworkIsolationLevelOpen     NetworkIsolationLevel = "Open"
	NetworkIsolationLevelIsolated NetworkIsolationLevel = "Isolated"
)

// TenantSpec defines the desired state of Tenant
type TenantSpec struct {
	// clientName identifica a qual cliente este tenant pertence
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// garanto que o formato do campo spec.clientName seja compatível com o formato de nomes de recursos do Kubernetes, que é um DNS subdomain name.
	ClientName string `json:"clientName"`

	// nodePool define se o tenant terá um NodePool dedicado ou não
	// +kubebuilder:validation:Optional
	NodePool bool `json:"nodePool,omitempty"`

	// tier define o nível de serviço do tenant, que pode ser "Small", "Medium" ou "Large".
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Small;Medium;Large
	TenantTier TenantTier `json:"tier"`

	// networkIsolation define o nível de isolamento de rede do tenant, que pode ser "Open" ou "Isolated".
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Open;Isolated
	NetworkIsolation NetworkIsolationLevel `json:"networkIsolation"`
}

// TenantStatus defines the observed state of Tenant.
type TenantStatus struct {
	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the Tenant resource.
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
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// Tenant is the Schema for the tenants API
type Tenant struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Tenant
	// +required
	Spec TenantSpec `json:"spec"`

	// status defines the observed state of Tenant
	// +optional
	Status TenantStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// TenantList contains a list of Tenant
type TenantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Tenant `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &Tenant{}, &TenantList{})
		return nil
	})
}
