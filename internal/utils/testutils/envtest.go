package testutils

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"golang.org/x/mod/semver"
)

func LatestK8sAssetsDir(baseDir string) (string, error) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return "", err
	}

	suffix := fmt.Sprintf("-%s-%s", runtime.GOOS, runtime.GOARCH)

	var versions []string
	versionToDir := make(map[string]string)

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		name := e.Name()
		if !strings.HasSuffix(name, suffix) {
			continue
		}

		ver := strings.TrimSuffix(name, suffix)
		semVer := "v" + ver
		if !semver.IsValid(semVer) {
			continue
		}

		versions = append(versions, semVer)
		versionToDir[semVer] = filepath.Join(baseDir, name)
	}

	if len(versions) == 0 {
		return "", fmt.Errorf("no Kubernetes assets found in %s", baseDir)
	}

	sort.Slice(versions, func(i, j int) bool {
		return semver.Compare(versions[i], versions[j]) > 0
	})

	return versionToDir[versions[0]], nil
}
