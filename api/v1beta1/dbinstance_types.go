/*
 * Copyright 2021 kloeckner.i GmbH
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

package v1beta1

import (
	"errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// Connection settings for the database server.
type DbInstanceSpec struct {
	// Important: Run "make generate" to regenerate code after modifying this file

	// Database engine. Supported values are postgres and mysql.
	// This field is immutable.
	Engine string `json:"engine"`
	// Reference to the Secret containing database administrator credentials. The optional user key
	// defaults to postgres for PostgreSQL and root for MySQL. PostgreSQL passwords are read
	// from password, postgresql-password, or postgresql-postgres-password; MySQL passwords
	// are read from password or mysql-root-password.
	AdminUserSecret NamespacedName `json:"adminSecretRef"`
	Backup        DbInstanceBackup        `json:"backup,omitempty"`
	Monitoring    DbInstanceMonitoring    `json:"monitoring,omitempty"`
	SSLConnection DbInstanceSSLConnection `json:"sslConnection,omitempty"`
	// Database roles that DbUser resources may request in extraPrivileges.
	// ALL PRIVILEGES is not allowed.
	AllowedPrivileges []string `json:"allowedPrivileges,omitempty"`
	// Permits Database resources to grant access to existing users
	// through their extraGrants field.
	AllowExtraGrants bool `json:"allowExtraGrants,omitempty"`
	// Provides values to credential templates through the InstanceVar function.
	// For example, a template can expose the address of a read-only PostgreSQL replica.
	InstanceVars     map[string]string `json:"instanceVars,omitempty"`
	DbInstanceSource `json:",inline"`
}

// DbInstanceSource represents the source of an instance.
// Only one of its members may be specified.
type DbInstanceSource struct {
	Google  *GoogleInstance  `json:"google,omitempty" protobuf:"bytes,1,opt,name=google"`
	Generic *GenericInstance `json:"generic,omitempty" protobuf:"bytes,2,opt,name=generic"`
}

// Observed state of the database server connection.
type DbInstanceStatus struct {
	// Important: Run "make generate" to regenerate code after modifying this file

	// One of Validating, Creating, Broadcasting, ProxyCreating, or Running.
	Phase string `json:"phase"`
	// True when the operator can connect to the database server.
	Status bool `json:"status"`
	// Contains DB_CONN, DB_PORT, and DB_PUBLIC_IP values discovered for the instance.
	Info      map[string]string `json:"info,omitempty"`
	Checksums map[string]string `json:"checksums,omitempty"`
}

// Google Cloud SQL instance managed through the Google API.
//
// Deprecated: Use generic instead. Google instances will be removed in v1beta2.
type GoogleInstance struct {
	InstanceName string `json:"instance"`
	// Reference to a ConfigMap containing the Google instance configuration.
	ConfigmapName NamespacedName `json:"configmapRef"`
	// Overrides the Google SQL Admin API endpoint.
	APIEndpoint string `json:"apiEndpoint,omitempty"`
	// Reference to the Secret containing Google API client credentials.
	ClientSecret NamespacedName `json:"clientSecretRef,omitempty"`
}

// BackendServer defines backend database server
type BackendServer struct {
	Host          string `json:"host"`
	Port          uint16 `json:"port"`
	MaxConnection uint16 `json:"maxConn"`
	ReadOnly      bool   `json:"readonly,omitempty"`
}

// Existing database server reachable by address and port.
type GenericInstance struct {
	// Address db-operator uses to connect to the database server.
	// It cannot be set with hostFrom.
	Host     string   `json:"host,omitempty"`
	HostFrom *FromRef `json:"hostFrom,omitempty"`
	// Database server port. It cannot be set with portFrom.
	Port     uint16   `json:"port,omitempty"`
	PortFrom *FromRef `json:"portFrom,omitempty"`
	// Externally reachable address exposed in generated credentials.
	// It cannot be set with publicIpFrom.
	PublicIP     string   `json:"publicIp,omitempty"`
	PublicIPFrom *FromRef `json:"publicIpFrom,omitempty"`
	// BackupAddress used by backup jobs. When empty, backup jobs use host.
	BackupHost string `json:"backupHost,omitempty"`
}

// Reference to one data entry in a Secret or ConfigMap.
type FromRef struct {
	// Referenced resource kind. Supported values are Secret and ConfigMap.
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	// Data key whose value should be read.
	Key string `json:"key"`
}

func (fr *FromRef) ToKubernetesType() k8stypes.NamespacedName {
	if fr == nil {
		return k8stypes.NamespacedName{}
	}

	return k8stypes.NamespacedName{
		Name:      fr.Name,
		Namespace: fr.Namespace,
	}
}

// Storage used by Database backup jobs.
type DbInstanceBackup struct {
	Bucket string `json:"bucket"`
}

// Database monitoring settings.
type DbInstanceMonitoring struct {
	// Creates and configures a monitoring user for databases on this instance.
	Enabled bool `json:"enabled"`
}

// TLS settings for connections from db-operator to the database server.
type DbInstanceSSLConnection struct {
	Enabled bool `json:"enabled"`
	// Accepts a server certificate without verifying its certificate authority or hostname.
	SkipVerify bool `json:"skip-verify"`
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

func init() {
	SchemeBuilder.Register(&DbInstance{}, &DbInstanceList{})
}

// ValidateEngine checks if defined engine by DbInstance object is supported by db-operator
func (dbin *DbInstance) ValidateEngine() error {
	if (dbin.Spec.Engine == "mysql") || (dbin.Spec.Engine == "postgres") {
		return nil
	}

	return errors.New("not supported engine type")
}

// ValidateBackend checks if backend type of instance is defined properly
// returns error when more than one backend types are defined
// or when no backend type is defined
func (dbin *DbInstance) ValidateBackend() error {
	source := dbin.Spec.DbInstanceSource

	if (source.Google == nil) && (source.Generic == nil) {
		return errors.New("no instance type defined")
	}

	numSources := 0

	if source.Google != nil {
		numSources++
	}

	if source.Generic != nil {
		numSources++
	}

	if numSources > 1 {
		return errors.New("may not specify more than 1 instance type")
	}

	return nil
}

// GetBackendType returns type of instance infrastructure.
// Infrastructure where database is running ex) google cloud sql, generic instance
func (dbin *DbInstance) GetBackendType() (string, error) {
	err := dbin.ValidateBackend()
	if err != nil {
		return "", err
	}

	source := dbin.Spec.DbInstanceSource

	if source.Google != nil {
		return "google", nil
	}

	if source.Generic != nil {
		return "generic", nil
	}

	return "", errors.New("no backend type defined")
}

// IsMonitoringEnabled returns boolean value if monitoring is enabled for the instance
func (dbin *DbInstance) IsMonitoringEnabled() bool {
	return dbin.Spec.Monitoring.Enabled
}

// DbInstances don't have the cleanup feature
func (dbin *DbInstance) IsCleanup() bool {
	return false
}

func (dbin *DbInstance) IsDeleted() bool {
	return dbin.GetDeletionTimestamp() != nil
}

// This method isn't supported by dbin
func (dbin *DbInstance) GetSecretName() string {
	return ""
}

func (db *DbInstance) Hub() {
	// Function to mark the DbInstance as a hub
}
