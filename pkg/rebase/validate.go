package rebase

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/joshmeranda/chartsutil/pkg/images"
	"github.com/rancher/charts-build-scripts/pkg/charts"
	chartspath "github.com/rancher/charts-build-scripts/pkg/path"
	"helm.sh/helm/v3/pkg/action"
)

type ValidateError struct {
	chart string
	inner error
}

func (e ValidateError) Error() string {
	return fmt.Sprintf("chart at '%s' failed validation: %s", e.chart, e.inner.Error())
}

func (e ValidateError) Is(err error) bool {
	_, ok := err.(ValidateError)
	return ok
}

// PackageValidateFunc is a function that verifies a package using the provided filesystem.
type PackageValidateFunc func(*charts.Package, *git.Worktree, billy.Filesystem) error

// ChartValidateFunc is a function that verifies a chart using the provided filesystem.
type ChartValidateFunc func(string) error

func ForEachChart(pkg *charts.Package, pkgFs billy.Filesystem, fn ChartValidateFunc) error {
	if err := fn(filepath.Join(pkgFs.Root(), pkg.WorkingDir)); err != nil {
		return err
	}

	for _, ac := range pkg.AdditionalCharts {
		if err := fn(filepath.Join(pkgFs.Root(), ac.WorkingDir)); err != nil {
			return nil
		}
	}

	return nil
}

func ValidateHelmLint(pkg *charts.Package, wt *git.Worktree, pkgFs billy.Filesystem) error {
	client := action.NewLint()

	err := ForEachChart(pkg, pkgFs, func(chartPath string) error {
		result := client.Run([]string{chartPath}, map[string]interface{}{})

		if len(result.Errors) > 0 {
			return &ValidateError{
				chart: chartPath,
				inner: fmt.Errorf("encountered lint errors: %w", errors.Join(result.Errors...)),
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func ValidatePatternNotFoundFactory(pattern string) PackageValidateFunc {
	return func(pkg *charts.Package, wt *git.Worktree, pkgFs billy.Filesystem) error {
		err := ForEachChart(pkg, pkgFs, func(chartPath string) error {
			err := filepath.WalkDir(chartPath, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}

				if d.IsDir() {
					return nil
				}

				file, err := os.Open(path)
				if err != nil {
					return err
				}
				defer file.Close()

				scanner := bufio.NewScanner(file)
				for scanner.Scan() {
					if strings.Contains(scanner.Text(), pattern) {
						return &ValidateError{
							chart: chartPath,
							inner: fmt.Errorf("found pattern '%s' in file '%s'", pattern, path),
						}
					}
				}

				return nil
			})

			if errors.Is(err, &ValidateError{}) {
				return err
			} else if err != nil {
				return fmt.Errorf("verification failed: %w", err)
			}

			return nil
		})

		if err != nil {
			return err
		}

		return nil
	}
}

func ValidateWorktree(pkg *charts.Package, wt *git.Worktree, _ billy.Filesystem) error {
	status, err := wt.Status()
	if err != nil {
		return fmt.Errorf("failed to get worktree status: %w", err)
	}

	pkgDir := filepath.Join(chartspath.RepositoryPackagesDir, pkg.Name)
	allowedPaths := []string{
		filepath.Join(pkgDir, pkg.WorkingDir),
	}

	for _, ac := range pkg.AdditionalCharts {
		allowedPaths = append(allowedPaths, filepath.Join(pkgDir, ac.WorkingDir))
	}

	for file, fs := range status {
		if fs.Worktree != git.Unmodified {
			return ValidateError{
				chart: pkgDir,
				inner: fmt.Errorf("worktree has unstaged changes"),
			}
		}

		isFileAllowed := Any(allowedPaths, func(p string) bool {
			return strings.HasPrefix(file, p)
		})

		if !isFileAllowed {
			return ValidateError{
				chart: pkgDir,
				inner: fmt.Errorf("only changes to <package>/generated-changes or chart working directory are allowed"),
			}
		}
	}

	return nil
}

func ValidateImagesInNamespaceFactory(namespace string) PackageValidateFunc {
	return func(pkg *charts.Package, wt *git.Worktree, pkgFs billy.Filesystem) error {
		return ForEachChart(pkg, pkgFs, func(chartPath string) error {
			imagesList, err := images.GetImagesFromChart(chartPath)
			if err != nil {
				return fmt.Errorf("failed to validate all image are within namespace")
			}

			for image := range imagesList {
				if !images.RepositoryInNamespace(image, namespace) {
					return &ValidateError{
						chart: chartPath,
						inner: fmt.Errorf("image '%s' is not in namespace '%s'", image, namespace),
					}
				}
			}

			return nil
		})
	}
}
