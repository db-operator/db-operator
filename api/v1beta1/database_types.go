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

	"github.com/db-operator/db-operator/api/v1beta2"
	"github.com/db-operator/db-operator/pkg/consts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// DatabaseSpec defines the desired state of Database
type DatabaseSpec struct {
	SecretName        string            `json:"secretName"`
	Instance          string            `json:"instance"`
	DeletionProtected bool              `json:"deletionProtected"`
	Backup            DatabaseBackup    `json:"backup"`
	SecretsTemplates  map[string]string `json:"secretsTemplates,omitempty"`
	Postgres          Postgres          `json:"postgres,omitempty"`
	Cleanup           bool              `json:"cleanup,omitempty"`
	Credentials       Credentials       `json:"credentials,omitempty"`
}

// Postgres struct should be used to provide resource that only applicable to postgres
type Postgres struct {
	Extensions []string `json:"extensions,omitempty"`
	// If set to true, the public schema will be dropped after the database creation
	DropPublicSchema bool `json:"dropPublicSchema,omitempty"`
	// Specify schemas to be created. The user created by db-operator will have all access on them.
	Schemas []string `json:"schemas,omitempty"`
	// Let user create database from template
	Template string `json:"template,omitempty"`
}

// DatabaseStatus defines the observed state of Database
type DatabaseStatus struct {
	// Important: Run "make generate" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	Status                bool                `json:"status"`
	MonitorUserSecretName string              `json:"monitorUserSecret,omitempty"`
	ProxyStatus           DatabaseProxyStatus `json:"proxyStatus,omitempty"`
	DatabaseName          string              `json:"database"`
	UserName              string              `json:"user"`
	Engine                string              `json:"engine"`
	OperatorVersion       string              `json:"operatorVersion,omitempty"`
}

// DatabaseProxyStatus defines whether proxy for database is enabled or not
// if so, provide information
type DatabaseProxyStatus struct {
	Status      bool   `json:"status"`
	ServiceName string `json:"serviceName"`
	SQLPort     int32  `json:"sqlPort"`
}

// DatabaseBackup defines the desired state of backup and schedule
type DatabaseBackup struct {
	Enable bool   `json:"enable"`
	Cron   string `json:"cron"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=db
// +kubebuilder:printcolumn:name="Status",type=boolean,JSONPath=`.status.status`,description="current db status"
// +kubebuilder:printcolumn:name="Protected",type=boolean,JSONPath=`.spec.deletionProtected`,description="If database is protected to not get deleted."
// +kubebuilder:printcolumn:name="DBInstance",type=string,JSONPath=`.spec.instance`,description="instance reference"
// +kubebuilder:printcolumn:name="OperatorVersion",type=string,JSONPath=`.status.operatorVersion`,description="db-operator version of last full reconcile"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`,description="time since creation of resource"

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

// Function to mark the Database as a hub
func (db *Database) Hub() {}

// ConvertTo converts this v1beta1 to v1beta2. (upgrade)
func (db *Database) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta2.Database)
	var newTemplates v1beta2.Templates
	for _, oldTemplate := range db.Spec.Credentials.Templates {
		newTemplates = append(newTemplates, &v1beta2.Template{
			Name:     oldTemplate.Name,
			Template: oldTemplate.Template,
		})
	}

	dst.ObjectMeta = db.ObjectMeta
	dst.Spec = v1beta2.DatabaseSpec{
		Instance:          db.Spec.Instance,
		DeletionProtected: db.Spec.DeletionProtected,
		Postgres: v1beta2.Postgres{
			Extensions:       db.Spec.Postgres.Extensions,
			DropPublicSchema: db.Spec.Postgres.DropPublicSchema,
			Schemas:          db.Spec.Postgres.Schemas,
			Params: v1beta2.PostgresDatabaseParams{
				Template: db.Spec.Postgres.Template,
			},
		},
		Credentials: v1beta2.Credentials{
			SecretName:        db.Spec.SecretName,
			SetOwnerReference: db.Spec.Cleanup,
			Templates:         newTemplates,
		},
	}
	return nil
}

// ConvertFrom converts from the Hub version (v1beta2) to (v1beta1). (downgrade)
func (dst *Database) ConvertFrom(srcRaw conversion.Hub) error {
	db := srcRaw.(*v1beta2.Database)
	var newTemplates Templates
	for _, oldTemplate := range db.Spec.Credentials.Templates {
		newTemplates = append(newTemplates, &Template{
			Name:     oldTemplate.Name,
			Template: oldTemplate.Template,
		})
	}

	dst.ObjectMeta = db.ObjectMeta
	dst.Spec = DatabaseSpec{
		SecretName:        db.Spec.Credentials.SecretName,
		Instance:          db.Spec.Instance,
		DeletionProtected: db.Spec.DeletionProtected,
		Postgres: Postgres{
			Extensions:       db.Spec.Postgres.Extensions,
			DropPublicSchema: db.Spec.Postgres.DropPublicSchema,
			Schemas:          db.Spec.Postgres.Schemas,
			Template:         db.Spec.Postgres.Params.Template,
		},
		Cleanup: db.Spec.Credentials.SetOwnerReference,
		Credentials: Credentials{
			Templates: newTemplates,
		},
	}
	return nil
}
