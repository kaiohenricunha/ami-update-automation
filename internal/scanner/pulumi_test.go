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

func TestPulumiYAMLScanner(t *testing.T) {
	dir := t.TempDir()
	content := `
name: my-eks
runtime: yaml
resources:
  nodeGroup:
    type: aws:eks:NodeGroup
    properties:
      amiReleaseVersion: "1.29.3-20240531"
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.yaml"), []byte(content), 0o600))

	s := &scanner.PulumiScanner{}
	assert.Equal(t, "pulumi", s.Type())

	matches, err := s.Scan(context.Background(), dir, nil)
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, "1.29.3-20240531", matches[0].OldVersion)
}

func TestPulumiGoScanner(t *testing.T) {
	dir := t.TempDir()
	content := `
package main

import "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/eks"

func main() {
	ami_release_version = "1.29.3-20240531"
}
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte(content), 0o600))

	s := &scanner.PulumiScanner{}
	matches, err := s.Scan(context.Background(), dir, nil)
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, "1.29.3-20240531", matches[0].OldVersion)
}
