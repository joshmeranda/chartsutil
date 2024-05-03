package rebase_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/joshmeranda/chartsutil/pkg/rebase"
	"github.com/rancher/charts-build-scripts/pkg/charts"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	chartspath "github.com/rancher/charts-build-scripts/pkg/path"
)

const (
	BadHelmTemplateContent = `{{ if .global.something.bad }}`
)

func setupVerify(t *testing.T, pkgName string) (*charts.Package, *git.Worktree, billy.Filesystem) {
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
	pkgFs, err := rootFs.Chroot(filepath.Join(chartspath.RepositoryPackagesDir, pkgName))
	if err != nil {
		t.Fatalf("failed to chroot to package dir: %v", err)
	}

	pkg, err := charts.GetPackage(rootFs, pkgName)
	if err != nil {
		t.Fatalf("failed to get package: %v", err)
	}

	repo, err := git.PlainOpen(chartsDir)
	if err != nil {
		t.Fatalf("failed to open git repo: %v", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}

	opts := git.CheckoutOptions{
		Hash: plumbing.NewHash(RebaseExampleCommit),
	}

	if err := wt.Checkout(&opts); err != nil {
		t.Fatalf("failed to checkout commit: %v", err)
	}

	if err := pkg.Prepare(); err != nil {
		t.Fatalf("failed to prepare package: %v", err)
	}

	return pkg, wt, pkgFs
}

func corruptHelm(chartPath string) error {
	templatePath := filepath.Join(chartPath, "templates", "bad-helm-template.yaml")
	if err := os.WriteFile(templatePath, []byte(BadHelmTemplateContent), 0644); err != nil {
		return err
	}

	return nil
}

func TestValidateHelmLint(t *testing.T) {
	pkg, wt, pkgFs := setupVerify(t, "chartsutil-example-archive")

	if err := rebase.ValidateHelmLint(pkg, wt, pkgFs); err != nil {
		t.Fatalf("failed to verify helm template: %v", err)
	}

	if err := corruptHelm(filepath.Join(pkgFs.Root(), pkg.WorkingDir)); err != nil {
		t.Fatalf("failed to corrupt helm template: %v", err)
	}

	if err := rebase.ValidateHelmLint(pkg, wt, pkgFs); err == nil {
		if !errors.Is(err, rebase.ValidateError{}) {
			t.Fatalf("expected ValidateError, got %T", err)
		}

		t.Fatalf("expected error, got nil")
	}
}

func TestValidatePatternNotFound(t *testing.T) {
	pkg, wt, pkgFs := setupVerify(t, "chartsutil-example-archive")

	verifyFunc := rebase.ValidatePatternNotFoundFactory(".something.bad")

	if err := verifyFunc(pkg, wt, pkgFs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := corruptHelm(filepath.Join(pkgFs.Root(), pkg.WorkingDir)); err != nil {
		t.Fatalf("failed to corrupt helm template: %v", err)
	}

	if err := verifyFunc(pkg, wt, pkgFs); err == nil {
		if !errors.Is(err, rebase.ValidateError{}) {
			t.Fatalf("expected ValidateError, got %T", err)
		}

		t.Fatalf("expected error, got nil")
	}
}

func TestValidateWorktree(t *testing.T) {
	pkg, wt, pkgFs := setupVerify(t, "chartsutil-example-archive")
	// chartPath := filepath.Join(pkgFs.Root(), pkg.WorkingDir)

	if _, err := wt.Add(chartspath.RepositoryPackagesDir); err != nil {
		t.Fatalf("failed to stage changes: %v", err)
	}

	if _, err := wt.Commit("save charts", &git.CommitOptions{}); err != nil {
		t.Fatalf("failed to commit charts: %v", err)
	}

	if err := rebase.ValidateWorktree(pkg, wt, pkgFs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := corruptHelm(filepath.Join(pkgFs.Root(), pkg.WorkingDir)); err != nil {
		t.Fatalf("failed to corrupt helm template: %v", err)
	}

	// fails on unstaged change
	if err := rebase.ValidateWorktree(pkg, wt, pkgFs); err == nil {
		if !errors.Is(err, rebase.ValidateError{}) {
			t.Fatalf("expected ValidateError, got %T", err)
		}

		t.Fatalf("expected error, got nil")
	}

	if _, err := wt.Add(filepath.Join(chartspath.RepositoryPackagesDir, pkg.Name, pkg.WorkingDir, "templates", "bad-helm-template.yaml")); err != nil {
		t.Fatalf("failed to stage change: %v", err)
	}

	// succeeds on staged change
	if err := rebase.ValidateWorktree(pkg, wt, pkgFs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := os.WriteFile(filepath.Join(pkgFs.Root(), "bad-file"), []byte("I should not exist"), 0644); err != nil {
		t.Fatalf("failed to create bad file: %v", err)
	}

	// fails on untracked file
	if err := rebase.ValidateWorktree(pkg, wt, pkgFs); err == nil {
		if !errors.Is(err, rebase.ValidateError{}) {
			t.Fatalf("expected ValidateError, got %T", err)
		}

		t.Fatalf("expected error, got nil")
	}

	// fails on staged file in disallowed location
	if _, err := wt.Add(filepath.Join(chartspath.RepositoryPackagesDir, pkg.Name, "bad-file")); err != nil {
		t.Fatalf("failed to stage change: %v", err)
	}

	if err := rebase.ValidateWorktree(pkg, wt, pkgFs); err == nil {
		if !errors.Is(err, rebase.ValidateError{}) {
			t.Fatalf("expected ValidateError, got %T", err)
		}

		t.Fatalf("expected error, got nil")
	}
}
