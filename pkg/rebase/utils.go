package rebase

import (
	"os"
	"path/filepath"
)

func ToPtr[T any](t T) *T {
	return &t
}

func shouldSkip(srcinfo os.FileInfo, src, dest string) (bool, error) {
	if filepath.Base(src) == ".git" {
		return true, nil
	}

	return false, nil
}
