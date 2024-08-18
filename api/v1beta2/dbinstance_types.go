/*
 * Copyright 2023 DB-Operator Authors
 *
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

package v1beta2

import (
	"github.com/db-operator/db-operator/api/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DbInstanceSpec defines the desired state of DbInstance
type DbInstanceSpec struct {
	// Which database should be used as a backend
	Engine Engine `json:"engine"`
	// Which user should be used to connecto to the database
	AdminCredentials *AdminCredentials    `json:"adminCredentials"`
	Monitoring       DbInstanceMonitoring `json:"monitoring,omitempty"`
	// A list of privileges that are allowed to be set as Dbuser's extra privileges
	AllowedPrivileges []string                `json:"allowedPrivileges,omitempty"`
	SSLConnection     DbInstanceSSLConnection `json:"sslConnection,omitempty"`
	// A connection data for the db-operator to find the server
	// It's immutable, if a dbinstance has Status.Connected = true
	InstanceData *InstanceData `json:"instanceData"`
}

// InstanceData is used when instance type is generic
// and describes necessary informations to use instance
// generic instance can be any backend, it must be reachable by described address and port
type InstanceData struct {
	Host     string          `json:"host,omitempty"`
	HostFrom *common.FromRef `json:"hostFrom,omitempty"`
	Port     uint16          `json:"port,omitempty"`
	PortFrom *common.FromRef `json:"portFrom,omitempty"`
}

type AdminCredentials struct {
	UsernameFrom *common.FromRef `json:"usernameFrom"`
	PasswordFrom *common.FromRef `json:"passwordFrom"`
}

// DbInstanceBackup defines name of google bucket to use for storing database dumps for backup when backup is enabled
type DbInstanceBackup struct {
	Bucket string `json:"bucket"`
}

// DbInstanceMonitoring defines if exporter
type DbInstanceMonitoring struct {
	Enabled bool `json:"enabled"`
}

// DbInstanceSSLConnection defines weather connection from db-operator to instance has to be ssl or not
type DbInstanceSSLConnection struct {
	Enabled bool `json:"enabled"`
	// SkipVerity use SSL connection, but don't check against a CA
	SkipVerify bool `json:"skip-verify"`
}

// IsMonitoringEnabled returns boolean value if monitoring is enabled for the instance
func (dbin *DbInstance) IsMonitoringEnabled() bool {
	return dbin.Spec.Monitoring.Enabled
}

// NamespacedName is a fork of the kubernetes api type of the same name.
// Sadly this is required because CRD structs must have all fields json tagged and the kubernetes type is not tagged.
type NamespacedName struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// DbInstanceStatus defines the observed state of DbInstance
type DbInstanceStatus struct {
	// Is db-operator able to connect to the database server
	Connected bool `json:"connected"`
	// A database instance URL
	URL string `json:"url,omitempty"`
	// A database server port that will be used for creating databases
	Port int64 `json:"info,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster,shortName=dbin
//+kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`,description="current phase"
//+kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.status`,description="health status"
// +kubebuilder:storageversion

// DbInstance is the Schema for the dbinstances API
type DbInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DbInstanceSpec   `json:"spec,omitempty"`
	Status DbInstanceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DbInstanceList contains a list of DbInstance
type DbInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DbInstance `json:"items"`
}

// DbInstances don't have the cleanup feature
func (dbin *DbInstance) IsCleanup() bool {
	return false
}

func (dbin *DbInstance) IsDeleted() bool {
	return dbin.GetDeletionTimestamp() != nil
}

// This method isn's supported by dbin
func (dbin *DbInstance) GetSecretName() string {
	return ""
}

func init() {
	SchemeBuilder.Register(&DbInstance{}, &DbInstanceList{})
}

func (db *DbInstance) Hub() {}
