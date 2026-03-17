package handler_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaiohenricunha/ami-update-automation/internal/config"
	"github.com/kaiohenricunha/ami-update-automation/internal/handler"
	"github.com/kaiohenricunha/ami-update-automation/internal/scanner"
	"github.com/kaiohenricunha/ami-update-automation/pkg/types"
)

// --- Mocks ---

type mockResolver struct {
	versions map[string]*types.AMIVersion
	err      error
}

func (m *mockResolver) Resolve(_ context.Context, k8sVer, _ string) (*types.AMIVersion, error) {
	if m.err != nil {
		return nil, m.err
	}
	if v, ok := m.versions[k8sVer]; ok {
		return v, nil
	}
	return nil, types.ErrSSMParameterNotFound
}

type mockVCS struct {
	cloneDir    string
	cloneErr    error
	prURL       string
	prErr       error
	prExists    bool
	prExistsErr error
	pushErr     error
}

func (m *mockVCS) Clone(_ context.Context, _, _, _ string) (string, error) {
	return m.cloneDir, m.cloneErr
}
func (m *mockVCS) CreateBranch(_ context.Context, _, _ string) error { return nil }
func (m *mockVCS) CommitAndPush(_ context.Context, _, _, _, _ string) error {
	return m.pushErr
}
func (m *mockVCS) CreatePR(_ context.Context, _ types.PRRequest, _ string) (*types.PRResult, error) {
	if m.prErr != nil {
		return nil, m.prErr
	}
	return &types.PRResult{URL: m.prURL, Number: 1}, nil
}
func (m *mockVCS) PRExists(_ context.Context, _, _, _, _ string) (bool, error) {
	return m.prExists, m.prExistsErr
}
func (m *mockVCS) Cleanup(_ string) error { return nil }

type mockSecrets struct {
	value string
	err   error
}

func (m *mockSecrets) GetSecret(_ context.Context, _ string) (string, error) {
	return m.value, m.err
}

func makeConfig(scanners []string, repos ...types.RepoTarget) *config.Config {
	if len(repos) == 0 {
		repos = []types.RepoTarget{{
			Owner:    "my-org",
			Repo:     "infra",
			Branch:   "main",
			Scanners: scanners,
		}}
	}
	return &config.Config{
		GitHub:      config.GitHubConfig{TokenSecretName: "test/token"},
		K8sVersions: []string{"1.29"},
		AMIFamily:   "amazon-linux-2",
		Repos:       repos,
		Concurrency: 2,
		PRTitle:     "chore: update AMI to {{.NewVersion}}",
	}
}

func newLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestHandleEventHappyPath(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.tf"), []byte(`ami_release_version = "1.29.0-20240101"`), 0o600))

	vcsProvider := &mockVCS{cloneDir: dir, prURL: "https://github.com/my-org/infra/pull/1"}
	resolver := &mockResolver{versions: map[string]*types.AMIVersion{
		"1.29": {K8sVersion: "1.29", AMIFamily: "amazon-linux-2", Version: "1.29.3-20240531"},
	}}

	h := handler.New(
		makeConfig([]string{"terraform"}),
		resolver,
		scanner.NewRegistry(),
		vcsProvider,
		&mockSecrets{value: "ghp_test"},
		newLogger(),
	)

	result, err := h.HandleEvent(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, result.Processed)
	assert.Equal(t, 1, result.Updated)
	assert.Equal(t, 0, result.Skipped)
	assert.Len(t, result.PRURLs, 1)
}

func TestHandleEventVersionCurrent(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.tf"), []byte(`ami_release_version = "1.29.3-20240531"`), 0o600))

	h := handler.New(
		makeConfig([]string{"terraform"}),
		&mockResolver{versions: map[string]*types.AMIVersion{
			"1.29": {K8sVersion: "1.29", AMIFamily: "amazon-linux-2", Version: "1.29.3-20240531"},
		}},
		scanner.NewRegistry(),
		&mockVCS{cloneDir: dir},
		&mockSecrets{value: "ghp_test"},
		newLogger(),
	)

	result, err := h.HandleEvent(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, result.Updated)
	assert.Equal(t, 1, result.Skipped)
}

func TestHandleEventSSMError(t *testing.T) {
	h := handler.New(
		makeConfig([]string{"terraform"}),
		&mockResolver{err: errors.New("ssm unavailable")},
		scanner.NewRegistry(),
		&mockVCS{},
		&mockSecrets{value: "ghp_test"},
		newLogger(),
	)
	_, err := h.HandleEvent(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolving AMI")
}

func TestHandleEventTokenError(t *testing.T) {
	h := handler.New(
		makeConfig([]string{"terraform"}),
		&mockResolver{},
		scanner.NewRegistry(),
		&mockVCS{},
		&mockSecrets{err: errors.New("access denied")},
		newLogger(),
	)
	_, err := h.HandleEvent(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetching GitHub token")
}

func TestHandleEventCloneFailure(t *testing.T) {
	h := handler.New(
		makeConfig([]string{"terraform"}),
		&mockResolver{versions: map[string]*types.AMIVersion{
			"1.29": {K8sVersion: "1.29", AMIFamily: "amazon-linux-2", Version: "1.29.3-20240531"},
		}},
		scanner.NewRegistry(),
		&mockVCS{cloneErr: types.ErrRepoCloneFailed},
		&mockSecrets{value: "ghp_test"},
		newLogger(),
	)

	result, err := h.HandleEvent(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, result.Failed)
}

func TestHandleEventPRAlreadyExists(t *testing.T) {
	h := handler.New(
		makeConfig([]string{"terraform"}),
		&mockResolver{versions: map[string]*types.AMIVersion{
			"1.29": {K8sVersion: "1.29", AMIFamily: "amazon-linux-2", Version: "1.29.3-20240531"},
		}},
		scanner.NewRegistry(),
		&mockVCS{prExists: true},
		&mockSecrets{value: "ghp_test"},
		newLogger(),
	)

	result, err := h.HandleEvent(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, result.Skipped)
}
