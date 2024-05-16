package release_test

import (
	"context"
	"regexp"
	"testing"

	"github.com/joshmeranda/chartsutil/pkg/release"
)

func TestTagPattern(t *testing.T) {
	type Case struct {
		Tag         string
		Expectmatch bool
	}

	cases := []Case{
		{
			Tag:         "v1.0.0",
			Expectmatch: true,
		},
		{
			Tag:         "1.0.0",
			Expectmatch: true,
		},
		{
			Tag:         "1.0.0-rc-8",
			Expectmatch: true,
		},

		{
			Tag:         "not a semvar",
			Expectmatch: false,
		},
		{
			Tag:         "some-prefix-v1.0.0",
			Expectmatch: true,
		},
	}

	pattern := regexp.MustCompile(release.DefaultReleaseNamePattern)

	for _, c := range cases {
		actual := pattern.MatchString(c.Tag)
		if actual != c.Expectmatch {
			t.Errorf("expected %t but got %t for tag %s", c.Expectmatch, actual, c.Tag)
		}
	}
}

func TestReleasesForUpstream(t *testing.T) {
	ref := release.RepoRef{
		Owner: "joshmeranda",
		Name:  "chartsutil-example-upstream",
	}

	query := release.ReleaseQuery{
		// Since:       time.Time{},
		NamePattern: regexp.MustCompile(release.DefaultReleaseNamePattern),
	}

	actual, err := release.ReleasesForUpstream(context.Background(), ref, query)
	if err != nil {
		t.Fatalf("failed to fetch releases: %v", err)
	}

	expected := []release.Release{
		{
			Name: "v0.0.1",
			Hash: "main",
		},
		{
			Name: "v0.0.0",
			Hash: "main",
		},
	}

	if len(actual) != len(expected) {
		t.Fatalf("expected %d releases but got %d", len(expected), len(actual))
	}

	for i := 0; i < len(actual); i++ {
		if actual[i].Name != expected[i].Name {
			t.Errorf("expected release name %s but got %s", expected[i].Name, actual[i].Name)
		}
		if actual[i].Hash != expected[i].Hash {
			t.Errorf("expected release hash %s but got %s", expected[i].Hash, actual[i].Hash)
		}
	}
}
