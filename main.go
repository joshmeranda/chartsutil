package main

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/go-github/github"
	chartsutil "github.com/joshmeranda/chartsutil/pkg"
	"github.com/joshmeranda/chartsutil/pkg/display"
	"github.com/joshmeranda/chartsutil/pkg/rebase"
	"github.com/joshmeranda/chartsutil/pkg/release"
	"github.com/rancher/charts-build-scripts/pkg/charts"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
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
	rootFs := filesystem.GetFilesystem(chartsDir)

	gitRoot, err := os.MkdirTemp(os.TempDir(), "chart-utils-")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(gitRoot)

	pkg, err := charts.GetPackage(rootFs, pkgName)
	if err != nil {
		return err
	}

	opts := rebase.Options{
		Logger: logger,
	}

	rb, err := rebase.NewRebase(pkg, "3173f0f0b3c040ac75617c5e8497dc479afc11f0", opts)
	if err != nil {
		return fmt.Errorf("invalid rebaser spec: %w", err)
	}
	defer rb.Close()

	logger.Info("attempting to rebase pacakge", "pkg", rb.Package.Name, "from", *pkg.Chart.Upstream.GetOptions().Commit, "to", rb.ToCommit)

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

	ref, err := chartsutil.RepoRefFromUrl(pullOpts.URL)
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
				Name:        "rebase",
				Action:      pkgRebase,
				Description: "Rebase a chart to a new version of the base chart",
				Subcommands: []*cli.Command{
					{
						Name:        "check",
						Description: "check the chart upstream for newer versions of the base chart",
						Action:      rebaseCheck,
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "release-pattern",
								Aliases: []string{"p"},
								Value:   release.DefaultReleaseNamePattern,
							},
						},
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