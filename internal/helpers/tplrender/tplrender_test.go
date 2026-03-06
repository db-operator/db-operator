package tplrender_test

import (
	"testing"

	"github.com/db-operator/db-operator/v2/internal/helpers/tplrender"
	"github.com/stretchr/testify/assert"
)

func TestUnitTplRenderEngine(t *testing.T) {
	data := &tplrender.TplData{
		Engine: "mysql",
	}
	res, err := tplrender.Render("{{ .Engine }}", data)

	assert.NoError(t, err)
	assert.Equal(t, string(res), data.Engine)
}

func TestUnitTplRenderImageRegistry(t *testing.T) {
	data := &tplrender.TplData{
		ImageRegistry: "ghcr.io/db-operator",
	}
	res, err := tplrender.Render("{{ .ImageRegistry }}", data)

	assert.NoError(t, err)
	assert.Equal(t, string(res), data.ImageRegistry)
}

func TestUnitTplRenderImageRepository(t *testing.T) {
	data := &tplrender.TplData{
		ImageRepository: "db-operator",
	}
	res, err := tplrender.Render("{{ .ImageRepository }}", data)

	assert.NoError(t, err)
	assert.Equal(t, string(res), data.ImageRepository)
}

func TestUnitTplRenderImageTag(t *testing.T) {
	data := &tplrender.TplData{
		ImageRegistry: "latest",
	}
	res, err := tplrender.Render("{{ .ImageTag }}", data)

	assert.NoError(t, err)
	assert.Equal(t, string(res), data.ImageTag)
}
