package rebase

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/otiai10/copy"
	"github.com/rancher/charts-build-scripts/pkg/charts"
)

var (
	commitOpts = git.CommitOptions{
		Author: &object.Signature{
			Name: "REBASE_BOT",
		},
	}
)

const (
	CHARTS_STAGING_BRANCH_NAME   = "charts-staging"
	CHARTS_QUARANTNE_BRANCH_NAME = "quarantine"
)

type Options struct {
	Logger      *slog.Logger
	StagingDir  string
	ChartsDir   string
	Incremental bool
}

type Rebase struct {
	Options

	Package  *charts.Package
	ToCommit string

	upstreamRepo *git.Repository
	chartsRepo   *git.Repository
}

func NewRebase(pkg *charts.Package, to string, opts Options) (*Rebase, error) {
	if !strings.HasSuffix(pkg.Chart.Upstream.GetOptions().URL, ".git") {
		return nil, fmt.Errorf("can only rebase packages with github upstreams")
	}

	if pkg.Chart.Upstream.GetOptions().Commit == nil {
		return nil, fmt.Errorf("upstream commit is required")
	}

	if opts.Logger == nil {
		opts.Logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}

	chartsRepo, err := git.PlainOpen(opts.ChartsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open charts repository: %w", err)
	}

	return &Rebase{
		Options: opts,

		Package:  pkg,
		ToCommit: to,

		chartsRepo: chartsRepo,
	}, nil
}

func (r *Rebase) commitCharts(msg string) error {
	chartsRepo, err := git.PlainOpen(r.ChartsDir)
	if err != nil {
		return fmt.Errorf("failed to open charts repository: %w", err)
	}

	chartsWt, err := chartsRepo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get charts worktree: %w", err)
	}

	if _, err := chartsWt.Add(path.Join("packages", r.Package.Name)); err != nil {
		return fmt.Errorf("failed to stage chart changes: %w", err)
	}

	if msg == "" {
		msg = r.Package.Name
	}

	if _, err = chartsWt.Commit(fmt.Sprintf("rebase: %s", msg), &commitOpts); err != nil {
		return fmt.Errorf("failed to commit chart changes: %w", err)
	}

	return nil
}

func (r *Rebase) getUpstreamCommitsBetween(from *object.Commit, to *object.Commit) ([]*object.Commit, error) {
	subDir := r.Package.Chart.Upstream.GetOptions().Subdirectory

	r.Logger.Info("checking upstream for commits in range", "from", from.Hash.String(), "to", to.Hash.String())

	// increment since to avoid including the current commit
	since := from.Committer.When.Add(1)
	logOpts := git.LogOptions{
		From:  to.Hash,
		Order: git.LogOrderDefault,
		PathFilter: func(p string) bool {
			if subDir == nil {
				return true
			}

			return strings.HasPrefix(p, *subDir)
		},
		Since: &since,
	}

	commitIter, err := r.upstreamRepo.Log(&logOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to get chart commits: %w", err)
	}

	commits := make([]*object.Commit, 0)
	err = commitIter.ForEach(func(c *object.Commit) error {
		commits = append(commits, c)
		return nil
	})

	return commits, nil
}

func (r *Rebase) handleCommit(commit *object.Commit) error {
	if err := CheckoutHash(r.upstreamRepo, commit.Hash); err != nil {
		return fmt.Errorf("failed to checkout commit '%s': %w", commit.Hash.String(), err)
	}

	if err := CreateBranch(r.upstreamRepo, CHARTS_STAGING_BRANCH_NAME); err != nil {
		return fmt.Errorf("failed to create staging branch: %w", err)
	}
	defer DeleteBranch(r.chartsRepo, CHARTS_STAGING_BRANCH_NAME)

	err := DoOnBranch(r.chartsRepo, CHARTS_STAGING_BRANCH_NAME, func(wt *git.Worktree) error {
		src := filepath.Join(r.StagingDir, *r.Package.Upstream.GetOptions().Subdirectory)
		dst := filepath.Join(r.ChartsDir, "packages", r.Package.Name, "charts")

		if err := copy.Copy(src, dst); err != nil {
			return fmt.Errorf("failed to copy files from stage to worktree: %w", err)
		}

		if err := r.commitCharts("saving copied upstream charts"); err != nil {
			return fmt.Errorf("failed to commit original chart: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// need to run as subprocess since go-git Pull only supports fast-forward merges
	cmd := exec.Command("git", "merge", "--no-ff", "--no-commit", CHARTS_STAGING_BRANCH_NAME)
	cmd.Dir = r.ChartsDir

	r.Logger.Info("merging branch", "cmd", cmd.String(), "dir", cmd.Dir)
	if output, err := cmd.CombinedOutput(); err != nil {
		// return fmt.Errorf("failed to merge branch %s: %s", CHARTS_STAGING_BRANCH_NAME, output)
		fmt.Println(string(output))
	}

	// todo: get user input
	var input string
	fmt.Println("Press [enter] to continue...")
	fmt.Scanln(&input)

	return nil
}

func (r *Rebase) Rebase() error {
	var err error

	if r.StagingDir == "" {
		r.StagingDir, err = os.MkdirTemp("", "chart-utils-rebase-*")
		if err != nil {
			return fmt.Errorf("failed to create temporary directory: %w", err)
		}
	}

	opts := git.CloneOptions{
		URL: r.Package.Chart.Upstream.GetOptions().URL,
	}

	r.upstreamRepo, err = git.PlainOpen(r.StagingDir)
	if errors.Is(git.ErrRepositoryNotExists, err) {
		r.Logger.Info("no repository exists at staging dir, attempting to clone...", "staging-dir", r.StagingDir, "url", opts.URL)

		r.upstreamRepo, err = git.PlainClone(r.StagingDir, false, &opts)
		if err != nil && !errors.Is(git.ErrRepositoryAlreadyExists, err) {
			return fmt.Errorf("failed to clone upstream repository: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to open staging repository: %w", err)
	} else {
		r.Logger.Info("using erxisting staging repository", "dir", r.StagingDir)
	}

	fromHash := plumbing.NewHash(*r.Package.Chart.Upstream.GetOptions().Commit)
	fromCommit, err := r.upstreamRepo.CommitObject(fromHash)
	if err != nil {
		return fmt.Errorf("failed to get commit object for '%s': %w", *r.Package.Chart.Upstream.GetOptions().Commit, err)
	}

	toHash := plumbing.NewHash(r.ToCommit)
	toCommit, err := r.upstreamRepo.CommitObject(toHash)
	if err != nil {
		return fmt.Errorf("failed to get commit object for '%s': %w", r.ToCommit, err)
	}

	upstreamWt, err := r.upstreamRepo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get staging worktree: %w", err)
	}

	if err := upstreamWt.Checkout(&git.CheckoutOptions{
		Hash: toHash,
	}); err != nil {
		return fmt.Errorf("failed to checkout staging repository: %w", err)
	}

	var commits []*object.Commit
	if r.Incremental {
		commits, err = r.getUpstreamCommitsBetween(fromCommit, toCommit)
		if err != nil {
			return fmt.Errorf("failed to get upstream commits: %w", err)
		}
	} else {
		commits = []*object.Commit{toCommit}
	}

	if err := CreateBranch(r.chartsRepo, CHARTS_QUARANTNE_BRANCH_NAME); err != nil {
		return fmt.Errorf("failed to create quarantine branch: %w", err)
	}
	defer DeleteBranch(r.chartsRepo, CHARTS_QUARANTNE_BRANCH_NAME)

	err = DoOnBranch(r.chartsRepo, CHARTS_QUARANTNE_BRANCH_NAME, func(wt *git.Worktree) error {
		for _, commit := range commits {
			r.Logger.Info("preparing package")
			err = r.Package.Prepare()
			if err != nil {
				return fmt.Errorf("failed to prepare the chart")
			}

			if err := r.commitCharts("copying current 	charts"); err != nil {
				return fmt.Errorf("failed to commit prepared package: %w", err)
			}

			if err := r.handleCommit(commit); err != nil {
				return fmt.Errorf("error bringing chart to commit: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (r *Rebase) Close() error {
	// if r.StagingDir != "" {
	// 	if err := os.RemoveAll(r.StagingDir); err != nil {
	// 		return fmt.Errorf("failed to remove staging directory: %w", err)
	// 	}
	// }

	return nil
}
