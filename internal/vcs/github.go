package vcs

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/google/go-github/v60/github"
	"golang.org/x/oauth2"

	"github.com/kaiohenricunha/ami-update-automation/internal/sanitize"
	"github.com/kaiohenricunha/ami-update-automation/pkg/types"
)

const defaultGitHubAPIURL = "https://api.github.com/"
const defaultGitHubCloneBase = "https://github.com"

// GitHubProvider implements VCSProvider using GitHub API and git CLI.
type GitHubProvider struct {
	apiURL    string
	cloneBase string
}

// NewGitHubProvider creates a GitHubProvider.
// apiURL defaults to https://api.github.com/ for GitHub.com.
func NewGitHubProvider(apiURL string) *GitHubProvider {
	if apiURL == "" {
		apiURL = defaultGitHubAPIURL
	}
	// Derive clone base from API URL.
	cloneBase := defaultGitHubCloneBase
	if apiURL != defaultGitHubAPIURL {
		u, err := url.Parse(apiURL)
		if err == nil {
			cloneBase = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
		}
	}
	return &GitHubProvider{apiURL: apiURL, cloneBase: cloneBase}
}

// newClient creates an authenticated GitHub client.
func (p *GitHubProvider) newClient(ctx context.Context, token string) (*github.Client, error) {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	httpClient := oauth2.NewClient(ctx, ts)

	if p.apiURL == defaultGitHubAPIURL {
		return github.NewClient(httpClient), nil
	}
	client, err := github.NewClient(httpClient).WithEnterpriseURLs(p.apiURL, p.apiURL)
	if err != nil {
		return nil, fmt.Errorf("creating GitHub enterprise client: %w", err)
	}
	return client, nil
}

// Clone shallow-clones the repo and returns the directory.
func (p *GitHubProvider) Clone(ctx context.Context, owner, repo, token string) (string, error) {
	dir, err := ShallowClone(ctx, owner, repo, p.cloneBase, token)
	if err != nil {
		return "", fmt.Errorf("%w: %w", types.ErrRepoCloneFailed, err)
	}
	return dir, nil
}

// CreateBranch creates and checks out a new branch.
func (p *GitHubProvider) CreateBranch(ctx context.Context, repoDir, branchName string) error {
	return CreateBranch(ctx, repoDir, branchName)
}

// CommitAndPush stages, commits, and pushes to origin.
func (p *GitHubProvider) CommitAndPush(ctx context.Context, repoDir, branchName, msg, token string) error {
	if err := Add(ctx, repoDir); err != nil {
		return err
	}
	if err := Commit(ctx, repoDir, msg); err != nil {
		return err
	}
	return Push(ctx, repoDir, branchName, token)
}

// CreatePR opens a pull request via the GitHub API.
func (p *GitHubProvider) CreatePR(ctx context.Context, req types.PRRequest, token string) (*types.PRResult, error) {
	if err := sanitize.ValidateOwner(req.Owner); err != nil {
		return nil, err
	}
	if err := sanitize.ValidateRepoName(req.Repo); err != nil {
		return nil, err
	}

	title := sanitize.SanitizePRContent(req.Title, 256)
	body := sanitize.SanitizePRContent(req.Body, 65536)

	client, err := p.newClient(ctx, token)
	if err != nil {
		return nil, err
	}

	pr, resp, err := client.PullRequests.Create(ctx, req.Owner, req.Repo, &github.NewPullRequest{
		Title: &title,
		Body:  &body,
		Head:  &req.Head,
		Base:  &req.Base,
		Draft: &req.Draft,
	})
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnprocessableEntity {
			return nil, fmt.Errorf("%w: %w", types.ErrPRAlreadyExists, err)
		}
		return nil, fmt.Errorf("creating PR: %w", err)
	}
	return &types.PRResult{
		URL:    pr.GetHTMLURL(),
		Number: pr.GetNumber(),
	}, nil
}

// PRExists checks if an open PR already exists for the given head branch.
func (p *GitHubProvider) PRExists(ctx context.Context, owner, repo, headBranch, token string) (bool, error) {
	client, err := p.newClient(ctx, token)
	if err != nil {
		return false, err
	}
	prs, _, err := client.PullRequests.List(ctx, owner, repo, &github.PullRequestListOptions{
		State: "open",
		Head:  owner + ":" + headBranch,
	})
	if err != nil {
		return false, fmt.Errorf("listing PRs: %w", err)
	}
	return len(prs) > 0, nil
}

// Cleanup removes the cloned repository directory.
func (p *GitHubProvider) Cleanup(repoDir string) error {
	return os.RemoveAll(repoDir)
}
