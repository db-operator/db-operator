package e2e

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

//var testData *utils.TestData = &utils.TestData{}

func TestE2e(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2e Suite")
}

// Not needed atm
//var _ = BeforeSuite(func() {
//	getCmCmd := exec.Command("kubectl", "--namespace", "default", "get", "configmap", "test-data", "--output", "yaml")
//	cmRaw, err := utils.Run(getCmCmd)
//	Expect(err).ToNot(HaveOccurred())
//
//	cm := &corev1.ConfigMap{}
//	err = yaml.Unmarshal([]byte(cmRaw), cm)
//	Expect(err).ToNot(HaveOccurred())
//
//	err = yaml.Unmarshal([]byte(cm.Data["test_data.yaml"]), testData)
//	Expect(err).ToNot(HaveOccurred())
//})
