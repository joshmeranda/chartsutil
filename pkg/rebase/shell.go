package rebase

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/rancher/charts-build-scripts/pkg/filesystem"
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
	// todo: maybe add commit to prompt
	return []byte(fmt.Sprintf(`PS1="(interactive-rebase-shell)> "; alias abort='touch %s && exit'; echo '%s'`, AbortFileName, ShellWelcomeMessage))
}

func (r *Rebase) shouldAbort() bool {
	_, err := r.RootFs.Stat(AbortFileName)
	return err == nil
}

func (r *Rebase) checkWorktree() (string, error) {
	status, err := r.chartsWt.Status()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree status: %w", err)
	}

	generatedChangesDir, err := filesystem.GetRelativePath(r.RootFs, filepath.Join(r.PkgFs.Root(), "generated-changes"))
	if err != nil {
		return "", fmt.Errorf("failed to get relative path to generated-changes: %w", err)
	}

	chartsDir, err := filesystem.GetRelativePath(r.RootFs, filepath.Join(r.PkgFs.Root(), r.Package.WorkingDir))
	if err != nil {
		return "", fmt.Errorf("failed to get relative path to charts dir: %w", err)
	}

	for file, fs := range status {
		if fs.Worktree != git.Unmodified {
			return "there are unstaged changes in the worktree", nil
		}

		if !strings.HasPrefix(file, generatedChangesDir) && !strings.HasPrefix(file, chartsDir) {
			return fmt.Sprintf("only changes to %s and %s are allowed", generatedChangesDir, chartsDir), nil
		}
	}

	return "", nil
}

func (r *Rebase) RunShell() error {
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
		cmd.Dir = r.RootFs.Root()
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return err
		}

		if r.shouldAbort() {
			if err := r.RootFs.Remove(AbortFileName); err != nil {
				r.Logger.Error("failed to remove abort file: %w", err)
			}

			return fmt.Errorf("rebase aborted by user")
		}

		msg, err := r.checkWorktree()
		if err != nil {
			return fmt.Errorf("failed to check if worktree is clean: %w", err)
		}

		if msg == "" {
			break
		}

		r.Logger.Error("worktree failed pre-commit checks", "msg", msg)
		r.Logger.Warn("re-running shell...")
	}

	return nil
}
