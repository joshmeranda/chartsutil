package rebase_test

import (
	"testing"

	"github.com/joshmeranda/chartsutil/pkg/puller"
	"github.com/joshmeranda/chartsutil/pkg/rebase"
	"github.com/joshmeranda/chartsutil/pkg/resolve"
	"github.com/rancher/charts-build-scripts/pkg/charts"
)

func TestArchive(t *testing.T) {
	chartsDir, logger, repo, pkg, rootFs, pkgFs := setupRebase(t, "rebase-example-archive")
	_ = chartsDir

	target := "https://github.com/joshmeranda/chartsutil-example-upstream/archive/refs/tags/v0.0.1.tar.gz"

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
	assertRebaseMessage(t, repo, "Updating rebase-example-archive to new base https://github.com/joshmeranda/chartsutil-example-upstream/archive/refs/tags/v0.0.1.tar.gz")

	pkg, err = charts.GetPackage(rootFs, pkg.Name)
	if err != nil {
		t.Fatalf("failed to get package: %v", err)
	}

	if pkg.Chart.Upstream.GetOptions().URL != target {
		t.Errorf("commit does not match expected value:\nExpected: '%s'\n   Found: '%s'", target, pkg.Chart.Upstream.GetOptions().URL)
	}
}
