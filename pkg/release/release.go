package release

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/google/go-github/github"
	chartsutil "github.com/joshmeranda/chartsutil/pkg"
)

const DefaultReleaseNamePattern = "^v?[0-9]+\\.[0-9]+\\.[0-9]+(-rc[-.]?[0-9]+)?$"

type ReleaseQuery struct {
	Since       time.Time
	NamePattern *regexp.Regexp
}

type Release struct {
	Name string
	Age  time.Duration
	Hash string
}

func ReleasesForUpstream(ctx context.Context, ref chartsutil.RepoRef, query ReleaseQuery) ([]Release, error) {
	client := github.NewClient(nil)
	releases, _, err := client.Repositories.ListReleases(ctx, ref.Owner, ref.Name, &github.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error fetching releases for chart upstream: %w", err)
	}

	now := time.Now().UTC()

	matchingReleases := make([]Release, 0)
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

	return matchingReleases, nil
}
