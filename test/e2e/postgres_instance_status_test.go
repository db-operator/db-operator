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

	"github.com/db-operator/db-operator/v2/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func getInstanceManifest(engine *utils.EngineConfig) (manifest string) {
	manifestEngine := engine.Name
	if !engine.SslConfig.Enabled {
		manifest = fmt.Sprintf("%s_no_ssl.yaml", manifestEngine)
	} else {
		if engine.SslConfig.SkipVerify {
			manifest = fmt.Sprintf("%s_ssl.yaml", manifestEngine)
		} else {
			manifest = fmt.Sprintf("%s_ssl_verify.yaml", manifestEngine)
		}
	}
	return
}

// Instances are pretty hard to test because they are more or less
// just a meta resource that will be used by Databases and DbUsers
// later.
var _ = Describe("Instance with config from", Ordered, func() {
	BeforeAll(func() {
		//for _, engine := range TestData.Engines {
		//By("creating resources")
		//cmd := exec.Command("kubectl", "create", "-f", fmt.Sprintf("./test/manifests/instances/%s", manifest))
		//_, err := utils.Run(cmd)
		//Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")
		// Create Namespace
		createNsCmd := exec.Command("kubectl", "create", "-f", "./test/manifests/namespace.yaml")
		_, err := utils.Run(createNsCmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")
		//}
	})
	AfterAll(func() {
		//By("deleting resources")
		//manifestEngine := engine.Name
		//if !engine.SslConfig.Enabled {
		//	manifest = fmt.Sprintf("%s_no_ssl.yaml", manifestEngine)
		//} else {
		//	if engine.SslConfig.SkipVerify {
		//		manifest = fmt.Sprintf("%s_ssl.yaml", manifestEngine)
		//	} else {
		//		manifest = fmt.Sprintf("%s_ssl_verify.yaml", manifestEngine)
		//	}
		//}
		//cmd := exec.Command("kubectl", "delete", "-f", fmt.Sprintf("./test/manifests/instances/%s", manifest))
		//_, err := utils.Run(cmd)
		//Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")
		deleteNsCmd := exec.Command("kubectl", "delete", "-f", "./test/manifests/namespace.yaml")
		_, err := utils.Run(deleteNsCmd)
		Expect(err).NotTo(HaveOccurred(), "Failed todelete namespace")
	})
	Context("Testing DbInstances", func() {
		It("DbInstance should become ready", func() {
			//By("Checking DbInstance status")
			//verifyDbInstance := func(g Gomega) {
			//	cmd := exec.Command("kubectl", "get", "dbinstance", "postgres", "-o", "yaml")
			//	instanceRaw, err := utils.Run(cmd)
			//	Expect(err).NotTo(HaveOccurred(), "Failed to get instance")
			//	instance := &kindarocksv1beta1.DbInstance{}
			//	err = yaml.Unmarshal([]byte(instanceRaw), instance)
			//	Expect(err).NotTo(HaveOccurred(), "Failed to unmarschal dbinstance")
			//	g.Expect(instance.Status.Status).To(Equal(true), "Status is not true")
			//	g.Expect(instance.Status.Phase).To(Equal("Running"), "Phase is not Running")
			//}
			//Eventually(verifyDbInstance).WithPolling(10 * time.Second).WithTimeout(60 * time.Second).Should(Succeed())
		})
		It("DbInstance should contain correct info", func() {
			//By("Checking DbInstance info")
			//verifyDbInstance := func(g Gomega) {
			//	cmd := exec.Command("kubectl", "get", "dbinstance", "postgres", "-o", "yaml")
			//	instanceRaw, err := utils.Run(cmd)
			//	Expect(err).NotTo(HaveOccurred(), "Failed to get instance")
			//	instance := &kindarocksv1beta1.DbInstance{}
			//	err = yaml.Unmarshal([]byte(instanceRaw), instance)
			//	Expect(err).NotTo(HaveOccurred(), "Failed to unmarschal dbinstance")
			//	g.Expect(instance.Status.Info["DB_CONN"]).To(Equal("postgres-instance.databases.svc.cluster.local"), "Info DB_CONN is not right")
			//	g.Expect(instance.Status.Info["DB_PORT"]).To(Equal("5432"), "Info DB_PORT is not right")
			//	g.Expect(instance.Status.Info["DB_PUBLIC_IP"]).To(Equal("1.2.3.4"), "Info DB_PUBLIC_IP is not right")
			//}
			//Eventually(verifyDbInstance).WithPolling(10 * time.Second).WithTimeout(60 * time.Second).Should(Succeed())
		})
	})
	for _, engine := range TestData.Engines {
		fmt.Println(engine.Name)
	}
})
