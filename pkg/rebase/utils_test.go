package rebase_test

import (
	"testing"

	"github.com/joshmeranda/chartsutil/pkg/iter"
	"github.com/joshmeranda/chartsutil/pkg/rebase"
	"github.com/rancher/charts-build-scripts/pkg/charts"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/puller"
)

func TestGetRelevantUpstreamChange(t *testing.T) {
	type testCase struct {
		Name         string
		Upstream     puller.Puller
		UpstreamOpts options.UpstreamOptions
		Expect       string
	}

	cases := []testCase{
		{
			Name: "GithubRepository",
			UpstreamOpts: options.UpstreamOptions{
				URL:          "https://github.com/joshmeranda/chartsutil-example-upstream.git",
				Subdirectory: nil,
				Commit:       rebase.ToPtr("SOME_COMMIT"),
			},
			Expect: "SOME_COMMIT",
		},
		{
			Name: "CheckoutPuller",
			Upstream: &iter.CheckoutPuller{
				Opts: options.UpstreamOptions{
					URL:          "https://github.com/joshmeranda/chartsutil-example-upstream.git",
					Subdirectory: nil,
					Commit:       rebase.ToPtr("SOME_COMMIT"),
				},
			},
			Expect: "SOME_COMMIT",
		},
		{
			Name: "Archive",
			UpstreamOpts: options.UpstreamOptions{
				URL: "https://github.com/joshmeranda/chartsutil-example-upstream/archive/refs/tags/v0.0.1.tar.gz",
			},
			Expect: "https://github.com/joshmeranda/chartsutil-example-upstream/archive/refs/tags/v0.0.1.tar.gz",
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			var err error
			upstream := c.Upstream
			if upstream == nil {
				upstream, err = charts.GetUpstream(c.UpstreamOpts)
				if err != nil {
					t.Fatalf("failed to construct upstream: %v", err)
				}
			}

			actual := rebase.GetRelaventUpstreamChange(upstream)
			if actual != c.Expect {
				t.Errorf("expected %q, got %q", c.Expect, actual)
			}
		})
	}
}

func TestGetUpdateExpression(t *testing.T) {
	type testCase struct {
		Name         string
		Upstream     puller.Puller
		UpstreamOpts options.UpstreamOptions
		Expect       string
	}

	cases := []testCase{
		{
			Name: "GithubRepository",
			UpstreamOpts: options.UpstreamOptions{
				URL:          "https://github.com/joshmeranda/chartsutil-example-upstream.git",
				Subdirectory: nil,
				Commit:       rebase.ToPtr("SOME_COMMIT"),
			},
			Expect: ".commit=\"SOME_COMMIT\"",
		},
		{
			Name: "CheckoutPuller",
			Upstream: &iter.CheckoutPuller{
				Opts: options.UpstreamOptions{
					URL:          "https://github.com/joshmeranda/chartsutil-example-upstream.git",
					Subdirectory: nil,
					Commit:       rebase.ToPtr("SOME_COMMIT"),
				},
			},
			Expect: ".commit=\"SOME_COMMIT\"",
		},
		{
			Name: "Archive",
			UpstreamOpts: options.UpstreamOptions{
				URL: "https://github.com/joshmeranda/chartsutil-example-upstream/archive/refs/tags/v0.0.1.tar.gz",
			},
			Expect: ".url=\"https://github.com/joshmeranda/chartsutil-example-upstream/archive/refs/tags/v0.0.1.tar.gz\"",
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			var err error
			upstream := c.Upstream
			if upstream == nil {
				upstream, err = charts.GetUpstream(c.UpstreamOpts)
				if err != nil {
					t.Fatalf("failed to construct upstream: %v", err)
				}
			}

			actual := rebase.GetUpdateExpression(upstream)
			if actual != c.Expect {
				t.Errorf("expected %q, got %q", c.Expect, actual)
			}
		})
	}
}
