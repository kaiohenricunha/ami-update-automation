package vcs_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaiohenricunha/ami-update-automation/internal/vcs"
)

func initBareRepo(t *testing.T) string {
	t.Helper()
	bare := t.TempDir()
	runGit(t, bare, "init", "--bare")
	// Create a non-bare clone, add a commit, then push to bare.
	work := t.TempDir()
	runGit(t, work, "init")
	runGit(t, work, "remote", "add", "origin", bare)
	require.NoError(t, os.WriteFile(filepath.Join(work, "README.md"), []byte("hello"), 0o600))
	runGit(t, work, "-c", "user.email=test@test.com", "-c", "user.name=Test", "add", "-A")
	runGit(t, work, "-c", "user.email=test@test.com", "-c", "user.name=Test", "commit", "-m", "init")
	runGit(t, work, "push", "origin", "HEAD:main")
	return bare
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := gitCmd(dir, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("git %v output: %s", args, out)
	}
	require.NoError(t, err)
}

func TestCreateBranch(t *testing.T) {
	work := t.TempDir()
	runGit(t, work, "init")
	runGit(t, work, "-c", "user.email=t@t.com", "-c", "user.name=T", "commit", "--allow-empty", "-m", "init")

	err := vcs.CreateBranch(context.Background(), work, "feature/test-branch")
	require.NoError(t, err)
}

func TestCreateBranchInvalid(t *testing.T) {
	err := vcs.CreateBranch(context.Background(), "/tmp", "branch name with spaces")
	require.Error(t, err)
}

func TestCommit(t *testing.T) {
	work := t.TempDir()
	runGit(t, work, "init")
	require.NoError(t, os.WriteFile(filepath.Join(work, "file.tf"), []byte("v1"), 0o600))
	runGit(t, work, "-c", "user.email=t@t.com", "-c", "user.name=T", "add", "-A")
	err := vcs.Commit(context.Background(), work, "chore: update AMI version")
	require.NoError(t, err)
}

var _ = assert.Equal // avoid unused import if assert is only used indirectly
