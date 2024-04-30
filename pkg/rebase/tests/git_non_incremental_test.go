package rebase_test

import (
	"testing"

	"github.com/joshmeranda/chartsutil/pkg/puller"
	"github.com/joshmeranda/chartsutil/pkg/rebase"
	"github.com/joshmeranda/chartsutil/pkg/resolve"
	"github.com/rancher/charts-build-scripts/pkg/charts"
)

func TestGitNonIncremental(t *testing.T) {
	chartsDir, logger, repo, pkg, rootFs, pkgFs := setupRebase(t, "rebase-example")
	_ = chartsDir

	target := "933d8b2975efa50cda4dca6234e5e522b8f58cdc"

	iter, err := puller.NewSingleIter(pkg.Chart.Upstream, target)
	if err != nil {
		t.Fatalf("failed to create single iterator: %v", err)
	}

	opts := rebase.Options{
		Logger:   logger,
		Resolver: &resolve.Blind{},
	}

	rb, err := rebase.NewRebase(pkg, rootFs, pkgFs, iter, opts)
	if err != nil {
		t.Fatalf("failed to create rebase: %v", err)
	}

	if err := rb.Rebase(); err != nil {
		t.Fatalf("failed to rebase: %v", err)
	}

	assertPackageMessage(t, repo, "Update package.yaml")
	assertRebaseMessage(t, repo, "Updating rebase-example to new base 933d8b2975efa50cda4dca6234e5e522b8f58cdc")

	pkg, err = charts.GetPackage(rootFs, pkg.Name)
	if err != nil {
		t.Fatalf("failed to get package: %v", err)
	}

	if *pkg.Chart.Upstream.GetOptions().Commit != target {
		t.Errorf("commit does not match expected value:\nExpected: '%s'\n   Found: '%s'", target, *pkg.Chart.Upstream.GetOptions().Commit)
	}
}
