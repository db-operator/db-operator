package tplrender

import (
	"bytes"
	"fmt"
	"os"
	"text/template"

	"github.com/db-operator/db-operator/v2/pkg/consts"
)

type TplData struct {
	Engine          string
	Namespace       string
	ImageRegistry   string
	ImageRepository string
	ImageTag        string
	ImagePullPolicy string
	DatabaseName    string
}

func Render(tpl string, data *TplData) ([]byte, error) {
	t, err := template.New("manifest-template").Parse(tpl)
	if err != nil {
		return nil, err
	}

	var tplRes bytes.Buffer
	if err := t.Execute(&tplRes, data); err != nil {
		return nil, err
	}

	return tplRes.Bytes(), nil
}

func ReadFile(dir string, name string) (string, error) {
	if len(dir) == 0 {
		dir = consts.DEFAULT_TEMPLATES_DIR
	}

	filePath := fmt.Sprintf("%s/%s", dir, name)

	data, err := os.ReadFile(filePath)
	if err != nil {
		// If template is not found in the custom directory, try the default one
		if os.IsNotExist(err) && dir == consts.DEFAULT_TEMPLATES_DIR {
			defaultPath := fmt.Sprintf("%s/%s", consts.DEFAULT_TEMPLATES_DIR, name)
			data, err = os.ReadFile(defaultPath)
			if err != nil {
				return "", err
			}
		}
		return "", err
	}

	return string(data), err
}
