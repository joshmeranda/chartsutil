package iter_test

import (
	"testing"

	"github.com/joshmeranda/chartsutil/pkg/iter"
	"github.com/joshmeranda/chartsutil/pkg/rebase"
	"github.com/rancher/charts-build-scripts/pkg/options"
)

func assertNextCommit(t *testing.T, iter *iter.GitIter, expected string) {
	t.Helper()

	upstream, err := iter.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if *upstream.GetOptions().Commit != expected {
		t.Fatalf("expected commit %s, got %s", expected, *upstream.GetOptions().Commit)
	}
}

func TestNewGitIter(t *testing.T) {
	opts := options.UpstreamOptions{
		URL:    "https://github.com/joshmeranda/chartsutil-example-upstream.git",
		Commit: rebase.ToPtr("d71e29b3f50fbe2ff4d3c2dd95684739b4d00310"),
	}
	delta := iter.UpstreamDelta{
		Commit: rebase.ToPtr("933d8b2975efa50cda4dca6234e5e522b8f58cdc"),
	}

	iter, err := iter.NewGitIter(opts, delta)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertNextCommit(t, iter, "be3f43b07c7c3f034f6aada9af90a812a0b44aa8")
	assertNextCommit(t, iter, "553ab27381dbc13c63c92ffb35f5c7634b52dd26")
	assertNextCommit(t, iter, "933d8b2975efa50cda4dca6234e5e522b8f58cdc")
}
