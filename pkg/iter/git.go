package iter

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	cp "github.com/otiai10/copy"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/options"
	"github.com/rancher/charts-build-scripts/pkg/puller"
)

type GitIter struct {
	// UpstreamOptions are the options for the current package.
	UpstreamOptions options.UpstreamOptions
	Delta           UpstreamDelta

	fromCommit plumbing.Hash
	toCommit   plumbing.Hash

	// commits []*object.Commit
	deltas []UpstreamDelta

	repo   *git.Repository
	repoWt *git.Worktree

	isInit bool
}

func NewGitIter(opts options.UpstreamOptions, delta UpstreamDelta) (*GitIter, error) {
	if opts.Commit == nil {
		return nil, fmt.Errorf("upstream must have an initial commit")
	}

	if delta.Subdirectory != nil {
		return nil, fmt.Errorf("subdirectory is not supported for git iter")
	}

	iter := &GitIter{
		UpstreamOptions: opts,
	}

	iter.fromCommit = plumbing.NewHash(*opts.Commit)
	iter.toCommit = plumbing.NewHash(*delta.Commit)

	return iter, nil
}

func (i *GitIter) init() error {
	// clone the repository
	tempDir, err := os.MkdirTemp("", "rebase-commit-iter-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}

	cloneOpts := git.CloneOptions{
		URL: i.UpstreamOptions.URL,
	}

	if i.repo, err = git.PlainClone(tempDir, false, &cloneOpts); err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	if i.repoWt, err = i.repo.Worktree(); err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// get the commit iterator
	checkoutOpts := git.CheckoutOptions{
		Hash: i.fromCommit,
	}
	if err := i.repoWt.Checkout(&checkoutOpts); err != nil {
		return fmt.Errorf("failed to checkout final commit: %w", err)
	}

	fromCommit, err := i.repo.CommitObject(i.fromCommit)
	if err != nil {
		return fmt.Errorf("failed to get from commit: %w", err)
	}

	// increment "since" to avoid including the current commit
	since := fromCommit.Committer.When.Add(1)
	logOpts := git.LogOptions{
		From:  i.toCommit,
		Order: git.LogOrderDefault,
		PathFilter: func(p string) bool {
			if i.UpstreamOptions.Subdirectory == nil {
				return true
			}

			return strings.HasPrefix(p, *i.UpstreamOptions.Subdirectory)
		},
		Since: &since,
	}

	commitIter, err := i.repo.Log(&logOpts)
	if err != nil {
		return fmt.Errorf("failed to get commit iterator: %w", err)
	}

	i.deltas = make([]UpstreamDelta, 0)
	commitIter.ForEach(func(c *object.Commit) error {
		delta := i.Delta

		hash := c.Hash.String()
		delta.Commit = &hash

		i.deltas = append(i.deltas, delta)

		return nil
	})

	i.isInit = true

	return nil
}

func (i *GitIter) Next() (puller.Puller, error) {
	if !i.isInit {
		if err := i.init(); err != nil {
			return nil, fmt.Errorf("failed to init git iter: %w", err)
		}
	}

	if len(i.deltas) == 0 {
		return nil, io.EOF
	}

	// deltas are stored in reverse order
	delta := i.deltas[len(i.deltas)-1]
	i.deltas = i.deltas[:len(i.deltas)-1]

	newOpts, err := delta.Apply(i.UpstreamOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to apply upstream delta: %w", err)
	}

	p := &CheckoutPuller{
		Wt:   i.repoWt,
		Opts: newOpts,
	}

	return p, nil
}

func shouldSkipCommit(srcinfo os.FileInfo, src, dest string) (bool, error) {
	if filepath.Base(src) == ".git" {
		return true, nil
	}

	return false, nil
}

type CheckoutPuller struct {
	Wt   *git.Worktree
	Opts options.UpstreamOptions
}

// Pull checks out the commit from upstream options and copies the files to the destination.
//
// Because this method mutatues the filesystem, it is not safe to call concurrently.
func (p *CheckoutPuller) Pull(rootFs billy.Filesystem, fs billy.Filesystem, path string) error {
	checkoutOpts := git.CheckoutOptions{
		Hash: plumbing.NewHash(*p.Opts.Commit),
	}
	if err := p.Wt.Checkout(&checkoutOpts); err != nil {
		return fmt.Errorf("failed to checkout commit: %w", err)
	}

	src := p.Wt.Filesystem.Root()
	if p.Opts.Subdirectory != nil {
		src = filepath.Join(src, *p.Opts.Subdirectory)
	}

	dst := filesystem.GetAbsPath(fs, path)

	if err := cp.Copy(src, dst, cp.Options{Skip: shouldSkipCommit}); err != nil {
		return fmt.Errorf("failed to copy files: %w", err)
	}

	return nil
}

func (p *CheckoutPuller) GetOptions() options.UpstreamOptions {
	return p.Opts
}

func (p *CheckoutPuller) IsWithinPackage() bool {
	return false
}
