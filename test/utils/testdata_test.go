package utils_test

import (
	"db-operator-test/utils"
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"
)

func TestUnitTestData1(t *testing.T) {
	yamldata := `---
engines: 
  - name: postgres
    sslConfig:
      enabled: true
      skipVerify: false
`

	data := &utils.TestData{}
	assert.NoError(t, yaml.Unmarshal([]byte(yamldata), data))
	assert.Equal(t, data.Engines[0].Name, "postgres")
	assert.Equal(t, data.Engines[0].SslConfig.Enabled, true)
	assert.Equal(t, data.Engines[0].SslConfig.SkipVerify, false)
}

func TestUnitTestData2(t *testing.T) {
	yamldata := `---
engines: 
  - name: mysql
    sslConfig:
      enabled: false
      skipVerify: true
`
	data := &utils.TestData{}
	assert.NoError(t, yaml.Unmarshal([]byte(yamldata), data))
	assert.Equal(t, data.Engines[0].Name, "mysql")
	assert.Equal(t, data.Engines[0].SslConfig.Enabled, false)
	assert.Equal(t, data.Engines[0].SslConfig.SkipVerify, true)
}

func TestUnitTestData3(t *testing.T) {
	yamldata := `---
engines: 
  - name: postgres
    sslConfig:
      enabled: true
      skipVerify: false
  - name: mysql
    sslConfig:
      enabled: false
      skipVerify: true
`

	data := &utils.TestData{}
	assert.NoError(t, yaml.Unmarshal([]byte(yamldata), data))
	assert.Equal(t, data.Engines[0].Name, "postgres")
	assert.Equal(t, data.Engines[0].SslConfig.Enabled, true)
	assert.Equal(t, data.Engines[0].SslConfig.SkipVerify, false)
	assert.Equal(t, data.Engines[1].Name, "mysql")
	assert.Equal(t, data.Engines[1].SslConfig.Enabled, false)
	assert.Equal(t, data.Engines[1].SslConfig.SkipVerify, true)
}
