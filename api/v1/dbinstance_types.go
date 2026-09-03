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
	// When not set, db-operator will try to determine the engine itself.
	// +kubebuilder:validation:Enum=postgres;mysql
	Engine *string `json:"engine"`
	// How the operator should authenticate to the database instance.
	Auth *DbInstanceAuth `json:"auth"`
	// The endpoint of the database instance.
	Endpoint *DbInstanceEndpoint `json:"endpoint"`
	// ReadOnlyEndpoint is an optional endpoint that should point to a read only replica of a database server.
	// +optional
	ReadOnlyEndpoint *DbInstanceEndpoint `json:"readOnlyEndpoint,omitempty"`
	// InstanceVars can be used by any database/dbuser that are deployed
	// to this instance to build templated credentials with some generic values.
	// Can be used for example to provide a read only postgres replica url
	// +optional
	InstanceVars map[string]string `json:"instanceVars,omitempty"`
	// If set, only databases from allowed namespaces will be created on the instance.
	// +optional
	NamespaceFilters []string `json:"namespaceFilters,omitempty"`
	// If set, only databases from allowed namespaces will be created on the instance.
	// +optional
	GrantRules []*DbInstanceGrantRule `json:"grantRules,omitempty"`
	// If set, the operator will create backup jobs for database that are deployed to the namespaces
	// that will match the namespace regex
	// +optional
	// BackupRules []*DbInstanceBackupRule `json:"backupRules,omitempty"`
	// A list of privileges that are allowed to be set as Dbuser's extra privileges
	AllowedPrivileges []string `json:"allowedPrivileges,omitempty"`
}

// DbInstanceGrantRule defines a rule for granting access to a database.
type DbInstanceGrantRule struct {
	NamespaceRegex string `json:"namespace"`
	Role           string `json:"role"`
	AccessLevel    string `json:"accessLevel"`
}

// DbInstanceBackupRule defines a rule for backing up databases.
type DbInstanceBackupRule struct {
	Name           string `json:"name"`
	NamespaceRegex string `json:"namespace"`
	Cron           string `json:"cron"`
}

// DbInstanceAuth defines the authentication information for a database instance.
type DbInstanceAuth struct {
	Username *ValueSource `json:"username,omitempty"`
	Password *ValueSource `json:"password,omitempty"`
	// Will be supported in the future, but not yet implemented
	// Certificate *ValueSource `json:"certificate,omitempty"`
}

// DbInstanceEndpoint defines the endpoint information for a database instance.
type DbInstanceEndpoint struct {
	Host          *ValueSource             `json:"host,omitempty"`
	Port          *ValueSource             `json:"port,omitempty"`
	SSLConnection *DbInstanceSSLConnection `json:"sslConnection,omitempty"`
}

// DbInstanceSSLConnection defines whether connection from db-operator to instance has to be ssl or not
type DbInstanceSSLConnection struct {
	Enabled bool `json:"enabled"`
	// SkipVerify use SSL connection, but don't check against a CA
	SkipVerify bool `json:"skip-verify"`
}

// ValueSource is a helper struct to let users either provide a value directly or fetch a value from Secret/ConfigMap.
type ValueSource struct {
	Value     *string    `json:"value,omitempty"`
	ValueFrom *ValueFrom `json:"valueFrom,omitempty"`
}

// ValueFrom is a helper struct to let users fetch data from Secret/ConfigMap.
type ValueFrom struct {
	SecretKeyRef    *SecretOrCMRef `json:"secret,omitempty"`
	ConfigMapKeyRef *SecretOrCMRef `json:"configMap,omitempty"`
}

// SecretOrCMRef is a helper struct to let users fetch a value from Secret/ConfigMap.
type SecretOrCMRef struct {
	Namespace *string `json:"namespace,omitempty"`
	Name      *string `json:"name,omitempty"`
	Key       *string `json:"key,omitempty"`
}

// DbInstanceStatus defines the observed state of DbInstance.
type DbInstanceStatus struct {
	// Which engine is used as a backend for this instance
	// +kubebuilder:validation:Enum=postgres;mysql
	Engine string `json:"engine,omitempty"`
	// Which version of the engine is used as a backend for this instance
	Version string `json:"version,omitempty"`
	// VersionTTL is the time when the version of the backend will be checked again
	VersionTTL int64 `json:"versionTTL,omitempty"`
	// Which version of the db-operator is used to manage this instance
	OperatorVersion string `json:"operatorVersion,omitempty"`
	// ServerStatus contains information about the databases and users on this instance.
	ServerStatus *DbInstanceServerStatus `json:"serverStatus"`
	// MainEndpoint contains information about the host and port of this instance.
	MainEndpoint *DbInstanceServerData `json:"mainEndpoint,omitempty"`
	// ReadOnlyEndpoint contains information about the host and port of a read only replica of this instance.
	// +optional
	ReadOnlyEndpoint *DbInstanceServerData `json:"readOnlyEndpoint,omitempty"`
	Ready            bool                  `json:"ready,omitempty"`
	// WatchedResources is a list of resources that are being watched by the db-operator for this instance.
	WatchedResources []string               `json:"watchedResources,omitempty"`
	NamespaceFilters []string               `json:"namespaceFilters,omitempty"`
	AutoGrantRules   []*DbInstanceGrantRule `json:"autoGrantRules,omitempty"`
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

// DbInstanceServerStatus is a struct that holds information about the databases and users on a database instance.
type DbInstanceServerStatus struct {
	// Total number of databases created on this instance.
	DatabasesCount int `json:"databasesCount,omitempty"`
	// Total number of databases created on this instance by the db-operator.
	ManagedDatabasesCount int `json:"managedDatabasesCount"`
	// A list of databases on this instance.
	Databases []string `json:"databases,omitempty"`
	// A list of users on this instance.
	Users []string `json:"users,omitempty"`
	// When ready is true, an instance can be used by other controllers
}

// DbInstanceServerData is a struct that holds information about the host and port of a database instance.
type DbInstanceServerData struct {
	Host          string                  `json:"host,omitempty"`
	Port          uint16                  `json:"port,omitempty"`
	SSLConnection DbInstanceSSLConnection `json:"sslConnection,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
//+kubebuilder:resource:scope=Cluster,shortName=dbin
//+kubebuilder:printcolumn:name="Engine",type=string,JSONPath=`.status.engine`,description="Which engine is used as a backend for this instance"
//+kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.status.version`,description="Which version of the engine is used as a backend for this instance"
//+kubebuilder:printcolumn:name="Managed Databases Count",type=string,JSONPath=`.status.serverStatus.managedDatabasesCount`,description="How many databases on this instance are managed by the operator"
//+kubebuilder:printcolumn:name="Operator Version",type=string,JSONPath=`.status.operatorVersion`,description="Which version of the db-operator is used to manage this instance"
//+kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.ready`,description="	// When ready, an instance can be used by other controllers"

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
