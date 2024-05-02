package rebase_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/joshmeranda/chartsutil/pkg/rebase"
	"github.com/rancher/charts-build-scripts/pkg/charts"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	chartspath "github.com/rancher/charts-build-scripts/pkg/path"
)

const (
	BadHelmTemplateContent = `{{ if .global.something.bad }}`
)

func setupVerify(t *testing.T, pkgName string) string {
	chartsDir := fmt.Sprintf("%s-charts", t.Name())
	t.Cleanup(func() {
		if err := os.RemoveAll(chartsDir); err != nil {
			t.Fatalf("failed to remove temp dir: %v", err)
		}
	})

	_, err := cloneChartsTo(chartsDir)
	if err != nil {
		t.Fatalf("failed to clone charts: %v", err)
	}

	rootFs := filesystem.GetFilesystem(chartsDir)

	pkg, err := charts.GetPackage(rootFs, pkgName)
	if err != nil {
		t.Fatalf("failed to get package: %v", err)
	}

	if err := pkg.Prepare(); err != nil {
		t.Fatalf("failed to prepare package: %v", err)
	}

	return filepath.Join(chartsDir, chartspath.RepositoryPackagesDir, pkgName, pkg.WorkingDir)
}

func corruptHelm(chartPath string) error {
	templatePath := filepath.Join(chartPath, "templates", "bad-helm-template.yaml")
	if err := os.WriteFile(templatePath, []byte(BadHelmTemplateContent), 0644); err != nil {
		return err
	}

	return nil
}

func TestVerifyHelmLint(t *testing.T) {
	chartPath := setupVerify(t, "chartsutil-example-archive")

	if err := rebase.VerifyHelmLint(chartPath); err != nil {
		t.Fatalf("failed to verify helm template: %v", err)
	}

	if err := corruptHelm(chartPath); err != nil {
		t.Fatalf("failed to corrupt helm template: %v", err)
	}

	if err := rebase.VerifyHelmLint(chartPath); err == nil {
		if !errors.Is(err, rebase.ValidateError{}) {
			t.Fatalf("expected ValidateError, got %T", err)
		}

		t.Fatalf("expected error, got nil")
	}
}

func TestVerifyPatternNotFound(t *testing.T) {
	chartPath := setupVerify(t, "chartsutil-example-archive")

	verifyFunc := rebase.VerifyPatternNotFoundFactory(".something.bad")

	if err := verifyFunc(chartPath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := corruptHelm(chartPath); err != nil {
		t.Fatalf("failed to corrupt helm template: %v", err)
	}

	if err := verifyFunc("something.bad"); err == nil {
		if !errors.Is(err, rebase.ValidateError{}) {
			t.Fatalf("expected ValidateError, got %T", err)
		}

		t.Fatalf("expected error, got nil")
	}
}
