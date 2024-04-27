package rebase

import "github.com/rancher/charts-build-scripts/pkg/puller"

func ToPtr[T any](t T) *T {
	return &t
}

func GetRelaventUpstreamChange(upstream puller.Puller) string {
	opts := upstream.GetOptions()

	switch upstream.(type) {
	case puller.GithubRepository:
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

func Exists(path string) {
}
