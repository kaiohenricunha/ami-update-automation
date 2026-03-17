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

func TestTerraformScanner(t *testing.T) {
	dir := t.TempDir()
	content := `
resource "aws_eks_node_group" "workers" {
  ami_release_version = "1.29.3-20240531"
  cluster_name        = "my-cluster"
}
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "nodes.tf"), []byte(content), 0o600))

	s := &scanner.TerraformScanner{}
	assert.Equal(t, "terraform", s.Type())

	matches, err := s.Scan(context.Background(), dir, nil)
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, "1.29.3-20240531", matches[0].OldVersion)

	results, err := s.Update(context.Background(), dir, matches, "1.29.5-20240701")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "1.29.5-20240701", results[0].NewVersion)

	updated, err := os.ReadFile(filepath.Join(dir, "nodes.tf"))
	require.NoError(t, err)
	assert.Contains(t, string(updated), `ami_release_version = "1.29.5-20240701"`)
}

func TestTerraformScannerNoMatch(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.tf"), []byte(`resource "aws_vpc" "main" {}`), 0o600))

	s := &scanner.TerraformScanner{}
	matches, err := s.Scan(context.Background(), dir, nil)
	require.NoError(t, err)
	assert.Empty(t, matches)
}
