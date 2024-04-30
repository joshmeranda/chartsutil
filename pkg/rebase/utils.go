package rebase

import (
	"fmt"
	"strings"

	utilpuller "github.com/joshmeranda/chartsutil/pkg/puller"
	"github.com/rancher/charts-build-scripts/pkg/puller"
)

func ToPtr[T any](t T) *T {
	return &t
}

func GetRelaventUpstreamChange(upstream puller.Puller) string {
	opts := upstream.GetOptions()

	switch upstream.(type) {
	case puller.GithubRepository, *utilpuller.CheckoutPuller:
		if opts.Commit == nil {
			panic("bug: found nil commit on github repository")
		}

		return *opts.Commit
	default:
		if opts.URL == "" {
			panic("bug: found empty URL on upstream")
		}

		return opts.URL
	}
}

func GetUpdateExpression(upstream puller.Puller) string {
	opts := upstream.GetOptions()
	updates := make([]string, 0, 3)

	switch upstream.(type) {

	case puller.GithubRepository, *utilpuller.CheckoutPuller:
		updates = append(updates, fmt.Sprintf(".commit=\"%s\"", *opts.Commit))
	default:
		updates = append(updates, fmt.Sprintf(".url=\"%s\"", opts.URL))
	}

	return strings.Join(updates, " | ")
}
