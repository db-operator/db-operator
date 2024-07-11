package v1beta2_test

import (
	"testing"

	"github.com/db-operator/db-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
)

func TestUnitTemplatesValidator(t *testing.T) {
	validTemplates := v1beta1.Templates{
		{Name: "TEMPLATE_1", Template: "{{ .Protocol }} {{ .Hostname }} {{ .Port }} {{ .Username }} {{ .Password }} {{ .Database }}"},
		{Name: "TEMPLATE_2", Template: "{{.Protocol }}"},
		{Name: "TEMPLATE_3", Template: "{{.Protocol }}"},
		{Name: "TEMPLATE_4", Template: "{{.Protocol}}"},
		{Name: "TEMPLATE_5", Template: "jdbc:{{ .Protocol }}://{{ .Username }}:{{ .Password }}@{{ .Hostname }}:{{ .Port }}/{{ .Database }}"},
		{Name: "TEMPLATE_6", Template: "{{ .Secret \"CHECK\" }}"},
		{Name: "TEMPLATE_7", Template: "{{ .ConfigMap \"CHECK\" }}"},
		{Name: "TEMPLATE_8", Template: "{{ .Query \"CHECK\" }}"},
		{Name: "TEMPLATE_9", Template: "{{ if eq 1 1 }} It's true {{ else }} It's false {{ end }}"},
	}

	err := v1beta1.ValidateTemplates(validTemplates, true)
	assert.NoErrorf(t, err, "expected no error %v", err)

	invalidTemplates := v1beta1.Templates{
		{Name: "TEMPLATE_1", Template: "{{ .InvalidField }}"},
		{Name: "TEMPLATE_2", Template: "{{ .Secret invalid }}"},
		{Name: "TEMPLATE_3", Template: "{{ .Secret }}"},
	}

	err = v1beta1.ValidateTemplates(invalidTemplates, true)
	assert.Errorf(t, err, "should get error %v", err)

	cmTemplates := v1beta1.Templates{
		{Name: "TEMPLATE_1", Template: "configmap template", Secret: false},
	}

	err = v1beta1.ValidateTemplates(cmTemplates, true)
	assert.NoErrorf(t, err, "expected no error: %v", err)
	err = v1beta1.ValidateTemplates(cmTemplates, false)
	assert.ErrorContains(t, err, "ConfigMap templating is not allowed for that kind. Please set .secret to true")
}
