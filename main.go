package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"github.com/joshmeranda/chartsutil/pkg/display"
	"github.com/joshmeranda/chartsutil/pkg/images"
	"github.com/joshmeranda/chartsutil/pkg/iter"
	"github.com/joshmeranda/chartsutil/pkg/rebase"
	"github.com/joshmeranda/chartsutil/pkg/release"
	"github.com/rancher/charts-build-scripts/pkg/charts"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	chartspath "github.com/rancher/charts-build-scripts/pkg/path"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

const (
	EnvPackage   = "PACKAGE"
	EnvChartsDir = "CHARTS_DIR"

	CategoryPatternMatching = "Pattern Matching"
	CategoryVerbosity       = "Verbosity"
	CategoryUpstreamSpec    = "Upstream Specifications"

	ImageMirrorFileUrl = "https://raw.githubusercontent.com/rancher/image-mirror/master/images-list"
)

var (
	logger *slog.Logger

	Version = "v0.1.0"
)

func setup(ctx *cli.Context) error {
	if !ctx.Bool("show-charts-logs") {
		logrus.SetOutput(io.Discard)
	}

	var out io.Writer

	out = os.Stdout
	opts := &slog.HandlerOptions{}

	switch {
	case ctx.Bool("silent"):
		out = io.Discard
	case ctx.Bool("verbose"):
		out = os.Stdout
		opts.Level = slog.LevelDebug
	}

	logger = slog.New(slog.NewTextHandler(out, opts))

	return nil
}

func pkgRebase(ctx *cli.Context) error {
	pkgName := ctx.String("package")
	chartsDir := ctx.String("charts-dir")
	incremental := ctx.Bool("increment")
	backup := ctx.Bool("backup")

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

	delta := iter.UpstreamDelta{}

	if ctx.IsSet("commit") {
		delta.Commit = rebase.ToPtr(ctx.String("commit"))
	}

	if ctx.IsSet("url") {
		delta.URL = ctx.String("url")
	}

	if ctx.IsSet("subdirectory") {
		delta.Subdirectory = rebase.ToPtr(ctx.String("subdirectory"))
	}

	var upstreamIter iter.UpstreamIter

	if incremental {
		upstreamIter, err = iter.IterForUpstream(pkg.Chart.Upstream, delta)
		if err != nil {
			return fmt.Errorf("failed to create puller iterator: %w", err)
		}
	} else {
		upstreamIter, err = iter.NewSingleIter(pkg.Chart.Upstream, delta)
		if err != nil {
			return fmt.Errorf("failed to create single puller: %w", err)
		}
	}

	opts := rebase.Options{
		Logger:       logger,
		EnableBackup: backup,
	}

	rb, err := rebase.NewRebase(pkg, rootFs, pkgFs, upstreamIter, opts)
	if err != nil {
		return fmt.Errorf("invalid rebaser spec: %w", err)
	}

	newOpts, err := delta.Apply(pkg.Chart.Upstream.GetOptions())
	if err != nil {
		return fmt.Errorf("failed to apply upstream delta: %w", err)
	}

	logger.Info("attempting to rebase pacakge",
		"pkg", rb.Package.Name,
		"from", rebase.UpstreamRef(pkg.Chart.Upstream.GetOptions()),
		"to", rebase.UpstreamRef(newOpts),
		"incremental", incremental,
	)

	if err := rb.Rebase(); err != nil {
		return err
	}

	return nil
}

func upstreamCheck(ctx *cli.Context) error {
	pkgName := ctx.String("package")
	chartsDir := ctx.String("charts-dir")
	rootFs := filesystem.GetFilesystem(chartsDir)

	releaseNamePattern := ctx.String("prefix") + ctx.String("pattern") + ctx.String("postfix")
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
		logger.Warn("failed to fetch tag for current commit, using commit date", "err", err, "commit", *pullOpts.Commit)

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

	logger.Info("checking for upstream releases", "query", query)
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

func imagesMirror(ctx *cli.Context) error {
	pkgName := ctx.String("package")
	chartsDir := ctx.String("charts-dir")
	rootFs := filesystem.GetFilesystem(chartsDir)
	imagesListUrl := ctx.String("images-list")

	pkg, err := charts.GetPackage(rootFs, pkgName)
	if err != nil {
		return err
	}

	pkgFs, err := rootFs.Chroot(filepath.Join(chartspath.RepositoryPackagesDir, pkgName))
	if err != nil {
		return fmt.Errorf("failed to chroot to package dir: %w", err)
	}

	chartPath := filepath.Join(pkgFs.Root(), pkg.WorkingDir)

	imageMap, err := images.GetImagesFromChart(chartPath)
	if err != nil {
		return err
	}

	resp, err := http.Get(imagesListUrl)
	if err != nil {
		return fmt.Errorf("failed to fetch images list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch images list: %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read images list: %w", err)
	}

	mirrors, err := images.UnmarshalImagesList(data)
	if err != nil {
		return fmt.Errorf("failed to unmarshal images list: %w", err)
	}

	newMirrors, err := images.GetMissingMirrorRefs("rancher", imageMap, mirrors)
	if err != nil {
		return fmt.Errorf("failed to get missing mirrors: %w", err)
	}

	for destination, sourceRef := range newMirrors {
		for _, tag := range sourceRef.Tags {
			fmt.Printf("%s %s %s\n", sourceRef.Source, destination, tag)
		}
	}

	return nil
}

func main() {
	app := cli.App{
		Name:    "chart-utils",
		Version: Version,
		Before:  setup,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "charts-dir",
				Usage:   "Working directory for chart operations",
				Value:   ".",
				EnvVars: []string{EnvChartsDir},
			},
			&cli.StringFlag{
				Name:     "package",
				Usage:    "The target package for chartsutils operations",
				EnvVars:  []string{EnvPackage},
				Required: true,
			},

			&cli.BoolFlag{
				Name:     "show-charts-logs",
				Category: CategoryVerbosity,
			},
			&cli.BoolFlag{
				Name:     "silent",
				Category: CategoryVerbosity,
			},
		},
		Commands: []*cli.Command{
			{
				Name:        "upstream",
				Description: "commands for interacting with upstream repositories",
				Subcommands: []*cli.Command{
					{
						Name:        "check",
						Description: "check the chart upstream for newer versions of the base chart",
						Action:      upstreamCheck,
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "pattern",
								Usage:    "regex pattern to match release names",
								Value:    release.DefaultReleaseNamePattern,
								Category: CategoryPatternMatching,
							},
							&cli.StringFlag{
								Name:     "prefix",
								Usage:    "prefix to add to the release pattern",
								Category: CategoryPatternMatching,
							},
							&cli.StringFlag{
								Name:     "postfux",
								Aliases:  []string{"sufix"},
								Usage:    "postfix to add to the release pattern",
								Category: CategoryPatternMatching,
							},
						},
					},
				},
			},
			{
				Name:      "rebase",
				Action:    pkgRebase,
				Usage:     "Rebase a chart to a new version of the base chart",
				UsageText: "chart-utils rebase [options]",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "increment",
						Usage: "iterate through intermediary versions until the target upstream is achieved (only meaningful fr giuthub upstreams)",
					},
					&cli.BoolFlag{
						Name:  "backup",
						Usage: "create a backup of the package working dir after each upstream is merged",
					},
					&cli.BoolFlag{
						Name:  "no-validate",
						Usage: "do not run validators after resolving upstream changes",
					},
					&cli.StringFlag{
						Name:     "commit",
						Usage:    "the commit to rebase to",
						Category: CategoryUpstreamSpec,
					},
					&cli.StringFlag{
						Name:     "url",
						Usage:    "the URL of the upstream repository to rebase to",
						Category: CategoryUpstreamSpec,
					},
					&cli.StringFlag{
						Name:     "subdirectory",
						Usage:    "the subdirectory of the upstream repository to rebase to",
						Category: CategoryUpstreamSpec,
					},
				},
			},
			{
				Name: "images",
				Subcommands: []*cli.Command{
					{
						Name:   "mirror",
						Usage:  "Get a list of images from a package which cannot be found in rancher/image-mirror",
						Action: imagesMirror,
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "images-list",
								Usage: "URL to the images list file",
								Value: ImageMirrorFileUrl,
							},
						},
					},
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		os.Exit(1)
	}
}
