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
	"k8s.io/apimachinery/pkg/types"
)

// NamespacedName is a fork of the kubernetes api type of the same name.
// Sadly this is required because CRD structs must have all fields json tagged and the kubernetes type is not tagged.
type NamespacedName struct {
	Namespace string `json:"Namespace"`
	Name      string `json:"Name"`
}

// ToKubernetesType converts our local type to the kubernetes API equivalent.
func (nn *NamespacedName) ToKubernetesType() types.NamespacedName {
	if nn == nil {
		return types.NamespacedName{}
	}

	return types.NamespacedName{
		Name:      nn.Name,
		Namespace: nn.Namespace,
	}
}

// One generated credential entry.
type Template struct {
	// Data key written to the generated Secret or ConfigMap.
	Name string `json:"name"`
	// Go template evaluated with database connection values and template helper functions.
	Template string `json:"template"`
	// Writes the entry to a Secret when true and to a ConfigMap when false.
	// DbUser templates must set this field to true.
	Secret bool `json:"secret"`
}

type Templates []*Template

// CredentialsMetadata contains additional metadata that should be applied
// to Kubernetes objects created from credentials configuration.
//
// At the moment, this is used for Secret resources created for Database
// and DbUser credentials.
type CredentialsMetadata struct {
	// Labels to merge into the generated credential Secret.
	// Values in this map replace existing values for the same keys.
	ExtraLabels map[string]string `json:"extraLabels,omitempty"`

	// Annotations to merge into the generated credential Secret.
	// Values in this map replace existing values for the same keys.
	ExtraAnnotations map[string]string `json:"extraAnnotations,omitempty"`
}

// TODO(@allanger): Field .spec.secretName should be moved here in the v1beta2 version

// Generated credential data and Secret metadata.
type Credentials struct {
	// Additional data entries for generated Secrets and ConfigMaps.
	Templates Templates `json:"templates,omitempty"`

	// Labels and annotations on the generated credential Secret.
	Metadata *CredentialsMetadata `json:"metadata,omitempty"`
}
