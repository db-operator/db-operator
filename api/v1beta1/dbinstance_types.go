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

	"github.com/db-operator/db-operator/api/common"
	"github.com/db-operator/db-operator/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DbInstanceSpec defines the desired state of DbInstance
type DbInstanceSpec struct {
	// Important: Run "make generate" to regenerate code after modifying this file
	Engine          string                  `json:"engine"`
	AdminUserSecret NamespacedName          `json:"adminSecretRef"`
	Backup          DbInstanceBackup        `json:"backup,omitempty"`
	Monitoring      DbInstanceMonitoring    `json:"monitoring,omitempty"`
	SSLConnection   DbInstanceSSLConnection `json:"sslConnection,omitempty"`
	// A list of privileges that are allowed to be set as Dbuser's extra privileges
	AllowedPrivileges []string `json:"allowedPrivileges,omitempty"`
	DbInstanceSource  `json:",inline"`
}

// DbInstanceSource represents the source of an instance.
// Only one of its members may be specified.
type DbInstanceSource struct {
	Google  *GoogleInstance  `json:"google,omitempty" protobuf:"bytes,1,opt,name=google"`
	Generic *GenericInstance `json:"generic,omitempty" protobuf:"bytes,2,opt,name=generic"`
}

// DbInstanceStatus defines the observed state of DbInstance
type DbInstanceStatus struct {
	// Important: Run "make generate" to regenerate code after modifying this file
	Phase     string            `json:"phase"`
	Status    bool              `json:"status"`
	Info      map[string]string `json:"info,omitempty"`
	Checksums map[string]string `json:"checksums,omitempty"`
}

// GoogleInstance is used when instance type is Google Cloud SQL
// and describes necessary informations to use google API to create sql instances
type GoogleInstance struct {
	InstanceName  string         `json:"instance"`
	ConfigmapName NamespacedName `json:"configmapRef"`
	APIEndpoint   string         `json:"apiEndpoint,omitempty"`
	ClientSecret  NamespacedName `json:"clientSecretRef,omitempty"`
}

// BackendServer defines backend database server
type BackendServer struct {
	Host          string `json:"host"`
	Port          uint16 `json:"port"`
	MaxConnection uint16 `json:"maxConn"`
	ReadOnly      bool   `json:"readonly,omitempty"`
}

// GenericInstance is used when instance type is generic
// and describes necessary information to use instance
// generic instance can be any backend, it must be reachable by described address and port
type GenericInstance struct {
	Host         string          `json:"host,omitempty"`
	HostFrom     *common.FromRef `json:"hostFrom,omitempty"`
	Port         uint16          `json:"port,omitempty"`
	PortFrom     *common.FromRef `json:"portFrom,omitempty"`
	PublicIP     string          `json:"publicIp,omitempty"`
	PublicIPFrom *common.FromRef `json:"publicIpFrom,omitempty"`
	// BackupHost address will be used for dumping database for backup
	// Usually secondary address for primary-secondary setup or cluster lb address
	// If it's not defined, above Host will be used as backup host address.
	BackupHost string `json:"backupHost,omitempty"`
}

// DbInstanceBackup defines name of google bucket to use for storing database dumps for backup when backup is enabled
type DbInstanceBackup struct {
	Bucket string `json:"bucket"`
}

// DbInstanceMonitoring defines if exporter
type DbInstanceMonitoring struct {
	Enabled bool `json:"enabled"`
}

// DbInstanceSSLConnection defines whether connection from db-operator to instance has to be ssl or not
type DbInstanceSSLConnection struct {
	Enabled bool `json:"enabled"`
	// SkipVerify use SSL connection, but don't check against a CA
	SkipVerify bool `json:"skip-verify"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster,shortName=dbin
//+kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`,description="current phase"
//+kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.status`,description="health status"

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

// Function to mark the DbInstance as a hub
func (db *DbInstance) Hub() {}

// ConvertTo converts this v1beta1 to v1beta2. (upgrade)
func (dbin *DbInstance) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta2.DbInstance)
	dst.ObjectMeta = dbin.ObjectMeta
	dst.Spec.AdminCredentials = &v1beta2.AdminCredentials{
		UsernameFrom: &common.FromRef{
			Kind:      "Secret",
			Name:      dbin.Spec.AdminUserSecret.Name,
			Namespace: dbin.Spec.AdminUserSecret.Namespace,
			Key:       "user",
		},
		PasswordFrom: &common.FromRef{
			Kind:      "Secret",
			Name:      dbin.Spec.AdminUserSecret.Name,
			Namespace: dbin.Spec.AdminUserSecret.Namespace,
			Key:       "password",
		},
	}
	dst.Spec.AllowedPrivileges = dbin.Spec.AllowedPrivileges
	dst.Spec.SSLConnection = v1beta2.DbInstanceSSLConnection(dbin.Spec.SSLConnection)
	if dbin.Spec.DbInstanceSource.Generic != nil {
		dst.Spec.InstanceData = &v1beta2.InstanceData{
			Host: dbin.Spec.Generic.Host,
			Port: dbin.Spec.Generic.Port,
		}
	} else if dbin.Spec.DbInstanceSource.Google != nil {
		return errors.New("google instances are not supported anymore, please conside migrating to other tools for providing them. E.g crossplane")
	}
	dst.Spec.Engine = v1beta2.Engine(dbin.Spec.Engine)
	return nil
}

// ConvertFrom converts from the Hub version (v1beta2) to (v1beta1). (downgrade)
func (dst *DbInstance) ConvertFrom(srcRaw conversion.Hub) error {
	dbin := srcRaw.(*v1beta2.DbInstance)
	dst.ObjectMeta = dbin.ObjectMeta
	dst.Spec.AdminUserSecret = NamespacedName{
		Namespace: dbin.Spec.AdminCredentials.UsernameFrom.Namespace,
		Name:      dbin.Spec.AdminCredentials.UsernameFrom.Name,
	}
	dst.Spec.SSLConnection = DbInstanceSSLConnection(dbin.Spec.SSLConnection)
	dst.Spec.Backup = DbInstanceBackup{}
	dst.Spec.DbInstanceSource.Generic = &GenericInstance{
		Host:         dbin.Spec.InstanceData.Host,
		HostFrom:     (*common.FromRef)(dbin.Spec.InstanceData.HostFrom),
		Port:         dbin.Spec.InstanceData.Port,
		PortFrom:     (*common.FromRef)(dbin.Spec.InstanceData.PortFrom),
		PublicIP:     "",
		PublicIPFrom: nil,
		BackupHost:   "",
	}
	dst.Spec.AllowedPrivileges = dbin.Spec.AllowedPrivileges
	dst.Spec.Engine = string(dbin.Spec.Engine)
	return nil
}
