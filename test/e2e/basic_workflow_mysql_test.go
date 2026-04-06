package e2e

import (
	"errors"
	"os/exec"
	"time"

	"db-operator-test/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const mysqlTest1NS = "mysql-e2e-1"

var _ = Describe("Basic Workflow", Ordered, func() {
	BeforeAll(func() {
		cmd := exec.Command("helm", "install", "--namespace", mysqlTest1NS, "--wait", "watcher", "test-mysql", "../charts/db-operator-mysql-test/")
		_, err := utils.Run(cmd)
		Expect(err).ToNot(HaveOccurred())
	})
	It("DbInstance should become ready", func() {
		cmd := exec.Command("sh", "-c", "helm template ../charts/db-operator-mysql-test/ | yq 'select(.kind == \"DbInstance\") | .metadata.name'")
		dbinstance, err := utils.Run(cmd)
		Expect(err).ToNot(HaveOccurred())
		dbInstanceReady := func(g Gomega) error {
			cmd := exec.Command("kubectl", "get", "dbinstance", dbinstance, "--output", "jsonpath='{.status.status}'")
			status, err := utils.Run(cmd)
			Expect(err).ToNot(HaveOccurred())
			if status != "'true'" {
				return errors.New("not ready")
			}
			return nil
		}
		Eventually(dbInstanceReady).WithPolling(10 * time.Second).WithTimeout(60 * time.Second).Should(Succeed())
	})
	It("Database should become ready", func() {
		cmd := exec.Command("sh", "-c", "helm template ../charts/db-operator-mysql-test/ | yq 'select(.kind == \"Database\") | .metadata.name'")
		database, err := utils.Run(cmd)
		Expect(err).ToNot(HaveOccurred())
		databaseReady := func(g Gomega) error {
			cmd := exec.Command("kubectl", "get", "-n", mysqlTest1NS, "database", database, "--output", "jsonpath='{.status.status}'")
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
		cmd := exec.Command("sh", "-c", "helm template ../charts/db-operator-mysql-test/ | yq 'select(.kind == \"Database\") | .spec.secretName'")
		secret, err := utils.Run(cmd)
		Expect(err).ToNot(HaveOccurred())
		dbInstanceReady := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "-n", mysqlTest1NS, "secret", secret)
			_, err := utils.Run(cmd)
			Expect(err).ToNot(HaveOccurred())
		}
		Eventually(dbInstanceReady).WithPolling(10 * time.Second).WithTimeout(60 * time.Second).Should(Succeed())
	})
	It("Database ConfigMap should be created", func() {
		cmd := exec.Command("sh", "-c", "helm template ../charts/db-operator-mysql-test/ | yq 'select(.kind == \"Database\") | .spec.secretName'")
		secret, err := utils.Run(cmd)
		Expect(err).ToNot(HaveOccurred())
		dbInstanceReady := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "-n", mysqlTest1NS, "secret", secret)
			_, err := utils.Run(cmd)
			Expect(err).ToNot(HaveOccurred())
		}
		Eventually(dbInstanceReady).WithPolling(10 * time.Second).WithTimeout(60 * time.Second).Should(Succeed())
	})
	It("Create the tester pod", func() {
		cmd := exec.Command("helm", "test", "test-mysql")
		_, err := utils.Run(cmd)
		Expect(err).ToNot(HaveOccurred())
	})
	AfterAll(func() {
		cmd := exec.Command("helm", "uninstall", "test-mysql")
		_, err := utils.Run(cmd)
		Expect(err).ToNot(HaveOccurred())
	})
})
