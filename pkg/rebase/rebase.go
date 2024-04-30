package rebase

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	utilpuller "github.com/joshmeranda/chartsutil/pkg/puller"
	"github.com/joshmeranda/chartsutil/pkg/resolve"
	"github.com/mikefarah/yq/v4/pkg/yqlib"
	cp "github.com/otiai10/copy"
	"github.com/rancher/charts-build-scripts/pkg/charts"
	chartspath "github.com/rancher/charts-build-scripts/pkg/path"
	"github.com/rancher/charts-build-scripts/pkg/puller"
	"gopkg.in/op/go-logging.v1"
)

func init() {
	// silence yq logging using module name found here: https://github.com/mikefarah/yq/blob/master/pkg/yqlib/lib.go#L22
	level, err := logging.LogLevel(logging.CRITICAL.String())
	if err != nil {
		panic("bug: failed to silence yq logger: " + err.Error())
	}
	logging.SetLevel(level, "yq-lib")
}

// todo: if nothing to do afterresolving merge conflicts, don't write a commit
// todo: NICE TO HAVE might be a good idea to add some prefix to thesae branch names
// todo: SHOULD check for some obvious errors in rebase (unresolved conflicts, helm templating failing, package can be prepared, etc)

const (
	// ChartsStagingBranchName is the name of the branch used to stage changes for user interaction / review.
	ChartsStagingBranchName = "charts-staging"

	// ChartsQuarantineBranchName is the name of the "working" branch where the incoming changes are applied.
	ChartsQuarantineBranchName = "quarantine"

	// RebaseBackupDir is the directory where the charts are backed up to.
	RebaseBackupDir = ".rebase-backup"
)

type Options struct {
	Logger       *slog.Logger
	Resolver     resolve.Resolver
	EnableBackup bool
}

type Rebase struct {
	Options

	Package *charts.Package
	RootFs  billy.Filesystem
	PkgFs   billy.Filesystem
	Iter    utilpuller.PullerIter

	chartsRepo *git.Repository
	chartsWt   *git.Worktree
}

func NewRebase(pkg *charts.Package, rootFs billy.Filesystem, pkgFs billy.Filesystem, iter utilpuller.PullerIter, opts Options) (*Rebase, error) {
	if pkg.Chart.Upstream.GetOptions().Commit == nil {
		return nil, fmt.Errorf("upstream commit is required")
	}

	if opts.Logger == nil {
		opts.Logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}

	if opts.Resolver == nil {
		opts.Resolver = &resolve.Shell{
			Logger:  opts.Logger.WithGroup("shell"),
			Package: pkg,
		}
	}

	chartsRepo, err := git.PlainOpen(rootFs.Root())
	if err != nil {
		return nil, fmt.Errorf("failed to open charts repository: %w", err)
	}

	chartsWorktree, err := chartsRepo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get charts worktree: %w", err)
	}

	return &Rebase{
		Options: opts,

		Package: pkg,
		RootFs:  rootFs,
		PkgFs:   pkgFs,
		Iter:    iter,

		chartsRepo: chartsRepo,
		chartsWt:   chartsWorktree,
	}, nil
}

func (r *Rebase) commitCharts(msg string) (plumbing.Hash, error) {
	return Commit(r.chartsWt, msg, path.Join("packages", r.Package.Name))
}

func (r *Rebase) handleUpstream(p puller.Puller) error {
	if err := CreateBranch(r.chartsRepo, ChartsStagingBranchName); err != nil {
		return fmt.Errorf("failed to create staging branch: %w", err)
	}
	defer DeleteBranch(r.chartsRepo, ChartsStagingBranchName)

	err := DoOnBranch(r.chartsRepo, r.chartsWt, ChartsStagingBranchName, func(wt *git.Worktree) error {
		if err := p.Pull(r.RootFs, r.PkgFs, r.Package.WorkingDir); err != nil {
			return fmt.Errorf("failed to pull upstream changes: %w", err)
		}

		if _, err := r.commitCharts("saving copied upstream charts"); err != nil {
			return fmt.Errorf("failed to commit original chart: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// need to run as subprocess since go-git Pull only supports fast-forward merges
	cmd := exec.Command("git", "merge", "--squash", "--no-commit", ChartsStagingBranchName)
	cmd.Dir = r.RootFs.Root()

	r.Logger.Info("merging branch", "cmd", cmd.String(), "dir", cmd.Dir)
	if output, err := cmd.CombinedOutput(); err != nil {
		// return fmt.Errorf("failed to merge branch %s: %s", CHARTS_STAGING_BRANCH_NAME, output)
		fmt.Println(string(output))
	}

	isClean, err := IsWorktreeClean(r.chartsWt)
	if err != nil {
		return fmt.Errorf("failed to check if worktree is clean: %w", err)
	}

	if !isClean {
		r.Logger.Info("could not merge automatically, running interactive shell...")
		err := r.Resolver.Resolve(r.chartsWt)

		if errors.Is(err, resolve.ErrAbort) {
			if err := r.chartsWt.Reset(&git.ResetOptions{Mode: git.HardReset}); err != nil {
				return fmt.Errorf("failed to reset worktree after abort: %w", err)
			}

			return err
		}

		if err != nil {
			return fmt.Errorf("received error from shell: %w", err)
		}
	}

	if _, err := r.commitCharts(fmt.Sprintf("brining charts to %s", GetRelaventUpstreamChange(p))); err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}

	return nil
}

func (r *Rebase) updatePatches(whatChanged string) (plumbing.Hash, error) {
	if err := r.Package.GeneratePatch(); err != nil {
		return plumbing.ZeroHash, fmt.Errorf("failed to generate patch: %w", err)
	}

	r.chartsWt.Reset(&git.ResetOptions{Mode: git.HardReset})

	patchDir := path.Join("packages", r.Package.Name, "generated-changes")

	hash, err := Commit(r.chartsWt, fmt.Sprintf("Updating %s to new base %s", r.Package.Name, whatChanged), patchDir)
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("failed to commit patch changes: %w", err)
	}

	if err := r.chartsWt.Reset(&git.ResetOptions{Mode: git.HardReset}); err != nil {
		return plumbing.ZeroHash, fmt.Errorf("failed to revert changes to chart")
	}

	return hash, nil
}

func (r *Rebase) updatePackageYaml(upstream puller.Puller) (plumbing.Hash, error) {
	pkgFile := filepath.Join(r.PkgFs.Root(), chartspath.PackageOptionsFile)
	relativePkgPath := filepath.Join(chartspath.RepositoryPackagesDir, r.Package.Name)

	inPlaceHandler := yqlib.NewWriteInPlaceHandler(pkgFile)
	out, err := inPlaceHandler.CreateTempFile()
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("failed to create temporary file: %w", err)
	}

	decoder := yqlib.YamlFormat.DecoderFactory()
	encoder := yqlib.YamlFormat.EncoderFactory()
	printerWriter := yqlib.NewSinglePrinterWriter(out)
	printer := yqlib.NewPrinter(encoder, printerWriter)

	expression := GetUpdateExpression(upstream)

	r.Logger.Debug("updating package.yaml", "expr", expression)

	allAtOnceEvaluator := yqlib.NewAllAtOnceEvaluator()
	if err := allAtOnceEvaluator.EvaluateFiles(expression, []string{pkgFile}, printer, decoder); err != nil {
		return plumbing.ZeroHash, fmt.Errorf("failed to update package.yaml: %w", err)
	}

	if err := inPlaceHandler.FinishWriteInPlace(true); err != nil {
		return plumbing.ZeroHash, fmt.Errorf("failed to complete in-place update: %w", err)
	}

	hash, err := Commit(r.chartsWt, fmt.Sprintf("Updating %s to new base %s", relativePkgPath, GetRelaventUpstreamChange(upstream)), relativePkgPath)
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("failed to commit package.yaml changes: %w", err)
	}

	return hash, nil
}

func (r *Rebase) Rebase() error {
	isClean, err := IsWorktreeClean(r.chartsWt)
	if err != nil {
		return fmt.Errorf("failed to check if worktree is clean: %w", err)
	}

	if !isClean {
		return fmt.Errorf("charts worktree is not clean")
	}

	if err := CreateBranch(r.chartsRepo, ChartsQuarantineBranchName); err != nil {
		return fmt.Errorf("failed to create quarantine branch: %w", err)
	}
	defer DeleteBranch(r.chartsRepo, ChartsQuarantineBranchName)

	var patchHash plumbing.Hash
	var packageHash plumbing.Hash

	err = DoOnBranch(r.chartsRepo, r.chartsWt, ChartsQuarantineBranchName, func(wt *git.Worktree) error {
		r.Logger.Info("preparing package")

		if err := r.Package.Prepare(); err != nil {
			return fmt.Errorf("failed to prepare the chart: %w", err)
		}

		if _, err := r.commitCharts("preparing package"); err != nil {
			return fmt.Errorf("failed to save charts before pulling new upstream: %w", err)
		}

		var last puller.Puller

		err := utilpuller.ForEach(r.Iter, func(p puller.Puller) error {
			if r.EnableBackup {
				defer func() {
					r.Logger.Info("backing up current state of charts")
					src := filepath.Join(r.PkgFs.Root(), r.Package.WorkingDir)
					dst := filepath.Join(RebaseBackupDir)

					if err := cp.Copy(src, dst, cp.Options{}); err != nil {
						r.Logger.Warn("failed to backup %s: %s", src, err.Error())
					}
				}()
			}

			last = p

			if err := r.handleUpstream(p); err != nil {
				return fmt.Errorf("failed to handle upstream: %w", err)
			}

			return nil
		})
		if err != nil {
			return err
		}

		if last == nil {
			return fmt.Errorf("bug: no upstreams were checked (iterator was empty)")
		}

		if patchHash, err = r.updatePatches(GetRelaventUpstreamChange(last)); err != nil {
			return fmt.Errorf("failed to generate patch: %w", err)
		}

		if packageHash, err = r.updatePackageYaml(last); err != nil {
			return fmt.Errorf("failed to update package.yaml: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// sleep via https://github.com/go-git/go-git/issues/37#issuecomment-1360057685
	r.Logger.Info("letting git catch up...")
	time.Sleep(time.Second * 2)

	cmd := exec.Command("git", "cherry-pick", patchHash.String(), packageHash.String())
	cmd.Dir = r.PkgFs.Root()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to cherry-pick changes: %w", err)
	}

	return nil
}
