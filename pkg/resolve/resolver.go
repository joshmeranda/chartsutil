package resolve

import (
	"fmt"

	"github.com/go-git/go-git/v5"
)

var (
	ErrAbort = fmt.Errorf("rebase aborted by user")
)

// Resolver defines how to handle conflicts between the stating and quarantine brnanches, and stages the conflicting files in git once resolved.
type Resolver interface {
	Resolve(*git.Worktree) error
}

// Aborter immediately aborts the rebase.
type Aborter struct{}

func (a Aborter) Resolve(*git.Worktree) error {
	return ErrAbort
}

// Blind behaves as if there are no conflicts and simply stages the changes files without doing anything.
type Blind struct{}

func (b Blind) Resolve(wt *git.Worktree) error {
	if _, err := wt.Add("."); err != nil {
		return fmt.Errorf("failed to stage changed files: %w", err)
	}

	return nil
}
