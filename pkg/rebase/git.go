package rebase

import (
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// GetLocalBranchRefName returns the reference name of a given local branch
func GetLocalBranchRefName(branch string) plumbing.ReferenceName {
	return plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", branch))
}

// GetRemoteBranchRefName returns the reference name of a given remote branch
func GetRemoteBranchRefName(branch, remote string) plumbing.ReferenceName {
	return plumbing.ReferenceName(fmt.Sprintf("refs/remote/%s/%s", remote, branch))
}

type WorktreeFunc func(wt *git.Worktree) error

// CreateBranch creates a new branch with the given hash as the head, or the current HEAD hash if empty.
func CreateBranch(r *git.Repository, branch string, hash plumbing.Hash) error {
	if hash == plumbing.ZeroHash {
		head, err := r.Head()
		if err != nil {
			return fmt.Errorf("failed to get HEAD: %w", err)
		}

		hash = head.Hash()
	}

	ref := plumbing.NewHashReference(GetLocalBranchRefName(branch), hash)
	if err := r.Storer.SetReference(ref); err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}

	return nil
}

func DeleteBranch(r *git.Repository, branch string) error {
	if err := r.Storer.RemoveReference(GetLocalBranchRefName(branch)); err != nil {
		return fmt.Errorf("failed to delete branch: %w", err)
	}

	return nil
}

func DoOnWorktree(wt *git.Worktree, f WorktreeFunc) error {
	if err := f(wt); err != nil {
		return err
	}

	return nil
}

func SwitchToBranch(wt *git.Worktree, branch string) error {
	err := DoOnWorktree(wt, func(wt *git.Worktree) error {
		opts := &git.CheckoutOptions{
			Branch: GetLocalBranchRefName(branch),
		}

		if err := wt.Checkout(opts); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil
	}

	return nil
}

func CheckoutHash(wt *git.Worktree, hash plumbing.Hash) error {
	err := DoOnWorktree(wt, func(wt *git.Worktree) error {
		opts := &git.CheckoutOptions{
			Hash: hash,
		}

		if err := wt.Checkout(opts); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil
	}

	return nil
}

func DoOnBranch(r *git.Repository, wt *git.Worktree, branch string, f WorktreeFunc) error {
	currentBranch, err := r.Head()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	if err := SwitchToBranch(wt, branch); err != nil {
		return err
	}
	defer SwitchToBranch(wt, currentBranch.Name().Short())

	err = DoOnWorktree(wt, f)
	if err != nil {
		return err
	}

	return nil
}

func IsWorktreeClean(wt *git.Worktree) (bool, error) {
	status, err := wt.Status()
	if err != nil {
		return false, fmt.Errorf("failed to get worktree status: %w", err)
	}

	return status.IsClean(), nil
}

func Commit(wt *git.Worktree, shouldCherryPick bool, message string, paths ...string) (plumbing.Hash, error) {
	for _, path := range paths {
		if _, err := wt.Add(path); err != nil {
			return plumbing.ZeroHash, fmt.Errorf("failed to add '%s' to index: %w", path, err)
		}
	}

	if message == "" {
		message = fmt.Sprintf("made changes to %s", strings.Join(paths, ", "))
	}

	commitOpts := &git.CommitOptions{}
	if !shouldCherryPick {
		commitOpts.Author = &object.Signature{
			Name: "chartsutil-rebase",
		}
	}

	hash, err := wt.Commit(message, commitOpts)
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("failed to commit changes: %w", err)
	}

	return hash, nil
}
