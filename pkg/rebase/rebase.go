package rebase

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/joshmeranda/chartsutil/pkg/iter"
	"github.com/joshmeranda/chartsutil/pkg/resolve"
	"github.com/mikefarah/yq/v4/pkg/yqlib"
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

// todo: NICE TO HAVE might be a good idea to add some prefix to thesae branch names
// todo: MUST ensure that there are no empty commits when cherry-picking

const (
	// ChartsStagingBranchName is the name of the branch used to stage changes for user interaction / review.
	ChartsStagingBranchName = "charts-staging"

	// ChartsQuarantineBranchName is the name of the "working" branch where the incoming changes are applied.
	ChartsQuarantineBranchName = "quarantine"

	// RebaseBackupDir is the directory where the charts are backed up to.
	RebaseBackupDir = ".rebase-backup"
)

type Options struct {
	Logger            *slog.Logger
	Resolver          resolve.Resolver
	EnableBackup      bool
	DisableValidators bool
}

type Rebase struct {
	Options

	Package *charts.Package
	RootFs  billy.Filesystem
	PkgFs   billy.Filesystem
	Iter    iter.UpstreamIter

	chartsRepo   *git.Repository
	chartsWt     *git.Worktree
	startingHead plumbing.Hash

	validators []PackageValidateFunc
}

func NewRebase(pkg *charts.Package, rootFs billy.Filesystem, pkgFs billy.Filesystem, iter iter.UpstreamIter, opts Options) (*Rebase, error) {
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

	head, err := chartsRepo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", err)
	}

	var validators []PackageValidateFunc
	if opts.DisableValidators {
		validators = []PackageValidateFunc{}
	} else {
		validators = []PackageValidateFunc{
			ValidateWorktree,
			ValidatePatternNotFoundFactory("<<<<<<< HEAD"),
			ValidateHelmLint,
		}
	}

	return &Rebase{
		Options: opts,

		Package: pkg,
		RootFs:  rootFs,
		PkgFs:   pkgFs,
		Iter:    iter,

		chartsRepo:   chartsRepo,
		chartsWt:     chartsWorktree,
		startingHead: head.Hash(),

		validators: validators,
	}, nil
}

func (r *Rebase) commitCharts(msg string) (plumbing.Hash, error) {
	pkgDir := path.Join(chartspath.RepositoryPackagesDir, r.Package.Name)

	chartPaths := make([]string, len(r.Package.AdditionalCharts)+1)

	chartPaths[0] = filepath.Join(pkgDir, r.Package.WorkingDir)
	for i, chart := range r.Package.AdditionalCharts {
		chartPaths[i+1] = filepath.Join(pkgDir, chart.WorkingDir)
	}

	return Commit(r.chartsWt, false, msg, chartPaths...)
}

func (r *Rebase) resolve() error {
resolveLoop:
	for {
		err := r.Resolver.Resolve(r.chartsWt)

		if errors.Is(err, resolve.ErrAbort) {
			if err := r.chartsWt.Reset(&git.ResetOptions{Mode: git.HardReset}); err != nil {
				return fmt.Errorf("failed to reset worktree after abort: %w", err)
			}

			return err
		}

		if err != nil {
			return fmt.Errorf("received error from resolver: %w", err)
		}

		for _, validator := range r.validators {
			err := validator(r.Package, r.chartsWt, r.PkgFs)
			if errors.Is(err, ValidateError{}) {
				r.Logger.Error("failed validation", "err", err)
				continue resolveLoop
			} else if err != nil {
				return fmt.Errorf("could not verify chart: %w", err)
			}
		}

		r.Logger.Info("worktree has passed all validators")

		break
	}

	r.Logger.Info("all conflicts resolved!")

	return nil
}

func (r *Rebase) handleUpstream(upstream puller.Puller) error {
	r.Logger.Info("bringing charts to next upstream", "upstream", UpstreamRef(upstream.GetOptions()))

	if err := CreateBranch(r.chartsRepo, ChartsStagingBranchName, r.startingHead); err != nil {
		return fmt.Errorf("failed to create staging branch: %w", err)
	}
	defer DeleteBranch(r.chartsRepo, ChartsStagingBranchName)

	err := DoOnBranch(r.chartsRepo, r.chartsWt, ChartsStagingBranchName, func(wt *git.Worktree) error {
		if err := upstream.Pull(r.RootFs, r.PkgFs, r.Package.WorkingDir); err != nil {
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

	r.Logger.Info("merging branch", "dir", cmd.Dir, "cmd", cmd.String())
	output, err := cmd.CombinedOutput()

	if err != nil {
		switch err.(type) {
		case *exec.ExitError:
			fmt.Println(string(output))
		default:
			return fmt.Errorf("could not run merge command '%s': %w", cmd.String(), err)
		}

	}

	r.Logger.Info("could not merge automatically, running resolver")

	if err := r.resolve(); err != nil {
		return fmt.Errorf("failed to resolve conflicts: %w", err)
	}

	if _, err := r.commitCharts(fmt.Sprintf("bringing charts to %s", GetRelaventUpstreamChange(upstream))); err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}

	return nil
}

func (r *Rebase) updatePatches(upstream puller.Puller) (plumbing.Hash, error) {
	r.Logger.Info("generating patch")

	if err := r.Package.GeneratePatch(); err != nil {
		return plumbing.ZeroHash, fmt.Errorf("failed to generate patch: %w", err)
	}

	patchDir := path.Join("packages", r.Package.Name, "generated-changes")

	hash, err := Commit(r.chartsWt, true, fmt.Sprintf("Updating %s to new base %s", r.Package.Name, GetRelaventUpstreamChange(upstream)), patchDir)
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("failed to commit patch changes: %w", err)
	}

	if err := r.chartsWt.Reset(&git.ResetOptions{Mode: git.HardReset}); err != nil {
		return plumbing.ZeroHash, fmt.Errorf("failed to revert changes to chart")
	}

	return hash, nil
}

func (r *Rebase) updatePackageYaml(upstream puller.Puller) (plumbing.Hash, error) {
	r.Logger.Info("updating package.yaml")

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

	r.Logger.Info("updating package.yaml", "expr", expression)

	allAtOnceEvaluator := yqlib.NewAllAtOnceEvaluator()
	if err := allAtOnceEvaluator.EvaluateFiles(expression, []string{pkgFile}, printer, decoder); err != nil {
		return plumbing.ZeroHash, fmt.Errorf("failed to update package.yaml: %w", err)
	}

	if err := inPlaceHandler.FinishWriteInPlace(true); err != nil {
		return plumbing.ZeroHash, fmt.Errorf("failed to complete in-place update: %w", err)
	}

	hash, err := Commit(r.chartsWt, true, "Update package.yaml", relativePkgPath)
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("failed to commit package.yaml changes: %w", err)
	}

	return hash, nil
}

func (r *Rebase) reloadPackage() error {
	pkg, err := charts.GetPackage(r.RootFs, r.Package.Name)
	if err != nil {
		return fmt.Errorf("failed to get package: %w", err)
	}

	r.Package = pkg

	return nil
}

func (r *Rebase) Rebase() error {
	isClean, err := IsWorktreeClean(r.chartsWt)
	if err != nil {
		return fmt.Errorf("failed to check if worktree is clean: %w", err)
	}

	if !isClean {
		return fmt.Errorf("charts worktree is not clean")
	}

	cherryPickCommits := []string{}
	rebaseStart := time.Now()

	if err := CreateBranch(r.chartsRepo, ChartsQuarantineBranchName, plumbing.ZeroHash); err != nil {
		return fmt.Errorf("failed to create quarantine branch: %w", err)
	}
	defer DeleteBranch(r.chartsRepo, ChartsQuarantineBranchName)

	var backup Backup

	if r.EnableBackup {
		sources := make([]string, len(r.Package.AdditionalCharts)+1)
		sources[0] = filepath.Join(r.PkgFs.Root(), r.Package.WorkingDir)
		for i, chart := range r.Package.AdditionalCharts {
			sources[i+1] = filepath.Join(r.PkgFs.Root(), chart.WorkingDir)
		}

		backup, err = NewBackup(sources, RebaseBackupDir)
		if err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}
	} else {
		backup = &NoopBackup{}
	}
	defer func() {
		if err := backup.Close(); err != nil {
			r.Logger.Warn("failed to close backup", "err", err)
		}
	}()

	err = DoOnBranch(r.chartsRepo, r.chartsWt, ChartsQuarantineBranchName, func(wt *git.Worktree) error {
		r.Logger.Info("preparing package")

		if err := r.Package.Prepare(); err != nil {
			return fmt.Errorf("failed to prepare the chart: %w", err)
		}

		if _, err := r.commitCharts("preparing package"); err != nil {
			return fmt.Errorf("failed to save charts before pulling new upstream: %w", err)
		}

		var last puller.Puller

		err := iter.ForEach(r.Iter, func(p puller.Puller) error {
			defer func() {
				if err := backup.Backup(); err != nil {
					r.Logger.Warn("failed to backup charts", "err", err)
				}
			}()

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

		if _, err = r.updatePackageYaml(last); err != nil {
			return fmt.Errorf("failed to update package.yaml: %w", err)
		}

		if err := r.reloadPackage(); err != nil {
			return fmt.Errorf("failed to update package: %w", err)
		}

		if _, err = r.updatePatches(last); err != nil {
			return fmt.Errorf("failed to generate patch: %w", err)
		}

		commitIter, err := r.chartsRepo.Log(&git.LogOptions{Since: &rebaseStart})
		if err != nil {
			return fmt.Errorf("failed to get commit iterator: %w", err)
		}

		commitIter.ForEach(func(c *object.Commit) error {
			if c.Author.Name != "chartsutil-rebase" {
				cherryPickCommits = append(cherryPickCommits, c.Hash.String())
			}

			return nil
		})
		slices.Reverse(cherryPickCommits)

		return nil
	})
	if err != nil {
		return err
	}

	if len(cherryPickCommits) == 0 {
		return fmt.Errorf("no commits to cherry-pick")
	}

	// sleep via https://github.com/go-git/go-git/issues/37#issuecomment-1360057685
	r.Logger.Info("letting git catch up...")
	time.Sleep(time.Second * 2)

	r.Logger.Info("cherry picking from quarantine brach", "commits", cherryPickCommits)
	cherryPickArgs := append([]string{"cherry-pick", "--allow-empty"}, cherryPickCommits...)
	cmd := exec.Command("git", cherryPickArgs...)
	cmd.Dir = r.PkgFs.Root()

	out, err := cmd.CombinedOutput()
	if err != nil {
		switch err.(type) {
		case *exec.ExitError:
			fmt.Println(string(out))
			return fmt.Errorf("failed to cherry-pick changes: %w", err)
		default:
			return fmt.Errorf("could not cherry-pick changes: %w", err)
		}

	}

	return nil
}
