package rebase_test

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/joshmeranda/chartsutil/pkg/iter"
	"github.com/joshmeranda/chartsutil/pkg/rebase"
	"github.com/joshmeranda/chartsutil/pkg/resolve"
	cp "github.com/otiai10/copy"
	"github.com/rancher/charts-build-scripts/pkg/charts"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	chartspath "github.com/rancher/charts-build-scripts/pkg/path"
)

const (
	RebaseExampleUrl = "https://github.com/joshmeranda/chartsutil-example"

	RebaseExampleUpstreamUrl = "https://github.com/joshmeranda/chartsutil-example-upstream"

	CacheDir = ".test-cache"

	RebaseExampleCommit = "101ce93f3a15c8122e3bb582e07b9f6648fbafc3"
)

var logger *slog.Logger

func init() {
	if err := os.MkdirAll(CacheDir, 0755); err != nil {
		panic(fmt.Sprintf("failed to create cache dir: %v", err))
	}

	// logger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))
}

func cloneChartsTo(path string) (*git.Repository, error) {
	cloneDir := filepath.Join(CacheDir, "chartsutil-example")

	if _, err := os.Stat(cloneDir); errors.Is(err, os.ErrNotExist) {
		cloneOpts := &git.CloneOptions{
			URL: RebaseExampleUrl,
		}

		if _, err := git.PlainClone(cloneDir, false, cloneOpts); err != nil {
			return nil, err
		}
	}

	if err := cp.Copy(cloneDir, path); err != nil {
		return nil, fmt.Errorf("failed to copy charts dir")
	}

	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open git repo: %v", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree: %v", err)
	}

	opts := git.CheckoutOptions{
		Hash: plumbing.NewHash(RebaseExampleCommit),
	}

	if err := wt.Checkout(&opts); err != nil {
		return nil, fmt.Errorf("failed to checkout commit: %v", err)
	}

	return repo, nil
}

func setupRebase(t *testing.T, pkgName string) (string, *slog.Logger, *git.Repository, *charts.Package, billy.Filesystem, billy.Filesystem) {
	chartsDir, err := os.MkdirTemp(".", "chartsutil-test-"+t.Name()+"-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	t.Cleanup(func() {
		if !t.Failed() {
			if err := os.RemoveAll(chartsDir); err != nil {
				t.Fatalf("failed to remove temp dir: %v", err)
			}
		}
	})

	repo, err := cloneChartsTo(chartsDir)
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

	return chartsDir, logger.WithGroup(t.Name()), repo, pkg, rootFs, pkgFs
}

func assertPackageMessage(t *testing.T, repo *git.Repository, message string) {
	t.Helper()

	head, err := repo.Head()
	if err != nil {
		t.Fatalf("failed to get HEAD: %v", err)
	}

	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		t.Fatalf("failed to get commit: %v", err)
	}

	if commit.Message != message {
		t.Errorf("commit message does not match expected value:\nExpected: '%s'\n   Found: '%s'", message, commit.Message)
	}
}

func assertRebaseMessage(t *testing.T, repo *git.Repository, message string) {
	t.Helper()

	head, err := repo.Head()
	if err != nil {
		t.Fatalf("failed to get HEAD: %v", err)
	}

	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		t.Fatalf("failed to get commit: %v", err)
	}

	parent := commit.ParentHashes[0]

	commit, err = repo.CommitObject(parent)
	if err != nil {
		t.Fatalf("failed to get commit: %v", err)
	}

	if commit.Message != message {
		t.Errorf("commit message does not match expected value:\nExpected: '%s'\n   Found: '%s'", message, commit.Message)
	}
}

func TestArchive(t *testing.T) {
	chartsDir, logger, repo, pkg, rootFs, pkgFs := setupRebase(t, "chartsutil-example-archive")
	_ = chartsDir

	delta := iter.UpstreamDelta{
		URL: "https://github.com/joshmeranda/chartsutil-example-upstream/archive/refs/tags/v0.0.1.tar.gz",
	}

	iter, err := iter.NewSingleIter(pkg.Chart.Upstream, delta)
	if err != nil {
		t.Fatalf("failed to create single iterator: %v", err)
	}

	opts := rebase.Options{
		Logger:            logger,
		Resolver:          &resolve.MergeResolver{Strategy: resolve.StrategyTheirs},
		DisableValidators: true,
	}

	rb, err := rebase.NewRebase(pkg, rootFs, pkgFs, iter, opts)
	if err != nil {
		t.Fatalf("failed to create rebase: %v", err)
	}

	if err := rb.Rebase(); err != nil {
		t.Fatalf("failed to rebase: %v", err)
	}

	assertPackageMessage(t, repo, "Update package.yaml")
	assertRebaseMessage(t, repo, "Updating chartsutil-example-archive to new base "+delta.URL)

	pkg, err = charts.GetPackage(rootFs, pkg.Name)
	if err != nil {
		t.Fatalf("failed to get package: %v", err)
	}

	if pkg.Chart.Upstream.GetOptions().URL != delta.URL {
		t.Errorf("commit does not match expected value:\nExpected: '%s'\n   Found: '%s'", delta.URL, pkg.Chart.Upstream.GetOptions().URL)
	}
}

func TestGitIncremental(t *testing.T) {
	chartsDir, logger, repo, pkg, rootFs, pkgFs := setupRebase(t, "chartsutil-example")
	_ = chartsDir

	newCommit := "933d8b2975efa50cda4dca6234e5e522b8f58cdc"
	delta := iter.UpstreamDelta{
		Commit: &newCommit,
	}

	iter, err := iter.NewGitIter(pkg.Chart.Upstream.GetOptions(), delta)
	if err != nil {
		t.Fatalf("failed to create git iterator: %v", err)
	}

	opts := rebase.Options{
		Logger:            logger,
		Resolver:          &resolve.Blind{},
		DisableValidators: true,
	}

	rb, err := rebase.NewRebase(pkg, rootFs, pkgFs, iter, opts)
	if err != nil {
		t.Fatalf("failed to create rebase: %v", err)
	}

	if err := rb.Rebase(); err != nil {
		t.Fatalf("failed to rebase: %v", err)
	}

	assertPackageMessage(t, repo, "Update package.yaml")
	assertRebaseMessage(t, repo, "Updating chartsutil-example to new base 933d8b2975efa50cda4dca6234e5e522b8f58cdc")

	pkg, err = charts.GetPackage(rootFs, pkg.Name)
	if err != nil {
		t.Fatalf("failed to get package: %v", err)
	}

	if *pkg.Chart.Upstream.GetOptions().Commit != *delta.Commit {
		t.Errorf("commit does not match expected value:\nExpected: '%s'\n   Found: '%s'", *delta.Commit, *pkg.Chart.Upstream.GetOptions().Commit)
	}
}

func TestGitNonIncremental(t *testing.T) {
	chartsDir, logger, repo, pkg, rootFs, pkgFs := setupRebase(t, "chartsutil-example")
	_ = chartsDir

	newCommit := "933d8b2975efa50cda4dca6234e5e522b8f58cdc"
	delta := iter.UpstreamDelta{
		Commit: &newCommit,
	}

	iter, err := iter.NewSingleIter(pkg.Chart.Upstream, delta)
	if err != nil {
		t.Fatalf("failed to create single iterator: %v", err)
	}

	opts := rebase.Options{
		Logger:            logger,
		Resolver:          &resolve.Blind{},
		DisableValidators: true,
	}

	rb, err := rebase.NewRebase(pkg, rootFs, pkgFs, iter, opts)
	if err != nil {
		t.Fatalf("failed to create rebase: %v", err)
	}

	if err := rb.Rebase(); err != nil {
		t.Fatalf("failed to rebase: %v", err)
	}

	assertPackageMessage(t, repo, "Update package.yaml")
	assertRebaseMessage(t, repo, "Updating chartsutil-example to new base 933d8b2975efa50cda4dca6234e5e522b8f58cdc")

	pkg, err = charts.GetPackage(rootFs, pkg.Name)
	if err != nil {
		t.Fatalf("failed to get package: %v", err)
	}

	if *pkg.Chart.Upstream.GetOptions().Commit != *delta.Commit {
		t.Errorf("commit does not match expected value:\nExpected: '%s'\n   Found: '%s'", *delta.Commit, *pkg.Chart.Upstream.GetOptions().Commit)
	}
}
