//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	amiresolver "github.com/kaiohenricunha/ami-update-automation/internal/ami"
	appconfig "github.com/kaiohenricunha/ami-update-automation/internal/config"
	"github.com/kaiohenricunha/ami-update-automation/internal/handler"
	"github.com/kaiohenricunha/ami-update-automation/internal/scanner"
	"github.com/kaiohenricunha/ami-update-automation/internal/vcs"
	"github.com/kaiohenricunha/ami-update-automation/pkg/types"
	. "github.com/kaiohenricunha/ami-update-automation/test/integration"
)

type staticSecrets struct{ token string }

func (s *staticSecrets) GetSecret(_ context.Context, _ string) (string, error) {
	return s.token, nil
}

// fileVCS clones from a local file:// bare repo but uses GitHubProvider for all other operations.
type fileVCS struct {
	bareURL string
	gh      *vcs.GitHubProvider
}

func (f *fileVCS) Clone(ctx context.Context, _, _, _ string) (string, error) {
	dir, err := os.MkdirTemp("", "integ-clone-*")
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth=1", f.bareURL, dir) //nolint:gosec
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(dir)
		return "", fmt.Errorf("file clone: %w: %s", err, out)
	}
	return dir, nil
}
func (f *fileVCS) CreateBranch(ctx context.Context, d, b string) error {
	return f.gh.CreateBranch(ctx, d, b)
}
func (f *fileVCS) CommitAndPush(ctx context.Context, d, b, m, t string) error {
	return f.gh.CommitAndPush(ctx, d, b, m, t)
}
func (f *fileVCS) CreatePR(ctx context.Context, r types.PRRequest, t string) (*types.PRResult, error) {
	return f.gh.CreatePR(ctx, r, t)
}
func (f *fileVCS) PRExists(ctx context.Context, o, r, b, t string) (bool, error) {
	return f.gh.PRExists(ctx, o, r, b, t)
}
func (f *fileVCS) Cleanup(d string) error { return os.RemoveAll(d) }

func makeIntegrationHandler(t *testing.T, fixtureDir string, ssmVersions map[string]string) (*handler.Handler, *[]RecordedRequest) {
	t.Helper()
	bareURL := SetupBareRepo(t, fixtureDir)
	ghSrv, requests := NewMockGitHubServer(t)

	// Map k8s version to full SSM paths.
	ssmParams := make(map[string]string)
	for k, v := range ssmVersions {
		ssmParams["/aws/service/eks/optimized-ami/"+k+"/amazon-linux-2/recommended/release_version"] = v
	}

	cfg := &appconfig.Config{
		GitHub:      appconfig.GitHubConfig{TokenSecretName: "test/token", APIURL: ghSrv.URL + "/"},
		K8sVersions: []string{"1.29"},
		AMIFamily:   "amazon-linux-2",
		Repos: []types.RepoTarget{{
			Owner: "my-org", Repo: "infra", Branch: "main",
			Scanners: []string{"terraform"},
		}},
		Concurrency: 1,
		PRTitle:     "chore: update AMI to {{.NewVersion}}",
	}

	h := handler.New(
		cfg,
		amiresolver.NewSSMResolver(NewMockSSMClient(ssmParams)),
		scanner.NewRegistry(),
		&fileVCS{bareURL: bareURL, gh: vcs.NewGitHubProvider(ghSrv.URL + "/")},
		&staticSecrets{token: "test-token"},
		slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	)
	return h, requests
}

func TestIntegrationHappyPath(t *testing.T) {
	fixtureDir := filepath.Join("..", "fixtures", "terraform")
	h, requests := makeIntegrationHandler(t, fixtureDir, map[string]string{"1.29": "1.29.5-20240701"})

	result, err := h.HandleEvent(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, result.Updated)
	assert.Len(t, result.PRURLs, 1)

	prCreated := false
	for _, req := range *requests {
		if req.Method == "POST" && req.Body != nil && req.Body["title"] != nil {
			prCreated = true
			break
		}
	}
	assert.True(t, prCreated, "expected a PR creation request to the mock GitHub server")
}

func TestIntegrationVersionCurrent(t *testing.T) {
	// The fixture already has version "1.29.3-20240531" which matches SSM.
	fixtureDir := filepath.Join("..", "fixtures", "terraform")
	h, _ := makeIntegrationHandler(t, fixtureDir, map[string]string{"1.29": "1.29.3-20240531"})

	result, err := h.HandleEvent(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, result.Updated)
	assert.Equal(t, 1, result.Skipped)
}
