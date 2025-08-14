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

package common

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"

	kindav1beta1 "github.com/db-operator/db-operator/v2/api/v1beta1"
	"github.com/db-operator/db-operator/v2/pkg/utils/kci"
	crdv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

var OperatorVersion string

func IsDBChanged(dbcr *kindav1beta1.Database, databaseSecret *corev1.Secret) bool {
	annotations := dbcr.GetAnnotations()

	return annotations["checksum/spec"] != kci.GenerateChecksum(dbcr.Spec) ||
		annotations["checksum/secret"] != GenerateChecksumSecretValue(databaseSecret)
}

func AddDBChecksum(dbcr *kindav1beta1.Database, databaseSecret *corev1.Secret) {
	annotations := dbcr.GetAnnotations()
	if len(annotations) == 0 {
		annotations = make(map[string]string)
	}

	annotations["checksum/spec"] = kci.GenerateChecksum(dbcr.Spec)
	annotations["checksum/secret"] = GenerateChecksumSecretValue(databaseSecret)
	dbcr.SetAnnotations(annotations)
}

func GenerateChecksumSecretValue(databaseSecret *corev1.Secret) string {
	if databaseSecret == nil || databaseSecret.Data == nil {
		return ""
	}
	return kci.GenerateChecksum(databaseSecret.Data)
}

func IsDBInstanceSpecChanged(ctx context.Context, dbin *kindav1beta1.DbInstance) bool {
	checksums := dbin.Status.Checksums
	if checksums["spec"] != kci.GenerateChecksum(dbin.Spec) {
		return true
	}

	if backend, _ := dbin.GetBackendType(); backend == "google" {
		instanceConfig, _ := kci.GetConfigResource(ctx, dbin.Spec.Google.ConfigmapName.ToKubernetesType())
		if checksums["config"] != kci.GenerateChecksum(instanceConfig) {
			return true
		}
	}

	return false
}

func AddDBInstanceChecksumStatus(ctx context.Context, dbin *kindav1beta1.DbInstance) {
	checksums := dbin.Status.Checksums
	if len(checksums) == 0 {
		checksums = make(map[string]string)
	}
	checksums["spec"] = kci.GenerateChecksum(dbin.Spec)

	if backend, _ := dbin.GetBackendType(); backend == "google" {
		instanceConfig, _ := kci.GetConfigResource(ctx, dbin.Spec.Google.ConfigmapName.ToKubernetesType())
		checksums["config"] = kci.GenerateChecksum(instanceConfig)
	}

	dbin.Status.Checksums = checksums
}

func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func SliceContainsSubString(slice []string, s string) bool {
	for _, item := range slice {
		if strings.Contains(item, s) {
			return true
		}
	}
	return false
}

// InCrdList returns true if monitoring is enabled in DbInstance spec.
func InCrdList(crds crdv1.CustomResourceDefinitionList, api string) bool {
	for _, crd := range crds.Items {
		if crd.Name == api {
			return true
		}
	}
	return false
}
