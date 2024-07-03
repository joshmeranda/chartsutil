package rebase

import (
	"fmt"
	"strings"

	"github.com/joshmeranda/chartsutil/pkg/iter"
	"github.com/rancher/charts-build-scripts/pkg/puller"
)

func ToPtr[T any](t T) *T {
	return &t
}

// todo: might not be needed with new upstream UpstreamDelta type
func GetRelaventUpstreamChange(upstream puller.Puller) string {
	opts := upstream.GetOptions()

	switch upstream.(type) {
	case puller.GithubRepository, *iter.CheckoutPuller:
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

// todo: should just replace everything or take an upstream delta
func GetUpdateExpression(upstream puller.Puller) string {
	opts := upstream.GetOptions()
	updates := make([]string, 0, 3)

	updates = append(updates, fmt.Sprintf(".url=\"%s\"", opts.URL))

	if opts.Commit != nil {
		updates = append(updates, fmt.Sprintf(".commit=\"%s\"", *opts.Commit))
	}

	if opts.Subdirectory != nil {
		updates = append(updates, fmt.Sprintf(".subdirectory=\"%s\"", *opts.Subdirectory))
	}

	return strings.Join(updates, " | ")
}

func Any[T any](s []T, f func(T) bool) bool {
	for _, t := range s {
		if f(t) {
			return true
		}
	}
	return false
}
