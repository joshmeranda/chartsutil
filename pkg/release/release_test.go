package release_test

import (
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
			Expectmatch: false,
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
