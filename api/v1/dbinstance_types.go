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
	"k8s.io/apimachinery/pkg/runtime"
)

// DbInstanceSpec defines the desired state of DbInstance
type DbInstanceSpec struct {
	// When not set, db-operator will try to determin the engine itself.
	// +kubebuilder:validation:Enum=postgres;mysql
	Engine *string `json:"engine,omitempty"`
	Auth DbInstanceAuth `json:"auth,omitempty"`
	// +optional
	NamespaceFilters *[]string `json:"namespaceFilters,omitempty"`
}

// DbInstanceAuth defines the authentication information for a database instance.
type DbInstanceAuth struct {
	Username *ValueSource `json:"username,omitempty"`
	Password *ValueSource `json:"password,omitempty"`
}

type ValueSource struct {
	Value *string `json:"value,omitempty"`
	ValueFrom *ValueFrom `json:"valueFrom,omitempty"`
}

type ValueFrom struct {
	SecretRef *SecretOrCMRef  `json:"secret,omitempty"`
	ConfigMapRef *SecretOrCMRef `json:"configMap,omitempty"`
}

type SecretOrCMRef struct {
	Namespace *string `json:"namespace,omitempty"`
	Name *string `json:"name,omitempty"`
	Key *string `json:"key,omitempty"`
}

// DbInstanceStatus defines the observed state of DbInstance.
type DbInstanceStatus struct {
	// Which engine is used as a backend for this instance
	// +kubebuilder:validation:Enum=postgres;mysql
	Engine *string `json:"engine,omitempty"`
	// When ready is true, an instance can be used by other controllers
	Ready *bool `json:"ready,omitempty"`
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
// +kubebuilder:storageversion

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
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &DbInstance{}, &DbInstanceList{})
		return nil
	})
}
