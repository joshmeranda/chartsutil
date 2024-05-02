package rebase

import (
	"errors"
	"fmt"

	"helm.sh/helm/v3/pkg/action"
)

type VerifyFunc func(string) error

func VerifyHelmLint(chartPath string) error {
	client := action.NewLint()
	result := client.Run([]string{chartPath}, map[string]interface{}{})

	if len(result.Errors) > 0 {
		return fmt.Errorf("encountered lint errors: %w", errors.Join(result.Errors...))
	}

	return nil
}
