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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DbRestoreSpec defines the desired state of DbRestore
type DbRestoreSpec struct {
	// Which image should be used
	// +kubebuilder:default={registry:"ghcr.io", repository:"db-operator/db-backup-tools", tag:"latest", pullPolicy:"IfNotPresent"}
	// +required
	Image *DbBackupImage `json:"image"`
	// A name of a database to back up. Must be in the same namespace
	// +required
	Database *string `json:"database"`
	// +required
	// +kubebuilder:default=3
	Retries *int32 `json:"retries"`
	// A name of the secret with credentials for uploading a backup
	// For example, rclone environment variables
	// Must be in the namespace where backup pods are created,
	// will be mounted directly to pods
	// +optional
	UploadCredentialsSecret *string `json:"uploadCredentialsSecret,omitzero"`
	// +required
	BackupPath *string `json:"backupPath"`
	// +required
	// +kubebuilder:default=false
	Cleanup *bool `json:"cleanup"`
}

// DbRestoreStatus defines the observed state of DbRestore.
type DbRestoreStatus struct {
	// How many retries have already failed
	// Use by the operator to stop retries once .spec.retries amount is reached
	// +kubebuilder:default=0
	FailedRetries *int32 `json:"failedRetries"`
	// +kubebuilder:default=false
	LockedByBackupJob *bool   `json:"lockedByBackupJob,omitempty"`
	OperatorVersion   *string `json:"operatorVersion,omitempty"`
	// +kubebuilder:default=false
	Status *bool `json:"status"`
	// conditions represent the current state of the DbRestore resource.
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

// DbRestore is the Schema for the dbrestores API
type DbRestore struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of DbRestore
	// +required
	Spec DbRestoreSpec `json:"spec"`

	// status defines the observed state of DbRestore
	// +optional
	Status DbRestoreStatus `json:"status,omitzero"`
}

// GetSecretName implements types.KindaObject.
func (in *DbRestore) GetSecretName() string {
	// Dummy
	return ""
}

// IsCleanup implements types.KindaObject.
func (in *DbRestore) IsCleanup() bool {
	return false
}

// IsDeleted implements types.KindaObject.
func (in *DbRestore) IsDeleted() bool {
	return in.GetDeletionTimestamp() != nil
}

// +kubebuilder:object:root=true

// DbRestoreList contains a list of DbRestore
type DbRestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []DbRestore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DbRestore{}, &DbRestoreList{})
}
