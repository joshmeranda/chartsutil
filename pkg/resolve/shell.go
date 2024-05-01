package resolve

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/rancher/charts-build-scripts/pkg/charts"
	"github.com/rancher/charts-build-scripts/pkg/path"
)

const (
	ShellWelcomeMessage = `Welcome to the charts interactive rebase shell!
< = = = = = = = = = = >

The changes from the current upstream has been loaded into the current branch.
Please look through the changed files to validate those changes and resolve conflicts.
Once the index is in the desired state add all changes and run 'exit'!

To abort the rebase at any time run 'abort'!`

	AbortFileName = ".abort_rebase"
)

func getShellRcContents() []byte {
	return []byte(fmt.Sprintf(`PS1="(interactive-rebase-shell)> "; alias abort='touch %s && exit'; echo '%s'`, AbortFileName, ShellWelcomeMessage))
}

type Shell struct {
	Logger  *slog.Logger
	Package *charts.Package
}

func (s *Shell) shouldAbort(fs billy.Filesystem) bool {
	_, err := fs.Stat(AbortFileName)
	return err == nil
}

func (s *Shell) checkWorktree(wt *git.Worktree) (string, error) {
	status, err := wt.Status()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree status: %w", err)
	}

	pkgDir := filepath.Join(path.RepositoryPackagesDir, s.Package.Name)

	allowedPaths := []string{
		filepath.Join(pkgDir, path.GeneratedChangesDir),
		filepath.Join(pkgDir, s.Package.WorkingDir),
	}

	for file, fs := range status {
		if fs.Worktree != git.Unmodified {
			return "there are unstaged changes in the worktree", nil
		}

		isFileAllowed := Any(allowedPaths, func(p string) bool {
			return strings.HasPrefix(file, p)
		})

		if !isFileAllowed {
			return "only changes to generated-changes or chart working dir are allowed", nil
		}
	}

	return "", nil
}

func (s *Shell) Resolve(wt *git.Worktree) error {
	f, err := os.CreateTemp("", "rebase-shell-rc-*")
	if err != nil {
		return fmt.Errorf("failed to create shell rc file: %w", err)
	}

	if _, err := f.Write(getShellRcContents()); err != nil {
		return fmt.Errorf("failed to write to shell rc file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close shell rc file: %w", err)
	}
	defer os.Remove(f.Name())

	for {
		cmd := exec.Command("bash", "--rcfile", f.Name(), "-i")
		cmd.Dir = wt.Filesystem.Root()
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Run()
		_, isExitErr := err.(*exec.ExitError)
		if err != nil && !isExitErr {
			return err
		}

		if s.shouldAbort(wt.Filesystem) {
			if err := wt.Filesystem.Remove(AbortFileName); err != nil {
				s.Logger.Error("failed to remove abort file: %w", err)
			}

			return ErrAbort
		}

		msg, err := s.checkWorktree(wt)
		if err != nil {
			return fmt.Errorf("failed to check if worktree is clean: %w", err)
		}

		if msg == "" {
			break
		}

		s.Logger.Error("worktree failed pre-commit checks", "msg", msg)
		s.Logger.Warn("re-running shell...")
	}

	return nil
}
