package vcs_test

import (
	"os/exec"
)

func gitCmd(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...) //nolint:gosec
	cmd.Dir = dir
	return cmd
}
