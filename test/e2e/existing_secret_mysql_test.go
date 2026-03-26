package e2e

import (
	"errors"
	"fmt"
	"os/exec"
	"time"

	"db-operator-test/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Existing Secret", Ordered, func() {
	const namespace string = "test-existing-secret-mysql"
	BeforeAll(func() {
		Expect(utils.CreateManifest("../manifests/namespace.yaml")).NotTo(HaveOccurred())
		cmd := exec.Command("kubectl", "create", "namespace", namespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred())
	})
	It("Create DbInstance using the config data", func() {
		Expect(utils.CreateManifestInNs("../manifests/admin_creds/mysql.yaml", namespace)).NotTo(HaveOccurred())
		Expect(utils.CreateManifestInNs("../manifests/instances/mysql_no_ssl.yaml", namespace)).NotTo(HaveOccurred())
	})
	It("DbInstance should become ready", func() {
		dbInstanceReady := func(g Gomega) error {
			cmd := exec.Command("kubectl", "get", "dbinstance", "mysql", "--output", "jsonpath='{.status.status}'")
			status, err := utils.Run(cmd)
			Expect(err).ToNot(HaveOccurred())
			if status != "'true'" {
				return errors.New("not ready")
			}
			return nil
		}
		Eventually(dbInstanceReady).WithPolling(10 * time.Second).WithTimeout(60 * time.Second).Should(Succeed())
	})
	It("Create a secret with credentials", func() {
		Expect(utils.CreateManifestInNs("../manifests/secrets/mysql-creds.yaml", namespace)).NotTo(HaveOccurred())
	})
	It("Create a Database", func() {
		Expect(utils.CreateManifestInNs("../manifests/databases/mysql.yaml", namespace)).NotTo(HaveOccurred())
	})
	It("Database should become ready", func() {
		databaseReady := func(g Gomega) error {
			cmd := exec.Command("kubectl", "get", "-n", namespace, "database", "mysql", "--output", "jsonpath='{.status.status}'")
			status, err := utils.Run(cmd)
			Expect(err).ToNot(HaveOccurred())
			if status != "'true'" {
				return errors.New("not ready")
			}
			return nil
		}
		Eventually(databaseReady).WithPolling(10 * time.Second).WithTimeout(120 * time.Second).Should(Succeed())
	})
	It("Database Secret should be created", func() {
		dbInstanceReady := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "-n", namespace, "secret", "mysql-creds")
			_, err := utils.Run(cmd)
			Expect(err).ToNot(HaveOccurred())
		}
		Eventually(dbInstanceReady).WithPolling(10 * time.Second).WithTimeout(60 * time.Second).Should(Succeed())
	})
	It("Database ConfigMap should be created", func() {
		dbInstanceReady := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "-n", namespace, "secret", "mysql-creds")
			_, err := utils.Run(cmd)
			Expect(err).ToNot(HaveOccurred())
		}
		Eventually(dbInstanceReady).WithPolling(10 * time.Second).WithTimeout(60 * time.Second).Should(Succeed())
	})
	It("Create ConfigMaps with testing scripts", func() {
		kusomizeCmd := exec.Command("kustomize", "build", "../manifests/scripts/mysql/", "-o", "/tmp/db-operator-test-cm.yaml")
		_, err := utils.Run(kusomizeCmd)
		Expect(err).ToNot(HaveOccurred())
		Expect(utils.CreateManifestInNs("/tmp/db-operator-test-cm.yaml", namespace)).NotTo(HaveOccurred())
	})
	It("Create the tester pod", func() {
		Expect(utils.CreateManifestInNs("../manifests/pods/mysql_tester.yaml", namespace)).NotTo(HaveOccurred())
	})
	It("Pod should be finished", func() {
		podSuccedded := func(g Gomega) error {
			cmd := exec.Command("kubectl", "get", namespace, "pod", "tester", "-o", "jsonpath='{.status.phase}'")
			status, err := utils.Run(cmd)
			Expect(err).ToNot(HaveOccurred())
			Expect(status).ToNot(Equal("'Failed'"))
			if status != "'Succeeded'" {
				logsCmd := exec.Command("kubectl", "logs", "-n", namespace, "tester")
				out, _ := utils.Run(logsCmd)
				fmt.Println(out)
				return errors.New("not yet succeeded")
			}
			return nil
		}
		Eventually(podSuccedded).WithPolling(10 * time.Second).WithTimeout(60 * time.Second).Should(Succeed())
	})
	AfterAll(func() {
		Expect(utils.DeleteManifestInNs("../manifests/databases/mysql.yaml", namespace)).NotTo(HaveOccurred())
		Expect(utils.DeleteManifest("../manifests/instances/mysql_no_ssl.yaml")).NotTo(HaveOccurred())
		cmd := exec.Command("kubectl", "delete", "namespace", namespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred())
	})
	// Create a Pod to connect to the database
})
