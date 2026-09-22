/*
 * Copyright 2021 kloeckner.i GmbH
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

package v1beta1

import (
	"fmt"

	"github.com/db-operator/db-operator/v2/pkg/consts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Desired configuration of the database and its primary user.
type DatabaseSpec struct {
	// Name of the Secret that stores the generated database credentials.
	SecretName string `json:"secretName"`
	// Name of the cluster-scoped DbInstance that hosts the database.
	// This field is immutable.
	Instance string `json:"instance"`
	// Keeps the database and its primary user in the database backend
	// when the Database resource is deleted.
	DeletionProtected bool `json:"deletionProtected"`
	Backup DatabaseBackup `json:"backup,omitempty"`
	// Legacy credential templates indexed by Secret data key.
	// It cannot be used with credentials.templates.
	//
	// Deprecated: Use credentials.templates instead.
	SecretsTemplates map[string]string `json:"secretsTemplates,omitempty"`
	Postgres         Postgres          `json:"postgres,omitempty"`
	// Adds this Database as an owner of the Kubernetes resources it creates.
	// Kubernetes garbage collection then removes those resources with the Database.
	Cleanup     bool        `json:"cleanup,omitempty"`
	Credentials Credentials `json:"credentials,omitempty"`
	// Grants other existing database users access to this database.
	// The referenced DbInstance must set allowExtraGrants to true.
	ExtraGrants []*ExtraGrant `json:"extraGrants,omitempty"`
	// Name of an existing database user to use as the primary user.
	// The operator grants and revokes permissions for this user but does not create or delete it.
	ExistingUser string `json:"existingUser,omitempty"`
}

type ExtraGrant struct {
	User string `json:"user"`
	// Access level to grant. Supported values are readOnly and readWrite.
	AccessType string `json:"accessType"`
}

// Settings that apply only to PostgreSQL databases.
type Postgres struct {
	Extensions []string `json:"extensions,omitempty"`
	// Removes the public schema after the database is created.
	DropPublicSchema bool `json:"dropPublicSchema,omitempty"`
	// Schemas to create. The primary user receives full access to each schema.
	Schemas []string `json:"schemas,omitempty"`
	// PostgreSQL template database used to create this database.
	// This field is immutable.
	Template string `json:"template,omitempty"`
}

// Observed state of the database.
type DatabaseStatus struct {
	// Important: Run "make generate" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// True after the database reconciles successfully.
	Status                bool                `json:"status"`
	MonitorUserSecretName string              `json:"monitorUserSecret,omitempty"`
	ProxyStatus           DatabaseProxyStatus `json:"proxyStatus,omitempty"`
	DatabaseName          string              `json:"database"`
	UserName              string              `json:"user"`
	Engine                string              `json:"engine"`
	// The db-operator version that last completed reconciliation.
	OperatorVersion string `json:"operatorVersion,omitempty"`
	// Records the grants applied during the last successful reconciliation.
	ExtraGrants []*ExtraGrant `json:"extraGrants,omitempty"`
}

// Connection proxy state and service details.
type DatabaseProxyStatus struct {
	// True when the proxy is ready.
	Status      bool   `json:"status"`
	ServiceName string `json:"serviceName"`
	SQLPort     int32  `json:"sqlPort"`
}

// Scheduled database dumps.
type DatabaseBackup struct {
	// Creates a CronJob that dumps the database on the configured schedule.
	Enable bool `json:"enable"`
	// CronJob schedule expression.
	Cron string `json:"cron"`
	// Name of a Secret whose data is exposed to the backup container as environment variables.
	EnvFromSecret string `json:"envFromSecret,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=db
// +kubebuilder:printcolumn:name="Status",type=boolean,JSONPath=`.status.status`,description="current db status"
// +kubebuilder:printcolumn:name="Protected",type=boolean,JSONPath=`.spec.deletionProtected`,description="If database is protected to not get deleted."
// +kubebuilder:printcolumn:name="DBInstance",type=string,JSONPath=`.spec.instance`,description="instance reference"
// +kubebuilder:printcolumn:name="OperatorVersion",type=string,JSONPath=`.status.operatorVersion`,description="db-operator version of last full reconcile"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`,description="time since creation of resource"
// +kubebuilder:storageversion
// Database is the Schema for the databases API
type Database struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DatabaseSpec   `json:"spec,omitempty"`
	Status DatabaseStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DatabaseList contains a list of Database
type DatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Database `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Database{}, &DatabaseList{})
}

// GetProtocol returns the protocol that is required for connection (postgresql or mysql)
func (db *Database) GetProtocol() (string, error) {
	switch db.Status.Engine {
	case consts.ENGINE_POSTGRES:
		return "postgresql", nil
	case consts.ENGINE_MYSQL:
		return db.Status.Engine, nil
	default:
		return "", fmt.Errorf("unknown engine %s", db.Status.Engine)
	}
}

func (db *Database) IsCleanup() bool {
	return db.Spec.Cleanup
}

func (db *Database) IsDeleted() bool {
	return db.GetDeletionTimestamp() != nil
}

func (db *Database) GetSecretName() string {
	return db.Spec.SecretName
}

func (db *Database) ToClientObject() client.Object {
	return db
}

// AccessSecretName returns string value to define name of the secret resource for accessing instance
func (db *Database) InstanceAccessSecretName() string {
	return "dbin-" + db.Spec.Instance + "-access-secret"
}

func (extraGrant *ExtraGrant) IsExtraGrant(extraGrants []*ExtraGrant) bool {
	for _, existingExtraGrant := range extraGrants {
		if existingExtraGrant.User == extraGrant.User &&
			existingExtraGrant.AccessType == extraGrant.AccessType {
			return true
		}
	}
	return false
}

// Function to mark the Database as a hub
func (db *Database) Hub() {}
