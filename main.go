package main

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	chartsutil "github.com/joshmeranda/chartsutil/pkg"
	"github.com/joshmeranda/chartsutil/pkg/display"
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

func rebase(ctx *cli.Context) error {
	return nil
}

// todo: add verbosity flags: verbose and quiet (only necessary output)

func rebaseCheck(ctx *cli.Context) error {
	pkgName := ctx.String("package")
	chartsDir := ctx.String("charts-dir")
	rootFs := filesystem.GetFilesystem(chartsDir)
	tagPattern := ctx.String("tag-pattern")
	tagRegex, err := regexp.Compile(tagPattern)
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

	if pullOpts.Commit == nil {
		return fmt.Errorf("upstream commit is not set")
	}

	logger.Info("cloning upstream", "url", pullOpts.URL)
	cloneOpts := &git.CloneOptions{
		URL: pullOpts.URL,
		// todo: add support for a single branch pull (more speed == more better)
		// ReferenceName: "refs/heads/main",
		// SingleBranch:  true,
		SingleBranch: false,
		Progress:     os.Stdin,
		Tags:         git.AllTags,
	}
	repo, err := git.PlainClone(gitRoot, false, cloneOpts)
	if err != nil {
		return fmt.Errorf("failed to clone upstream repository: %w", err)

	}

	logger.Info("checking upstream for newer tags for package", "pkg", pkgName)
	currentCommitHash := plumbing.NewHash(*pullOpts.Commit)
	currentCommit, err := repo.CommitObject(currentCommitHash)
	if err != nil {
		return fmt.Errorf("failed to get current commit: %w", err)
	}

	tagQuery := chartsutil.TagQuery{
		Since:     currentCommit.Committer.When.Local(),
		TagFilter: *tagRegex,
	}

	tags, err := chartsutil.TagsFromRepo(repo, tagQuery)
	if err != nil {
		return fmt.Errorf("failed to fetch tags with query: %w", err)
	}

	table := display.NewTable("Tag", "Age", "Hash")
	for _, tag := range tags {
		table.AddRow(tag.Name, tag.Age.String(), tag.Hash)
	}

	fmt.Println(table)

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
				Action:      rebase,
				Description: "Rebase a chart to a new version of the base chart",
				Subcommands: []*cli.Command{
					{
						Name:        "check",
						Description: "check the chart upstream for newer versions of the base chart",
						Action:      rebaseCheck,
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "tag-pattern",
								Aliases: []string{"p"},
								Value:   chartsutil.DefaultTagPattern,
							},
						},
					},
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		// if err := app.Run([]string{"chart-utils", "--charts-dir", "/home/wrinkle/workspaces/rancher/charts", "--package", "rancher-logging", "rebase", "check"}); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}
