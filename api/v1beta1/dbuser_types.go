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

package v1beta1

import (
	"fmt"
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// User and access settings for the referenced Database.
type DbUserSpec struct {
	// Name of the Database that the user can access.
	// The Database must be in the same namespace as the DbUser.
	DatabaseRef string `json:"databaseRef"`
	// Access level to grant. Supported values are readOnly and readWrite.
	AccessType string `json:"accessType"`
	// Name of the Secret that stores the generated user credentials.
	SecretName string `json:"secretName"`
	// Additional database roles to grant to the user.
	// Every role must also appear in the referenced DbInstance's allowedPrivileges list.
	// ALL PRIVILEGES is not allowed. This feature is experimental.
	ExtraPrivileges []string    `json:"extraPrivileges,omitempty"`
	Credentials     Credentials `json:"credentials,omitempty"`
	// Adds this DbUser as an owner of the Kubernetes resources it creates.
	// Kubernetes garbage collection then removes those resources with the DbUser.
	Cleanup bool `json:"cleanup,omitempty"`
	// TODO: Default should be false, but not to introduce breaking
	//       changes it's now set to true. It should be changed in
	//       in the next API version

	// Grants the new user to the PostgreSQL administrator role.
	// This is commonly required when the managed administrator is not a superuser,
	// such as on Azure Database for PostgreSQL. It may need to be false when using
	// roles such as rds_iam on Amazon RDS. It has no effect on MySQL.
	// +kubebuilder:default=true
	// +optional
	GrantToAdmin bool `json:"grantToAdmin"`
	// Name of an existing database user to manage instead of creating a user.
	// The operator grants and revokes permissions for this user but does not create or delete it.
	ExistingUser string `json:"existingUser,omitempty"`
}

// Observed state of the database user.
type DbUserStatus struct {
	Status       bool   `json:"status"`
	DatabaseName string `json:"database"`
	UserName     string `json:"user,omitempty"`
	// True after the operator has created the user or begun managing an existing user.
	Created         bool   `json:"created"`
	OperatorVersion string `json:"operatorVersion,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Status",type=boolean,JSONPath=`.status.status`,description="current dbuser status"
//+kubebuilder:printcolumn:name="DatabaseName",type=string,JSONPath=`.spec.databaseRef`,description="To which database user should have access"
//+kubebuilder:printcolumn:name="AccessType",type=string,JSONPath=`.spec.accessType`,description="A type of access the user has"
//+kubebuilder:printcolumn:name="OperatorVersion",type=string,JSONPath=`.status.operatorVersion`,description="db-operator version of last full reconcile"
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`,description="time since creation of resource"

// DbUser is the Schema for the dbusers API
type DbUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DbUserSpec   `json:"spec,omitempty"`
	Status DbUserStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DbUserList contains a list of DbUser
type DbUserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DbUser `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DbUser{}, &DbUserList{})
}

// Access types that are supported by the operator
const (
	READONLY  = "readOnly"
	READWRITE = "readWrite"
)

// IsAccessTypeSupported returns an error if access type is not supported
func IsAccessTypeSupported(wantedAccessType string) error {
	supportedAccessTypes := []string{READONLY, READWRITE}
	if slices.Contains(supportedAccessTypes, wantedAccessType) {
		return nil
	}
	return fmt.Errorf("the provided access type is not supported by the operator: %s - please choose one of these: %v",
		wantedAccessType,
		supportedAccessTypes,
	)
}

// IsCleanup reports whether Kubernetes resources created for this DbUser should use owner references.
func (dbu *DbUser) IsCleanup() bool {
	return dbu.Spec.Cleanup
}

func (dbu *DbUser) IsDeleted() bool {
	return dbu.GetDeletionTimestamp() != nil
}

func (dbu *DbUser) GetSecretName() string {
	return dbu.Spec.SecretName
}
