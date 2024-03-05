package chartsutil_test

import (
	"regexp"
	"testing"

	chartsutil "github.com/joshmeranda/chartsutil/pkg"
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

	tagRegexp := regexp.MustCompile(chartsutil.DefaultTagPattern)

	for _, c := range cases {
		actual := tagRegexp.MatchString(c.Tag)
		if actual != c.Expectmatch {
			t.Errorf("expected %t but got %t for tag %s", c.Expectmatch, actual, c.Tag)
		}
	}
}
