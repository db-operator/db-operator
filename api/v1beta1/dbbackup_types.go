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

// DbBackupSpec defines the desired state of DbBackup
type DbBackupSpec struct {
	// Which image should be used
	// +kubebuilder:default={registry:"ghcr.io", repository:"db-operator/db-backup-tools", tag:"latest", pullPolicy:"IfNotPresent"}
	// +required
	Image *DbBackupImage `json:"image"`
	// +required
	// +kubebuilder:default=3
	Retries *int32 `json:"retries"`
	// A name of a database to back up. Must be in the same namespace
	// +required
	Database *string `json:"database"`
	// A name of the secret with credentials for uploading a backup
	// For example, rclone environment variables
	// Must be in the namespace where backup pods are created,
	// will be mounted directly to pods
	// +optional
	StorageCredentials *string `json:"uploadCredentialsSecret,omitzero"`
	// +required
	// +kubebuilder:default=false
	Cleanup *bool `json:"cleanup"`
	// +kubebuilder:default=false
	Debug *bool `json:"debug"`
}

type DbBackupImage struct {
	// For example docker.io or ghcr.io
	// +kubebuilder:default="ghcr.io"
	// +required
	Registry *string `json:"registry"`
	// For example db-operator/postgresql-backup
	// +kubebuilder:default="db-operator/db-backup-tools"
	// +required
	Repository *string `json:"repository"`
	// For example latest
	// +kubebuilder:default="latest"
	// +required
	Tag *string `json:"tag"`
	// For example Never
	// +kubebuilder:default="IfNotPresent"
	// +required
	PullPolicy *string `json:"pullPolicy"`
}

// DbBackupStatus defines the observed state of DbBackup.
type DbBackupStatus struct {
	// How many retries have already failed
	// Use by the operator to stop retries once .spec.retries amount is reached
	// +kubebuilder:default=0
	FailedRetries *int32 `json:"failedRetries"`
	// +kubebuilder:default=false
	LockedByBackupJob *bool   `json:"lockedByBackupJob,omitempty"`
	OperatorVersion   *string `json:"operatorVersion,omitempty"`
	// +kubebuilder:default=0
	BackupDuration *int64 `json:"backupDuration,omitempty"`
	// +kubebuilder:default=0
	BackupSize *int64 `json:"backupSize,omitempty"`
	// +kubebuilder:default=0
	UploadDuration *int64 `json:"uploadDuration,omitempty"`
	// +kubebuilder:default=false
	Status *bool `json:"status,omitempty"`
	// Database engine. It's used by the db-backup-cli
	Engine *string `json:"engine,omitempty"`
	// Path of the backup in the storage
	Path *string `json:"path,omitempty"`
	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the DbBackup resource.
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
// +kubebuilder:printcolumn:name="Path",type=string,JSONPath=`.status.path`,description="A path to the backup in the external storage."
// +kubebuilder:printcolumn:name="Status",type=boolean,JSONPath=`.status.status`,description="If database was backed up."
// +kubebuilder:printcolumn:name="Locked",type=boolean,JSONPath=`.status.lockedByBackupJob`,description="Is DbBackup locked by the backup pod"
// +kubebuilder:printcolumn:name="OperatorVersion",type=string,JSONPath=`.status.operatorVersion`,description="db-operator version of last full reconcile"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`,description="time since creation of resource"

// DbBackup is the Schema for the dbbackups API
type DbBackup struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of DbBackup
	// +required
	Spec DbBackupSpec `json:"spec"`

	// status defines the observed state of DbBackup
	// +optional
	Status DbBackupStatus `json:"status,omitzero"`
}

// GetSecretName implements [types.KindaObject].
func (in *DbBackup) GetSecretName() string {
	// Dummy
	return ""
}

// IsCleanup implements [types.KindaObject].
func (in *DbBackup) IsCleanup() bool {
	return false
}

// IsDeleted implements [types.KindaObject].
func (in *DbBackup) IsDeleted() bool {
	return in.GetDeletionTimestamp() != nil
}

// +kubebuilder:object:root=true

// DbBackupList contains a list of DbBackup
type DbBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []DbBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DbBackup{}, &DbBackupList{})
}
