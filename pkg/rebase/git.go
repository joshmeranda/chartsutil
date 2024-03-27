package rebase

import (
	"fmt"
	"os/exec"

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
	// head, err := r.Head()
	// if err != nil {
	// 	return fmt.Errorf("failed to get HEAD: %w", err)
	// }

	// ref := plumbing.NewHashReference(GetLocalBranchRefName(branch), head.Hash())
	// if err := r.Storer.SetReference(ref); err != nil {
	// 	return fmt.Errorf("failed to create branch: %w", err)
	// }

	cmd := exec.Command("git", "branch", branch)
	cmd.Dir = "/home/wrinkle/workspaces/joshmeranda/rancher-charts"
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}

	return nil
}

func DeleteBranch(r *git.Repository, branch string) error {
	// if err := r.Storer.RemoveReference(GetLocalBranchRefName(branch)); err != nil {
	// 	return fmt.Errorf("failed to delete branch: %w", err)
	// }

	cmd := exec.Command("git", "branch", "-D", branch)
	cmd.Dir = "/home/wrinkle/workspaces/joshmeranda/rancher-charts"
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}

	return nil
}

func DoOnWorktree(r *git.Repository, f WorktreeFunc) error {
	wt, err := r.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	if err := f(wt); err != nil {
		return err
	}

	return nil
}

func SwitchToBranch(r *git.Repository, branch string) error {
	err := DoOnWorktree(r, func(wt *git.Worktree) error {
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

func CheckoutHash(r *git.Repository, hash plumbing.Hash) error {
	err := DoOnWorktree(r, func(wt *git.Worktree) error {
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

func DoOnBranch(r *git.Repository, branch string, f WorktreeFunc) error {
	currentBranch, err := r.Head()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	err = SwitchToBranch(r, branch)
	if err != nil {
		return err
	}
	defer SwitchToBranch(r, currentBranch.Name().Short())

	err = DoOnWorktree(r, f)
	if err != nil {
		return err
	}

	return nil
}
