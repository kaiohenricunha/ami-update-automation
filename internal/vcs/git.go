package vcs

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kaiohenricunha/ami-update-automation/internal/sanitize"
)

// gitRun executes a git command in dir, returning combined output on error.
// Token is never passed as a CLI argument.
func gitRun(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // args are validated before call
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, buf.String())
	}
	return nil
}

// ShallowClone clones a repository using a GIT_ASKPASS helper so the token
// is never exposed on the command line or in error messages.
func ShallowClone(ctx context.Context, owner, repo, baseURL, token string) (string, error) {
	if err := sanitize.ValidateOwner(owner); err != nil {
		return "", err
	}
	if err := sanitize.ValidateRepoName(repo); err != nil {
		return "", err
	}

	dir, err := os.MkdirTemp("", "ami-clone-*")
	if err != nil {
		return "", fmt.Errorf("creating temp dir: %w", err)
	}

	// Write a GIT_ASKPASS script so the token never appears in process args.
	askpass, err := writeAskpassScript(dir, token)
	if err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}

	cloneURL := fmt.Sprintf("%s/%s/%s.git", strings.TrimRight(baseURL, "/"), owner, repo)

	cmd := exec.CommandContext(ctx, "git", "clone", "--depth=1", cloneURL, dir) //nolint:gosec
	cmd.Env = append(os.Environ(),
		"GIT_ASKPASS="+askpass,
		"GIT_TERMINAL_PROMPT=0",
	)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		_ = os.RemoveAll(dir)
		output := sanitize.RedactToken(buf.String(), token)
		return "", fmt.Errorf("git clone: %w: %s", err, output)
	}
	return dir, nil
}

// writeAskpassScript writes a shell script that echoes the token and returns its path.
func writeAskpassScript(dir, token string) (string, error) {
	script := fmt.Sprintf("#!/bin/sh\necho '%s'\n", strings.ReplaceAll(token, "'", "'\\''"))
	path := filepath.Join(dir, ".git-askpass.sh")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil { //nolint:gosec
		return "", fmt.Errorf("writing askpass: %w", err)
	}
	return path, nil
}

// CreateBranch creates and checks out a new branch.
func CreateBranch(ctx context.Context, repoDir, branchName string) error {
	if err := sanitize.ValidateBranchName(branchName); err != nil {
		return err
	}
	return gitRun(ctx, repoDir, "checkout", "-b", branchName)
}

// Add stages all changed files.
func Add(ctx context.Context, repoDir string) error {
	return gitRun(ctx, repoDir, "add", "-A")
}

// Commit creates a commit with the given message.
func Commit(ctx context.Context, repoDir, message string) error {
	safe := sanitize.SanitizePRContent(message, 256)
	return gitRun(ctx, repoDir,
		"-c", "user.email=ami-automation@localhost",
		"-c", "user.name=AMI Automation",
		"commit", "-m", safe,
	)
}

// Push pushes the branch to origin using the token via GIT_ASKPASS.
func Push(ctx context.Context, repoDir, branchName, token string) error {
	if err := sanitize.ValidateBranchName(branchName); err != nil {
		return err
	}
	askpass, err := writeAskpassScript(repoDir, token)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "git", "push", "origin", branchName) //nolint:gosec
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(),
		"GIT_ASKPASS="+askpass,
		"GIT_TERMINAL_PROMPT=0",
	)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		output := sanitize.RedactToken(buf.String(), token)
		return fmt.Errorf("git push: %w: %s", err, output)
	}
	return nil
}
