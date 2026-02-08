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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// +kubebuilder:validation:Enum=postgres;mysql;
type Engine string

// DbInstanceSpec defines the desired state of DbInstance
type DbInstanceSpec struct {
	// If provided, operator will not try to detect the backend
	// automatically and will use the value to bootstrap
	// the database interface
	// +optional
	Engine           *Engine                     `json:"engine,omitempty"`
	AdminCredentials *DbInstanceAdminCredentials `json:"adminCredentials"`
	Host             string                      `json:"host"`
	Port             string                      `json:"port"`
}

// When using with ArgoCD, it might make sense to provide
// UsernameKey and PasswordKey explicitly to avoid state
// conflicts.
type DbInstanceAdminCredentials struct {
	*NamespacedName `json:",inline"`
	// When not set, defaults to "username"
	// +kubebuilder:default:=username
	UsernameKey string `json:"usernameKey"`
	// When not set, defaults to "password"
	// +kubebuilder:default:=password
	PasswordKey string `json:"passwordKey"`
}

// DbInstanceStatus defines the observed state of DbInstance.
type DbInstanceStatus struct {
	// Is the operator able to connect to the database server
	Connected bool `json:"connected"`
	// A list of databases deployed to the instance
	Databases []string `json:"databases"`
	// A version of the operator that was used for the last
	// full reconciliation
	OperatorVersion string `json:"operatorVersion,omitempty"`
	// Is the instance ready to accept instances and users
	Status bool `json:"status"`
	// Either detected engine, or the value of the engine in the spec
	Engine string `json:"engine,omitempty"`
	// Version of the database server
	Version string `json:"version,omitempty"`
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
// +kubebuilder:resource:scope=Cluster,shortName=dbin
//+kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.status`,description="health status"
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

// GetName implements [types.KindaObject].
// Subtle: this method shadows the method (ObjectMeta).GetName of DbInstance.ObjectMeta.
func (in *DbInstance) GetName() string {
	panic("unimplemented")
}

// GetNamespace implements [types.KindaObject].
// Subtle: this method shadows the method (ObjectMeta).GetNamespace of DbInstance.ObjectMeta.
func (in *DbInstance) GetNamespace() string {
	panic("unimplemented")
}

// GetObjectKind implements [types.KindaObject].
// Subtle: this method shadows the method (TypeMeta).GetObjectKind of DbInstance.TypeMeta.
func (in *DbInstance) GetObjectKind() schema.ObjectKind {
	panic("unimplemented")
}

// GetSecretName implements [types.KindaObject].
func (in *DbInstance) GetSecretName() string {
	panic("unimplemented")
}

// GetUID implements [types.KindaObject].
// Subtle: this method shadows the method (ObjectMeta).GetUID of DbInstance.ObjectMeta.
func (in *DbInstance) GetUID() types.UID {
	panic("unimplemented")
}

// IsCleanup implements [types.KindaObject].
func (in *DbInstance) IsCleanup() bool {
	panic("unimplemented")
}

// IsDeleted implements [types.KindaObject].
func (in *DbInstance) IsDeleted() bool {
	panic("unimplemented")
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
