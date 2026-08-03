package testutils_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/db-operator/db-operator/v2/internal/utils/testutils"
)

func TestUnitLatestK8sAssetsDir_SelectsHighestVersion(t *testing.T) {
	base := t.TempDir()

	suffix := fmt.Sprintf("-%s-%s", runtime.GOOS, runtime.GOARCH)

	for _, dir := range []string{
		"1.34.1" + suffix,
		"1.36.2" + suffix,
		"1.35.5" + suffix,
	} {
		if err := os.Mkdir(filepath.Join(base, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	got, err := testutils.LatestK8sAssetsDir(base)
	if err != nil {
		t.Fatalf("LatestK8sAssetsDir() error = %v", err)
	}

	want := filepath.Join(base, "1.36.2"+suffix)
	if got != want {
		t.Errorf("LatestK8sAssetsDir() = %q, want %q", got, want)
	}
}

func TestUnitLatestK8sAssetsDir_IgnoresInvalidEntries(t *testing.T) {
	base := t.TempDir()

	suffix := fmt.Sprintf("-%s-%s", runtime.GOOS, runtime.GOARCH)

	// Valid directory.
	if err := os.Mkdir(filepath.Join(base, "1.35.0"+suffix), 0o755); err != nil {
		t.Fatal(err)
	}

	// Invalid version.
	if err := os.Mkdir(filepath.Join(base, "latest"+suffix), 0o755); err != nil {
		t.Fatal(err)
	}

	// Regular file.
	if err := os.WriteFile(filepath.Join(base, "1.99.0"+suffix), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := testutils.LatestK8sAssetsDir(base)
	if err != nil {
		t.Fatalf("LatestK8sAssetsDir() error = %v", err)
	}

	want := filepath.Join(base, "1.35.0"+suffix)
	if got != want {
		t.Errorf("LatestK8sAssetsDir() = %q, want %q", got, want)
	}
}

func TestUnitLatestK8sAssetsDir_IgnoresOtherArchitectures(t *testing.T) {
	base := t.TempDir()

	current := fmt.Sprintf("-%s-%s", runtime.GOOS, runtime.GOARCH)

	if err := os.Mkdir(filepath.Join(base, "1.35.0"+current), 0o755); err != nil {
		t.Fatal(err)
	}

	otherArch := "amd64"
	if runtime.GOARCH == "amd64" {
		otherArch = "arm64"
	}

	if err := os.Mkdir(filepath.Join(base,
		fmt.Sprintf("1.99.0-%s-%s", runtime.GOOS, otherArch)), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := testutils.LatestK8sAssetsDir(base)
	if err != nil {
		t.Fatalf("LatestK8sAssetsDir() error = %v", err)
	}

	want := filepath.Join(base, "1.35.0"+current)
	if got != want {
		t.Errorf("LatestK8sAssetsDir() = %q, want %q", got, want)
	}
}

func TestUnitLatestK8sAssetsDir_NoMatchingDirectories(t *testing.T) {
	base := t.TempDir()

	_, err := testutils.LatestK8sAssetsDir(base)
	if err == nil {
		t.Fatal("LatestK8sAssetsDir() expected error, got nil")
	}
}
