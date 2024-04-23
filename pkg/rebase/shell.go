package rebase

import (
	"os"
	"os/exec"
)

const (
	// todo: maybe add commit to prompt
	RC_CONTENTS = `PS1="(interactive-rebase-shell) "; alias abort='touch .abort_rebase'`
)

func (r *Rebase) shell() error {
	// todo: create rc-file
	// todo: launch shell
	cmd := exec.Command("sh", "-i")

	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "PS1=(interactive-rebase-shell)")

	cmd.Dir = r.ChartsDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}
