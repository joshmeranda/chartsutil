package rebase

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	cp "github.com/otiai10/copy"
)

var (
	cpOptions = cp.Options{
		OnDirExists: func(src string, dest string) cp.DirExistsAction {
			return cp.Replace
		},
	}
)

type Backup interface {
	io.Closer

	Backup() error
}

type NoopBackup struct{}

func (b *NoopBackup) Close() error {
	return nil
}

func (b *NoopBackup) Backup() error {
	return nil
}

type FilesystemBackup struct {
	Sources     []string
	Destination string

	tmp string
}

func NewBackup(sources []string, destination string) (Backup, error) {
	tmp, err := os.MkdirTemp("", "rebase-backup-*")
	if err != nil {
		return nil, fmt.Errorf("could not create temp dir for backup: %w", err)
	}

	return &FilesystemBackup{
		Sources:     sources,
		Destination: destination,

		tmp: tmp,
	}, nil
}

func (b *FilesystemBackup) Close() error {
	if err := os.RemoveAll(b.Destination); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to remove existing destination: %w", err)
	}

	if err := os.Rename(b.tmp, b.Destination); err != nil {
		return fmt.Errorf("failed to move temp to destination: %w", err)
	}

	return nil
}

func (b *FilesystemBackup) Backup() error {
	for _, src := range b.Sources {
		dst := filepath.Join(b.tmp, filepath.Base(src))

		if err := cp.Copy(src, dst, cpOptions); err != nil {
			return fmt.Errorf("failed to commit to filesystem: %w", err)
		}
	}

	return nil
}
