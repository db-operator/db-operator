/*
 * Copyright 2023 Nikolai Rodionov (allanger)
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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DBUserSpec defines the desired state of DBUser
type DBUserSpec struct {
	// DatabaseRef should contain a name of a Database to create a user there
	// Database should be in the same namespace with the user
	DatabaseRef string `json:"databaseRef"`
	// Username to use for creating a user
	Username string `json:"username"`
	// AccessType that should be given to a user
	// Currently only readOnly and readWrite are supported by the operator
	AccessType string `json:"accessType"`
}

// DBUserStatus defines the observed state of DBUser
type DBUserStatus struct {
	Phase        string      `json:"phase"`
	Status       bool        `json:"status"`
	InstanceRef  *DbInstance `json:"instanceRef"`
	DatabaseName string      `json:"database"`
	UserName     string      `json:"user"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DBUser is the Schema for the dbusers API
type DBUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DBUserSpec   `json:"spec,omitempty"`
	Status DBUserStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DBUserList contains a list of DBUser
type DBUserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DBUser `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DBUser{}, &DBUserList{})
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
