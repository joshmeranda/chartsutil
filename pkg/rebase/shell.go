package rebase

import (
	"fmt"
	"os"
	"os/exec"
	"path"
)

const (
	// todo: maybe add commit to prompt
	RC_CONTENTS = `PS1="(interactive-rebase-shell)> "; alias abort='touch .abort_rebase && exit'`
)

func (r *Rebase) shouldAbort() bool {
	_, err := os.Stat(r.ChartsDir + "/.abort_rebase")
	return err == nil
}

func (r *Rebase) RunShell() error {
	f, err := os.CreateTemp("", "rebase-shell-rc-*")
	if err != nil {
		return fmt.Errorf("failed to create shell rc file: %w", err)
	}

	if _, err := f.Write([]byte(RC_CONTENTS)); err != nil {
		return fmt.Errorf("failed to write to shell rc file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close shell rc file: %w", err)
	}
	defer os.Remove(f.Name())

	for {
		cmd := exec.Command("bash", "--rcfile", f.Name(), "-i")
		cmd.Dir = r.ChartsDir
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return err
		}

		if r.shouldAbort() {
			if err := os.Remove(path.Join(r.ChartsDir, ".abort_rebase")); err != nil {
				r.Logger.Error("failed to remove '.abort_rebase' file: %w", err)
			}

			return fmt.Errorf("rebase aborted by user")
		}

		isClean, err := IsWorktreeClean(r.chartsWt)
		if err != nil {
			return fmt.Errorf("failed to check if worktree is clean: %w", err)
		}

		if isClean {
			break
		}

		r.Logger.Warn("worktree is not clean, re-running shell")
	}

	return nil
}
