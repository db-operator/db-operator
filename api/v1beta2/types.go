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

package v1beta2

// Tempaltes to add custom entries to ConfigMaps and Secrets
type Template struct {
	Name     string `json:"name"`
	Template string `json:"template"`
}

type Templates []*Template

// Credentials should be used to setup everything relates to k8s secrets and configmaps
type Credentials struct {
	// A secret with database credentials
	SecretName string `json:"secretName"`
	// When set to true, a secret with credentials will have
	// an owner reference of a Database CR. That means that once
	// a Database is removed, the secret will follow
	SetOwnerReference bool `json:"setOwnerReference"`
	// Templates to add custom entries to ConfigMaps and Secrets
	Templates Templates `json:"templates,omitempty"`
}

// +kubebuilder:validation:Enum:=postgres;mysql
type Engine string
