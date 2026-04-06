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

var _ = Describe("Basic Workflow", Ordered, func() {
	BeforeAll(func() {
		Expect(utils.CreateManifest("../manifests/namespace.yaml")).NotTo(HaveOccurred())
	})
	It("Create DbInstance using the config data", func() {
		Expect(utils.CreateManifest("../manifests/admin_creds/postgres.yaml")).NotTo(HaveOccurred())
		Expect(utils.CreateManifest("../manifests/instances/postgres_no_ssl.yaml")).NotTo(HaveOccurred())
	})
	It("DbInstance should become ready", func() {
		dbInstanceReady := func(g Gomega) error {
			cmd := exec.Command("kubectl", "get", "dbinstance", "postgres", "--output", "jsonpath='{.status.status}'")
			status, err := utils.Run(cmd)
			Expect(err).ToNot(HaveOccurred())
			if status != "'true'" {
				return errors.New("not ready")
			}
			return nil
		}
		Eventually(dbInstanceReady).WithPolling(10 * time.Second).WithTimeout(60 * time.Second).Should(Succeed())
	})
	It("Create a Database", func() {
		Expect(utils.CreateManifest("../manifests/databases/postgres.yaml")).NotTo(HaveOccurred())
	})
	It("Database should become ready", func() {
		databaseReady := func(g Gomega) error {
			cmd := exec.Command("kubectl", "get", "-n", "db-operator-e2e", "database", "postgres", "--output", "jsonpath='{.status.status}'")
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
			cmd := exec.Command("kubectl", "get", "-n", "db-operator-e2e", "secret", "postgres-creds")
			_, err := utils.Run(cmd)
			Expect(err).ToNot(HaveOccurred())
		}
		Eventually(dbInstanceReady).WithPolling(10 * time.Second).WithTimeout(60 * time.Second).Should(Succeed())
	})
	It("Database ConfigMap should be created", func() {
		dbInstanceReady := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "-n", "db-operator-e2e", "secret", "postgres-creds")
			_, err := utils.Run(cmd)
			Expect(err).ToNot(HaveOccurred())
		}
		Eventually(dbInstanceReady).WithPolling(10 * time.Second).WithTimeout(60 * time.Second).Should(Succeed())
	})
	It("Create ConfigMaps with testing scripts", func() {
		kusomizeCmd := exec.Command("kustomize", "build", "../manifests/scripts/postgres/", "-o", "/tmp/db-operator-test-cm.yaml")
		_, err := utils.Run(kusomizeCmd)
		Expect(err).ToNot(HaveOccurred())
		Expect(utils.CreateManifest("/tmp/db-operator-test-cm.yaml")).NotTo(HaveOccurred())
	})
	It("Create the tester pod", func() {
		Expect(utils.CreateManifest("../manifests/pods/postgres_tester.yaml")).NotTo(HaveOccurred())
	})
	It("Pod should be finished", func() {
		podSuccedded := func(g Gomega) error {
			cmd := exec.Command("kubectl", "get", "-n", "db-operator-e2e", "pod", "tester", "-o", "jsonpath='{.status.phase}'")
			status, err := utils.Run(cmd)
			Expect(err).ToNot(HaveOccurred())
			Expect(status).ToNot(Equal("'Failed'"))
			if status != "'Succeeded'" {
				logsCmd := exec.Command("kubectl", "logs", "-n", "db-operator-e2e", "tester")
				out, _ := utils.Run(logsCmd)
				fmt.Println(out)
				return errors.New("not yet succeeded")
			}
			return nil
		}
		Eventually(podSuccedded).WithPolling(10 * time.Second).WithTimeout(60 * time.Second).Should(Succeed())
	})
	AfterAll(func() {
		Expect(utils.DeleteManifest("../manifests/databases/postgres.yaml")).NotTo(HaveOccurred())
		Expect(utils.DeleteManifest("../manifests/namespace.yaml")).NotTo(HaveOccurred())
		Expect(utils.DeleteManifest("../manifests/instances/postgres_no_ssl.yaml")).NotTo(HaveOccurred())
	})
	// Create a Pod to connect to the database
})
