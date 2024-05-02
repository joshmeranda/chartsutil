package rebase

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

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

type VerifyFunc func(string) error

func VerifyHelmLint(chartPath string) error {
	client := action.NewLint()
	result := client.Run([]string{chartPath}, map[string]interface{}{})

	if len(result.Errors) > 0 {
		return &ValidateError{
			chart: chartPath,
			inner: fmt.Errorf("encountered lint errors: %w", errors.Join(result.Errors...)),
		}
	}

	return nil
}

func VerifyPatternNotFoundFactory(pattern string) VerifyFunc {
	return func(chartPath string) error {
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
	}
}
