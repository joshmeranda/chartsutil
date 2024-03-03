package chartsutil

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

const DefaultTagPattern = "^v?[0-9]+\\.[0-9]+\\.[0-9]+(-rc[-.]?[0-9]+)?$"

type TagEntry struct {
	Name string
	Age  time.Duration
	Hash string
}

type TagQuery struct {
	Since     time.Time
	TagFilter regexp.Regexp
}

func TagsFromRepo(repo *git.Repository, query TagQuery) ([]TagEntry, error) {
	tags, err := repo.Tags()
	if err != nil {
		return nil, err

	}

	foundTags := make([]TagEntry, 0)
	now := time.Now().Local()

	err = tags.ForEach(func(ref *plumbing.Reference) error {
		tagName := strings.TrimPrefix(ref.Name().String(), "refs/tags/")
		tagHash := ref.Hash()
		tagCommit, err := repo.CommitObject(tagHash)
		if err != nil {
			return fmt.Errorf("object is not a commit")
		}

		if tagCommit.Committer.When.Local().After(query.Since) && query.TagFilter.MatchString(tagName) {
			foundTags = append(foundTags, TagEntry{
				Name: tagName,
				Age:  now.Sub(tagCommit.Committer.When),
				Hash: tagHash.String(),
			})
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get tags from repo")
	}

	return foundTags, nil
}
