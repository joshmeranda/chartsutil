package resolve

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
)

type ResolveStrategy int
type section int

const (
	StrategyOurs ResolveStrategy = iota
	StrategyTheirs

	sectionNone section = iota
	sectionOurs
	sectionTheirs
)

type MergeResolver struct {
	Strategy ResolveStrategy
}

func (m MergeResolver) resolveFile(path string) error {
	tmp, err := os.CreateTemp("", "chartsutil-merge-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmp.Close()

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", path, err)
	}

	section := sectionNone

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "<<<<<<<"):
			section = sectionOurs
			continue
		case strings.HasPrefix(line, "======="):
			section = sectionNone
			continue
		case strings.HasPrefix(line, ">>>>>>>"):
			section = sectionTheirs
			continue
		}

		if section == sectionNone || (section == sectionOurs && m.Strategy == StrategyOurs) || (section == sectionTheirs && m.Strategy == StrategyTheirs) {
			if _, err := tmp.Write(append(scanner.Bytes(), '\n')); err != nil {
				return fmt.Errorf("failed to write to temp file: %w", err)
			}
		}
	}

	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("failed to update worktree with resolved content: %w", err)

	}

	return nil
}

func (m MergeResolver) handleFile(wt *git.Worktree, file string, info *git.FileStatus) error {
	if info.Worktree == git.Modified {
		if err := m.resolveFile(filepath.Join(wt.Filesystem.Root(), file)); err != nil {
			return fmt.Errorf("failed to resolve file %s: %w", file, err)
		}
	}

	if _, err := wt.Add(file); err != nil {
		return fmt.Errorf("failed to stage file %s: %w", file, err)
	}

	return nil
}

func (m MergeResolver) Resolve(wt *git.Worktree) error {
	status, err := wt.Status()
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	for file, info := range status {
		if err := m.handleFile(wt, file, info); err != nil {
			return fmt.Errorf("failed to handle file %s: %w", file, err)
		}
	}

	return nil
}
