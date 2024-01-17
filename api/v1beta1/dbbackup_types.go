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
	// A name of a database object that manages a database
	// that one wants to backup
	Database string `json:"database"`
	// A cron expression to be used in a cronjob
	Cron string	`json:"cron"`
}

// DbBackupStatus defines the observed state of DbBackup
type DbBackupStatus struct {
	LastBackupStatus bool `json:"lastBackupStatus"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DbBackup is the Schema for the dbbackups API
type DbBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DbBackupSpec   `json:"spec,omitempty"`
	Status DbBackupStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DbBackupList contains a list of DbBackup
type DbBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DbBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DbBackup{}, &DbBackupList{})
}
