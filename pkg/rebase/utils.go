package rebase

import (
	"strings"

	"github.com/go-git/go-git/v5/plumbing/object"
)

func messageFromCommit(commit *object.Commit) string {
	i := strings.Index(commit.Message, "\n")

	if i == -1 {
		return commit.Message
	}

	return commit.Message[:i]
}
