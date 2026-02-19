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
	// +required
	// +kubebuilder:default="{{ .ImageRegistry }}/{{ .ImageRepository }}-{{ .Engine }}:{{ .ImageTag }}"
	ImageTemplate *string `json:"imageTemplate"`
	// +required
	// +kubebuilder:default=3
	Retries *int32 `json:"retries"`
	// +required
	Database *string `json:"database"`

	// foo is an example field of DbBackup. Edit dbbackup_types.go to remove/update
	// +optional
	Foo *string `json:"foo,omitempty"`
}

// DbBackupStatus defines the observed state of DbBackup.
type DbBackupStatus struct {
	// Status of the backup process
	Backup *BackupStatus `json:"backup"`
	// Status of the upload process
	Upload *UploadStatus `json:"upload"`
	// Which operator version was used to reconcile this object
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

type BackupStatus struct {
	// How many retries have already failed
	// Use by the operator to stop retries once .spec.retries amount is reached
	// +kubebuilder:default=0
	FailedRetries *int32 `json:"failedRetries"`
	// If true, it means that the backup script was executed without errors
	// +kubebuilder:default=false
	Success *bool `json:"success"`
	// How long did it take to back up the database (in seconds)
	// +kubebuilder:default=0
	Duration *int32 `json:"duration"`
	// The size of the backup (in bytes)
	// +kubebuilder:default=0
	Size *int64 `json:"size"`
}

type UploadStatus struct {
	// How many retries have already failed
	// Use by the operator to stop retries once .spec.retries amount is reached
	// +kubebuilder:default=0
	FailedRetries *int `json:"failedRetries"`
	// If true, it means that the upload script was executed without errors
	// +kubebuilder:default=false
	Success *bool `json:"success"`
	// How long did it take to upload the backup of a  database (in seconds)
	// +kubebuilder:default=0
	Duration *int32 `json:"duration"`
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
