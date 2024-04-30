package puller

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

	fromCommit plumbing.Hash
	toCommit   plumbing.Hash

	commits []*object.Commit

	repo   *git.Repository
	repoWt *git.Worktree

	isInit bool
}

func NewGitIter(opts options.UpstreamOptions, toCommit string) (*GitIter, error) {
	iter := &GitIter{
		UpstreamOptions: opts,
	}

	if opts.Commit == nil {
		return nil, fmt.Errorf("upstream must have an initial commit")
	}

	if toCommit == "" {
		return nil, fmt.Errorf("to commit must be specified")
	}

	iter.fromCommit = plumbing.NewHash(*opts.Commit)
	iter.toCommit = plumbing.NewHash(toCommit)

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

	i.commits = make([]*object.Commit, 0)
	commitIter.ForEach(func(c *object.Commit) error {
		i.commits = append(i.commits, c)
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

	if len(i.commits) == 0 {
		return nil, io.EOF
	}

	commitStr := i.commits[len(i.commits)-1].Hash.String()
	i.commits = i.commits[:len(i.commits)-1]

	p := &CheckoutPuller{
		wt: i.repoWt,
		opts: options.UpstreamOptions{
			URL:          i.UpstreamOptions.URL,
			Subdirectory: i.UpstreamOptions.Subdirectory,
			Commit:       &commitStr,
		},
	}

	return p, nil
}

func shouldSkip(srcinfo os.FileInfo, src, dest string) (bool, error) {
	if filepath.Base(src) == ".git" {
		return true, nil
	}

	return false, nil
}

type CheckoutPuller struct {
	wt   *git.Worktree
	opts options.UpstreamOptions
}

// Pull checks out the commit from upstream options and copies the files to the destination.
//
// Because this method mutatues the filesystem, it is not safe to call concurrently.
func (p *CheckoutPuller) Pull(rootFs billy.Filesystem, fs billy.Filesystem, path string) error {
	checkoutOpts := git.CheckoutOptions{
		Hash: plumbing.NewHash(*p.opts.Commit),
	}
	if err := p.wt.Checkout(&checkoutOpts); err != nil {
		return fmt.Errorf("failed to checkout commit: %w", err)
	}

	src := p.wt.Filesystem.Root()
	if p.opts.Subdirectory != nil {
		src = filepath.Join(src, *p.opts.Subdirectory)
	}

	dst := filesystem.GetAbsPath(fs, path)

	if err := cp.Copy(src, dst, cp.Options{Skip: shouldSkip}); err != nil {
		return fmt.Errorf("failed to copy files: %w", err)
	}

	return nil
}

func (p *CheckoutPuller) GetOptions() options.UpstreamOptions {
	return p.opts
}

func (p *CheckoutPuller) IsWithinPackage() bool {
	return false
}
