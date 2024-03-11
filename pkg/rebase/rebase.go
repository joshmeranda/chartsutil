package rebase

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/rancher/charts-build-scripts/pkg/charts"
)

type Options struct {
	Logger     *slog.Logger
	StagingDir string
}

type Rebase struct {
	Options

	Package  *charts.Package
	ToCommit string
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

	return &Rebase{
		Options: opts,

		Package:  pkg,
		ToCommit: to,
	}, nil
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

		// Auth:              nil,

		// SingleBranch:      false,
		// ReferenceName: "",

		// Depth: 1,
	}

	r.Logger.Info("cloning upstream", "upstream", opts.URL)

	stagingRepo, err := git.PlainClone(r.StagingDir, false, &opts)
	if err != nil {
		return fmt.Errorf("failed to clone upstream repository: %w", err)
	}

	fromHash := plumbing.NewHash(*r.Package.Chart.Upstream.GetOptions().Commit)
	fromCommit, err := stagingRepo.CommitObject(fromHash)
	if err != nil {
		return fmt.Errorf("failed to get commit object for '%s': %w", *r.Package.Chart.Upstream.GetOptions().Commit, err)
	}

	toHash := plumbing.NewHash(r.ToCommit)
	toCommit, err := stagingRepo.CommitObject(toHash)
	if err != nil {
		return fmt.Errorf("failed to get commit object for '%s': %w", r.ToCommit, err)
	}

	subDir := r.Package.Chart.Upstream.GetOptions().Subdirectory

	r.Logger.Info("checking upstream for commits in range", "from", fromHash.String(), "to", toHash.String())

	// increment since to avoid including the current commit
	since := fromCommit.Committer.When.Add(1)
	logOpts := git.LogOptions{
		From:  toHash,
		Order: git.LogOrderDefault,
		PathFilter: func(p string) bool {
			if subDir == nil {
				return true
			}

			return strings.HasPrefix(p, *subDir)
		},
		Since: &since,
	}

	commits, err := stagingRepo.Log(&logOpts)
	if err != nil {
		return fmt.Errorf("failed to get chart commits: %w", err)
	}

	for commit, err := commits.Next(); err == nil; commit, err = commits.Next() {
		fmt.Printf("=== [Rebase.Rebase] 000 %s ===\n", commit.Hash.String()[0:7])
		_ = commit
	}

	_ = fromCommit
	_ = toCommit
	_ = commits

	return nil
}

func (r *Rebase) Close() error {
	if r.StagingDir != "" {
		if err := os.RemoveAll(r.StagingDir); err != nil {
			return fmt.Errorf("failed to remove staging directory: %w", err)
		}
	}

	return nil
}
