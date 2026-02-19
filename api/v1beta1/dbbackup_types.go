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
	// +kubebuilder:default={}
	// +optional
	Image *DbBackupImage `json:"image"`
	// +required
	// +kubebuilder:default=3
	Retries *int32 `json:"retries"`
	// A name of a database to back up. Must be in the same namespace
	// +required
	Database *string `json:"database"`
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
}

// DbBackupStatus defines the observed state of DbBackup.
type DbBackupStatus struct {
	// If true, operaror will not reconcile the object
	// Is needed to allow the backup pod to change the status of the CR
	// without triggering a full reconcile loop again
	LockedByBackupPod *bool `json:"lockedByBackupPod"`
	// How many retries have already failed
	// Use by the operator to stop retries once .spec.retries amount is reached
	// +kubebuilder:default=0
	FailedRetries *int32 `json:"failedRetries"`
	// How long did it take to back up the database (in seconds)
	// +kubebuilder:default=0
	DumpDuration *int32 `json:"dumpDuration"`
	// How long did it take to upload a backup (in seconds)
	// +kubebuilder:default=0
	UploadDuration *int32 `json:"uploadDuration"`
	// The size of the backup (in bytes)
	// +kubebuilder:default=0
	Size            *int64  `json:"size"`
	OperatorVersion *string `json:"operatorVersion,omitempty"`
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
// +kubebuilder:printcolumn:name="BackupSuccess",type=boolean,JSONPath=`.status.backup.success`,description="If database was backed up."
// +kubebuilder:printcolumn:name="UploadStatus",type=boolean,JSONPath=`.status.upload.success`,description="If a backup was uploaded to an external storage."
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
