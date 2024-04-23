package rebase

import (
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// todo: figure out why branching (create and delete) with go-git is not working

// GetLocalBranchRefName returns the reference name of a given local branch
func GetLocalBranchRefName(branch string) plumbing.ReferenceName {
	return plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", branch))
}

// GetRemoteBranchRefName returns the reference name of a given remote branch
func GetRemoteBranchRefName(branch, remote string) plumbing.ReferenceName {
	return plumbing.ReferenceName(fmt.Sprintf("refs/remote/%s/%s", remote, branch))
}

type WorktreeFunc func(wt *git.Worktree) error

func CreateBranch(r *git.Repository, branch string) error {
	head, err := r.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD: %w", err)
	}

	ref := plumbing.NewHashReference(GetLocalBranchRefName(branch), head.Hash())
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
