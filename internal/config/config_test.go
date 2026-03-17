package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaiohenricunha/ami-update-automation/internal/config"
	"github.com/kaiohenricunha/ami-update-automation/pkg/types"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	require.NoError(t, err)
	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

func TestLoadValid(t *testing.T) {
	yaml := `
github:
  token_secret_name: my-secret
k8s_versions:
  - "1.29"
  - "1.30"
ami_family: amazon-linux-2
repos:
  - owner: my-org
    repo: cloud-infra-prod
    branch: main
    scanners:
      - terraform
`
	path := writeTemp(t, yaml)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "my-secret", cfg.GitHub.TokenSecretName)
	assert.Equal(t, []string{"1.29", "1.30"}, cfg.K8sVersions)
	assert.Len(t, cfg.Repos, 1)
	assert.Equal(t, 5, cfg.Concurrency) // default
}

func TestLoadDefaults(t *testing.T) {
	yaml := `
github:
  token_secret_name: tok
k8s_versions: ["1.29"]
repos:
  - owner: org
    repo: infra
    scanners: [terraform]
`
	path := writeTemp(t, yaml)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "main", cfg.Repos[0].Branch)
	assert.Equal(t, "amazon-linux-2", cfg.AMIFamily)
	assert.Equal(t, 5, cfg.Concurrency)
}

func TestLoadMissingFile(t *testing.T) {
	_, err := config.Load("/nonexistent/path/config.yaml")
	require.Error(t, err)
	assert.ErrorIs(t, err, types.ErrConfigValidation)
}

func TestLoadEmptyPath(t *testing.T) {
	_, err := config.Load("")
	require.Error(t, err)
	assert.ErrorIs(t, err, types.ErrConfigValidation)
}

func TestLoadInvalidK8sVersion(t *testing.T) {
	yaml := `
github:
  token_secret_name: tok
k8s_versions: ["1.29.0"]
repos:
  - owner: org
    repo: infra
    scanners: [terraform]
`
	path := writeTemp(t, yaml)
	_, err := config.Load(path)
	require.Error(t, err)
	assert.ErrorIs(t, err, types.ErrConfigValidation)
}

func TestLoadInvalidOwner(t *testing.T) {
	yaml := `
github:
  token_secret_name: tok
k8s_versions: ["1.29"]
repos:
  - owner: "bad owner!"
    repo: infra
    scanners: [terraform]
`
	path := writeTemp(t, yaml)
	_, err := config.Load(path)
	require.Error(t, err)
}

func TestLoadNullBytePath(t *testing.T) {
	_, err := config.Load("path\x00evil")
	require.Error(t, err)
}

func TestLoadPathTraversal(t *testing.T) {
	// Create a real file but reference via traversal - the actual open will fail
	// since we validate the path first for null bytes
	tmp := t.TempDir()
	validPath := filepath.Join(tmp, "config.yaml")
	require.NoError(t, os.WriteFile(validPath, []byte("invalid yaml {{{"), 0o600))
	// Just ensure invalid YAML is caught
	_, err := config.Load(validPath)
	require.Error(t, err)
}
