package rebase

import (
	"os"
	"path/filepath"
)

func ToPtr[T any](t T) *T {
	return &t
}
