//go:build e2e
// +build e2e

/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/db-operator/db-operator/v2/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

var TestData *utils.TestData

func init() {
	cmGetCmd := exec.Command("kubectl", "get", "--namespace", "default", "configmap", "test-data", "--output", "yaml")
	cmRaw, err := utils.Run(cmGetCmd)
	if err != nil {
		panic(err)
	}
	cm := &corev1.ConfigMap{}
	if err := yaml.Unmarshal([]byte(cmRaw), cm); err != nil {
		panic(err)
	}
	if err := yaml.Unmarshal([]byte(cm.Data["test_data.yaml"]), TestData); err != nil {
		panic(err)
	}
}

// TestE2E runs the e2e test suite to validate the solution in an isolated environment.
// The default setup requires Kind and CertManager.
// A
// To skip CertManager installation, set: CERT_MANAGER_INSTALL_SKIP=true
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting operator e2e test suite\n")
	RunSpecs(t, "e2e suite")
}

var _ = BeforeSuite(func() {
	// Check CRDs
	crdCmd := exec.Command("kubectl", "get", "crds")
	output, err := utils.Run(crdCmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to get CRDs from the cluster")
	Expect(output).To(ContainSubstring("databases.kinda.rocks"))
	Expect(output).To(ContainSubstring("dbinstances.kinda.rocks"))
	Expect(output).To(ContainSubstring("dbusers.kinda.rocks"))
})
