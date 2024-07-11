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

	"github.com/db-operator/db-operator/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// DbUserSpec defines the desired state of DbUser
type DbUserSpec struct {
	// DatabaseRef should contain a name of a Database to create a user there
	// Database should be in the same namespace with the user
	DatabaseRef string `json:"databaseRef"`
	// AccessType that should be given to a user
	// Currently only readOnly and readWrite are supported by the operator
	AccessType string `json:"accessType"`
	// SecretName name that should be used to save user's credentials
	SecretName string `json:"secretName"`
	// A list of additional roles that should be added to the user
	ExtraPrivileges []string    `json:"extraPrivileges,omitempty"`
	Credentials     Credentials `json:"credentials,omitempty"`
	Cleanup         bool        `json:"cleanup,omitempty"`
	// Should the user be granted to the admin user
	// For example, it should be set to true on Azure instance,
	// because the admin given by them is not a super user,
	// but should be set to false on AWS, when rds_iam extra
	// privilege is added
	// By default is set to true
	// Only applies to Postgres, doesn't have any effect on Mysql
	// TODO: Default should be false, but not to introduce breaking
	//       changes it's now set to true. It should be changed in
	//       in the next API version
	// +kubebuilder:default=true
	// +optional
	GrantToAdmin bool `json:"grantToAdmin"`
}

// DbUserStatus defines the observed state of DbUser
type DbUserStatus struct {
	Status       bool   `json:"status"`
	DatabaseName string `json:"database"`
	// It's required to let the operator update users
	Created bool `json:"created"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Status",type=boolean,JSONPath=`.status.status`,description="current dbuser status"
//+kubebuilder:printcolumn:name="DatabaseName",type=string,JSONPath=`.spec.databaseRef`,description="To which database user should have access"
//+kubebuilder:printcolumn:name="AccessType",type=string,JSONPath=`.spec.accessType`,description="A type of access the user has"
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`,description="time since creation of resos¡urce"

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
	for _, supportedAccessType := range supportedAccessTypes {
		if supportedAccessType == wantedAccessType {
			return nil
		}
	}
	return fmt.Errorf("the provided access type is not supported by the operator: %s - please chose one of these: %v",
		wantedAccessType,
		supportedAccessTypes,
	)
}

// DbUsers don't have cleanup feature implemented
func (dbu *DbUser) IsCleanup() bool {
	return dbu.Spec.Cleanup
}

func (dbu *DbUser) IsDeleted() bool {
	return dbu.GetDeletionTimestamp() != nil
}

func (dbu *DbUser) GetSecretName() string {
	return dbu.Spec.SecretName
}

// ConvertTo converts this v1beta1 to v1beta2. (upgrade)
func (dbuser *DbUser) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta2.DbUser)
	dst.ObjectMeta = dbuser.ObjectMeta
	var newTemplates v1beta2.Templates
	for _, oldTemplate := range dbuser.Spec.Credentials.Templates {
		newTemplates = append(newTemplates, &v1beta2.Template{
			Name:     oldTemplate.Name,
			Template: oldTemplate.Template,
		})
	}
	dst.Spec = v1beta2.DbUserSpec{
		DatabaseRef:     dbuser.Spec.DatabaseRef,
		AccessType:      dbuser.Spec.AccessType,
		ExtraPrivileges: dbuser.Spec.ExtraPrivileges,
		Credentials: v1beta2.Credentials{
			SecretName:        dbuser.Spec.SecretName,
			SetOwnerReference: dbuser.Spec.Cleanup,
			Templates:         newTemplates,
		},
		GrantToAdmin: dbuser.Spec.GrantToAdmin,
	}
	return nil
}

// ConvertFrom converts from the Hub version (v1beta2) to (v1beta1). (downgrade)
func (dst *DbUser) ConvertFrom(srcRaw conversion.Hub) error {
	dbuser := srcRaw.(*v1beta2.DbUser)
	dst.ObjectMeta = dbuser.ObjectMeta
	var newTemplates Templates
	for _, oldTemplate := range dbuser.Spec.Credentials.Templates {
		newTemplates = append(newTemplates, &Template{
			Name:     oldTemplate.Name,
			Template: oldTemplate.Template,
		})
	}
	dst.Spec = DbUserSpec{
		DatabaseRef:     dbuser.Spec.DatabaseRef,
		AccessType:      dbuser.Spec.AccessType,
		ExtraPrivileges: dbuser.Spec.ExtraPrivileges,
		SecretName:      dbuser.Spec.Credentials.SecretName,
		Cleanup:         dbuser.Spec.Credentials.SetOwnerReference,
		Credentials: Credentials{
			Templates: newTemplates,
		},
		GrantToAdmin: dbuser.Spec.GrantToAdmin,
	}
	return nil
}
