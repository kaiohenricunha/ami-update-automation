// Package vcs defines the VCSProvider interface and Git/GitHub implementations.
package vcs

import (
	"context"

	"github.com/kaiohenricunha/ami-update-automation/pkg/types"
)

// VCSProvider abstracts version-control operations needed by the handler.
type VCSProvider interface {
	// Clone shallow-clones the repository to a temp directory and returns the path.
	Clone(ctx context.Context, owner, repo, token string) (repoDir string, err error)
	// CreateBranch creates and checks out a new branch in repoDir.
	CreateBranch(ctx context.Context, repoDir, branchName string) error
	// CommitAndPush stages all changes, commits with msg, and pushes to remote.
	CommitAndPush(ctx context.Context, repoDir, branchName, msg, token string) error
	// CreatePR opens a pull request and returns the result.
	CreatePR(ctx context.Context, req types.PRRequest, token string) (*types.PRResult, error)
	// PRExists checks whether an open PR with the given head branch already exists.
	PRExists(ctx context.Context, owner, repo, headBranch, token string) (bool, error)
	// Cleanup removes the cloned repository directory.
	Cleanup(repoDir string) error
}
