package scanner_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaiohenricunha/ami-update-automation/internal/scanner"
)

func TestTerragruntScanner(t *testing.T) {
	dir := t.TempDir()
	content := `
inputs = {
  ami_release_version = "1.29.3-20240531"
  cluster_name        = "prod"
}
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "terragrunt.hcl"), []byte(content), 0o600))

	s := &scanner.TerragruntScanner{}
	assert.Equal(t, "terragrunt", s.Type())

	matches, err := s.Scan(context.Background(), dir, nil)
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, "1.29.3-20240531", matches[0].OldVersion)

	results, err := s.Update(context.Background(), dir, matches, "1.29.5-20240701")
	require.NoError(t, err)
	assert.Len(t, results, 1)

	updated, _ := os.ReadFile(filepath.Join(dir, "terragrunt.hcl"))
	assert.Contains(t, string(updated), `ami_release_version = "1.29.5-20240701"`)
}
