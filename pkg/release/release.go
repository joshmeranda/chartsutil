package release

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/google/go-github/github"
)

const (
	DefaultReleaseNamePattern = "^v?[0-9]+\\.[0-9]+\\.[0-9]+(-rc[-.]?[0-9]+)?$"
	MaxAvailableReleases      = 1000
)

type ReleaseQuery struct {
	Since       time.Time
	NamePattern *regexp.Regexp
}

type Release struct {
	Name string
	Age  time.Duration
	Hash string
}

func ReleasesForUpstream(ctx context.Context, ref RepoRef, query ReleaseQuery) ([]Release, error) {
	client := github.NewClient(nil)

	matchingReleases := make([]Release, 0)
	now := time.Now().UTC()

	listOpts := &github.ListOptions{
		PerPage: 100,
	}

	for i := 1; i*listOpts.PerPage <= MaxAvailableReleases+1; i++ {
		releases, response, err := client.Repositories.ListReleases(ctx, ref.Owner, ref.Name, listOpts)
		if err != nil {
			return nil, fmt.Errorf("error fetching releases for chart upstream: %w", err)
		}

		for _, release := range releases {
			if release.CreatedAt.Time.After(query.Since) && query.NamePattern.MatchString(*release.Name) {
				entry := Release{
					Name: *release.Name,
					Age:  now.Sub(release.CreatedAt.Time),
					Hash: *release.TargetCommitish,
				}

				matchingReleases = append(matchingReleases, entry)
			}
		}

		if response.NextPage == 0 {
			break
		}

		listOpts.Page = response.NextPage
	}

	return matchingReleases, nil
}
