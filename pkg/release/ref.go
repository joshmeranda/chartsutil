package release

import (
	"fmt"
	"net/url"
	"strings"
)

type RepoRef struct {
	Owner string
	Name  string
}

func RepoRefFromUrl(s string) (RepoRef, error) {
	u, err := url.ParseRequestURI(s)
	if err != nil {
		return RepoRef{}, fmt.Errorf("provided upstream '%s' is not a valid url: %w", s, err)
	}

	components := strings.Split(strings.TrimSuffix(u.Path, ".git"), "/")

	// require 3 paths to take the leading '/'
	if l := len(components); l != 3 {
		return RepoRef{}, fmt.Errorf("expected upstream path to have only 2 compoents but found '%d': %v", l, components)
	}

	ref := RepoRef{
		Owner: components[len(components)-2],
		Name:  components[len(components)-1],
	}

	return ref, nil
}
