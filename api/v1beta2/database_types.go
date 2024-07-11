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
	"fmt"

	"github.com/db-operator/db-operator/pkg/consts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DatabaseSpec defines the desired state of Database
type DatabaseSpec struct {
	// A name of the DbInstance to which this Database should be assigned
	Instance string `json:"instance"`
	// If set to true, db-operator won't remove the database on the server
	// when the Database resource is removed from Kubernetes
	DeletionProtected bool        `json:"deletionProtected"`
	Postgres          Postgres    `json:"postgres,omitempty"`
	Mysql             Mysql       `json:"mysql,omitempty"`
	Credentials       Credentials `json:"credentials,omitempty"`
}

// Postgres struct should be used to provide resource that only applicable to postgres
type Postgres struct {
	Extensions []string `json:"extensions,omitempty"`
	// If set to true, the public schema will be dropped after the database creation
	DropPublicSchema bool `json:"dropPublicSchema,omitempty"`
	// Specify schemas to be created. The user created by db-operator will have all access on them.
	Schemas []string `json:"schemas,omitempty"`
	// Define params for database creation
	Params PostgresDatabaseParams `json:"params,omitempty"`
}

// Mysql struct should be used to provide resource that only applicable to mysql
type Mysql struct {
	// Define params for database creation
	Params MysqlDatabaseParams `json:"params,omitempty"`
}

/*
 * Parameters that should be set during the database creation or update
 * Please refer to official docs
 * https://www.postgresql.org/docs/current/sql-createdatabase.html
 * https://www.postgresql.org/docs/current/sql-alterdatabase.html
 * Any options that can be applied only during the creation of a database
 * should be immutable.
 */
type PostgresDatabaseParams struct {
	// The name of the template from which to create the new database, or DEFAULT to use the default template (template1).
	// (Only create)
	Template string `json:"template,omitempty"`
	// Character set encoding to use in the new database. Specify a string constant (e.g., 'SQL_ASCII'),
	// or an integer encoding number, or DEFAULT to use the default encoding (namely, the encoding of the template database).
	// The character sets supported by the PostgreSQL server are described in Section 24.3.1. See below for additional restrictions.
	// (Only create)
	Encoding string `json:"encoding,omitempty"`
}

/*
 * Parameters that should be set during the database creation or update
 * Please refer to official docs
 * https://dev.mysql.com/doc/refman/8.4/en/create-database.html
 * https://dev.mysql.com/doc/refman/8.4/en/alter-database.html
 * Any options that can be applied only during the creation of a database
 * should be immutable.
 */
type MysqlDatabaseParams struct {
	// The CHARACTER SET option specifies the default database character set.
	CharacterSet string `json:"charatcterSet,omitempty"`
	// The COLLATE option specifies the default database collation.
	Collate string `json:"collage,omitempty"`
	// The ENCRYPTION option defines the default database encryption,
	// which is inherited by tables created in the database.
	// The permitted values are 'Y' (encryption enabled) and 'N' (encryption disabled).
	Encryption MysqlEncryption `json:"encyption,omitempty"`
}

// +kubebuilder:validation:Enum:=Y,N
type MysqlEncryption string

// DatabaseStatus defines the observed state of Database
type DatabaseStatus struct {
	Status                bool   `json:"status"`
	MonitorUserSecretName string `json:"monitorUserSecret,omitempty"`
	DatabaseName          string `json:"database"`
	UserName              string `json:"user"`
	Engine                Engine `json:"engine"`
	OperatorVersion       string `json:"operatorVersion,omitempty"`
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

func (db *Database) GetProtocol() (string, error) {
	switch db.Status.Engine {
	case consts.ENGINE_POSTGRES:
		return "postgresql", nil
	case consts.ENGINE_MYSQL:
		return string(db.Status.Engine), nil
	default:
		return "", fmt.Errorf("unknown engine %s", db.Status.Engine)
	}
}

func (db *Database) IsCleanup() bool {
	return db.Spec.Credentials.SetOwnerReference
}

func (db *Database) IsDeleted() bool {
	return db.GetDeletionTimestamp() != nil
}

func (db *Database) GetSecretName() string {
	return db.Spec.Credentials.SecretName
}

func (db *Database) ToClientObject() client.Object {
	return db
}

// AccessSecretName returns string value to define name of the secret resource for accessing instance
// TODO: What does it mean?
func (db *Database) InstanceAccessSecretName() string {
	return "dbin-" + db.Spec.Instance + "-access-secret"
}

func (db *Database) Hub() {}
