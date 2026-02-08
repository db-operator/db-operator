/*
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:validation:Enum=postgres;mysql;
type Engine string

// DbInstanceSpec defines the desired state of DbInstance
type DbInstanceSpec struct {
	// If provided, operator will not try to detect the backend
	// automatically and will use the value to bootstrap
	// the database interface
	// +optional
	Engine Engine `json:"engine,omitempty"`
	// InstanceData
	// Credentials
}

// DbInstanceStatus defines the observed state of DbInstance.
type DbInstanceStatus struct {
	// Is the operator able to connect to the database server
	Connected bool `json:"connected"`
	// conditions represent the current state of the DbInstance resource.
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

// DbInstance is the Schema for the dbinstances API
type DbInstance struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of DbInstance
	// +required
	Spec DbInstanceSpec `json:"spec"`

	// status defines the observed state of DbInstance
	// +optional
	Status DbInstanceStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// DbInstanceList contains a list of DbInstance
type DbInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []DbInstance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DbInstance{}, &DbInstanceList{})
}
