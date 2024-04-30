package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"github.com/joshmeranda/chartsutil/pkg/display"
	utilpuller "github.com/joshmeranda/chartsutil/pkg/puller"
	"github.com/joshmeranda/chartsutil/pkg/rebase"
	"github.com/joshmeranda/chartsutil/pkg/release"
	"github.com/rancher/charts-build-scripts/pkg/charts"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	chartspath "github.com/rancher/charts-build-scripts/pkg/path"
	"github.com/urfave/cli/v2"
)

const (
	DefaultPackageEnv = "PACKAGE"
)

var (
	logger *slog.Logger
)

func pkgRebase(ctx *cli.Context) error {
	pkgName := ctx.String("package")
	chartsDir := ctx.String("charts-dir")
	incremental := ctx.Bool("increment")
	backup := ctx.Bool("backup")

	if ctx.NArg() != 1 {
		return fmt.Errorf("expected exactly one argument, got %d", ctx.NArg())
	}

	rebaseTarget := ctx.Args().First()

	rootFs := filesystem.GetFilesystem(chartsDir)
	pkgFs, err := rootFs.Chroot(filepath.Join(chartspath.RepositoryPackagesDir, pkgName))
	if err != nil {
		return fmt.Errorf("failed to chroot to package dir: %w", err)
	}

	gitRoot, err := os.MkdirTemp(os.TempDir(), "chart-utils-")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(gitRoot)

	pkg, err := charts.GetPackage(rootFs, pkgName)
	if err != nil {
		return err
	}

	var iter utilpuller.PullerIter

	if incremental {
		iter, err = utilpuller.IterForUpstream(pkg.Chart.Upstream, rebaseTarget)
		if err != nil {
			return fmt.Errorf("failed to create puller iterator: %w", err)
		}
	} else {
		iter, err = utilpuller.NewSingleIter(pkg.Chart.Upstream, rebaseTarget)
		if err != nil {
			return fmt.Errorf("failed to create single puller: %w", err)
		}
	}

	opts := rebase.Options{
		Logger:       logger,
		EnableBackup: backup,
	}

	rb, err := rebase.NewRebase(pkg, rootFs, pkgFs, iter, opts)
	if err != nil {
		return fmt.Errorf("invalid rebaser spec: %w", err)
	}

	logger.Info("attempting to rebase pacakge", "pkg", rb.Package.Name, "from", *pkg.Chart.Upstream.GetOptions().Commit, "to", rebaseTarget)

	if err := rb.Rebase(); err != nil {
		return err
	}

	return nil
}

// todo: add verbosity flags: verbose and quiet (only necessary output)

func rebaseCheck(ctx *cli.Context) error {
	pkgName := ctx.String("package")
	chartsDir := ctx.String("charts-dir")
	rootFs := filesystem.GetFilesystem(chartsDir)
	releaseNamePattern := ctx.String("release-pattern")
	releaseRegex, err := regexp.Compile(releaseNamePattern)
	if err != nil {
		return fmt.Errorf("failed to compile tag pattern: %w", err)
	}

	gitRoot, err := os.MkdirTemp(os.TempDir(), "chart-utils-")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(gitRoot)

	pkg, err := charts.GetPackage(rootFs, pkgName)
	if err != nil {
		return err
	}

	pullOpts := pkg.Chart.Upstream.GetOptions()

	if pullOpts.URL == "" {
		return fmt.Errorf("upstream URL is not set")
	}

	if !strings.HasSuffix(pullOpts.URL, ".git") {
		return fmt.Errorf("upstream URL '%s' is not a git repository", pullOpts.URL)
	}

	ref, err := release.RepoRefFromUrl(pullOpts.URL)
	if err != nil {
		return fmt.Errorf("failed to get upstream owner and name from url: %w", err)
	}

	var currentReleaseDate time.Time

	client := github.NewClient(nil)
	tag, _, err := client.Git.GetTag(ctx.Context, ref.Owner, ref.Name, *pullOpts.Commit)
	switch err {
	case nil:
		release, _, err := client.Repositories.GetReleaseByTag(ctx.Context, ref.Owner, ref.Name, *tag.Tag)
		if err != nil {
			return fmt.Errorf("failed to fetch release for tag: %w", err)
		}

		currentReleaseDate = release.CreatedAt.Time
	default:
		logger.Warn("failed to fetch tag for current commit, using commti date", "err", err, "commit", *pullOpts.Commit)

		commit, _, err := client.Git.GetCommit(ctx.Context, ref.Owner, ref.Name, *pullOpts.Commit)
		if err != nil {
			return fmt.Errorf("failed to fetch commit for hash: %w", err)
		}

		currentReleaseDate = *commit.Committer.Date
	}

	query := release.ReleaseQuery{
		Since:       currentReleaseDate,
		NamePattern: releaseRegex,
	}

	releases, err := release.ReleasesForUpstream(ctx.Context, ref, query)
	if err != nil {
		return fmt.Errorf("failed to list upstream releases: %w", err)
	}

	table := display.NewTable("Name", "Age", "Hash")
	for _, release := range releases {
		age := display.NewDuration(release.Age).Round()
		table.AddRow(release.Name, age.String(), release.Hash)
	}

	fmt.Print(table.String())

	return nil
}

func main() {
	logger = slog.New(slog.NewTextHandler(os.Stdout, nil))

	app := cli.App{
		Name: "chart-utils",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "charts-dir",
				Value: ".",
			},
			&cli.StringFlag{
				Name:     "package",
				EnvVars:  []string{DefaultPackageEnv},
				Required: true,
			},
		},
		Commands: []*cli.Command{
			{
				Name: "upstream",
				Subcommands: []*cli.Command{
					{
						Name:        "check",
						Description: "check the chart upstream for newer versions of the base chart",
						Action:      rebaseCheck,
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "release-pattern",
								Usage:   "regex pattern to match release names",
								Aliases: []string{"p"},
								Value:   release.DefaultReleaseNamePattern,
							},
						},
					},
				},
			},
			{
				Name:      "rebase",
				Action:    pkgRebase,
				Usage:     "Rebase a chart to a new version of the base chart",
				UsageText: "chart-utils rebase [options] <commit|url>",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "increment",
						Usage: "iterate through intermediary versions until the target upstream is achieved (only meaningful fr giuthub upstreams)",
					},
					&cli.BoolFlag{
						Name:  "backup",
						Usage: "create a backup of the package working dir after each upstream is merged",
					},
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}
