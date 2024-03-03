package chartsutil_test

import (
	"regexp"
	"testing"

	chartsutil "github.com/joshmeranda/chartsutil/pkg"
)

func assertBool(t *testing.T, expected bool, condition bool) {
	if expected != condition {
		t.Errorf("expected %t but got %t", expected, condition)
	}
}

func assertTrue(t *testing.T, condition bool) {
	assertBool(t, true, condition)
}

func assertFalse(t *testing.T, condition bool) {
	assertBool(t, false, condition)
}

func TestTagPattern(t *testing.T) {
	tagRegexp := regexp.MustCompile(chartsutil.DefaultTagPattern)

	assertTrue(t, tagRegexp.MatchString("v1.0.0"))
	assertTrue(t, tagRegexp.MatchString("1.0.0"))
	assertTrue(t, tagRegexp.MatchString("1.0.0-rc-8"))

	assertFalse(t, tagRegexp.MatchString("not a semvar"))
	assertFalse(t, tagRegexp.MatchString("some-prefix-v1.0.0"))
}
