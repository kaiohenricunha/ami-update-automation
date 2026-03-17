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

func TestCrossplaneScanner(t *testing.T) {
	dir := t.TempDir()
	content := `
apiVersion: eks.aws.upbound.io/v1beta1
kind: NodeGroup
metadata:
  name: workers
spec:
  forProvider:
    ami_release_version: "1.29.3-20240531"
    clusterNameSelector:
      matchLabels:
        name: prod
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "nodegroup.yaml"), []byte(content), 0o600))

	s := &scanner.CrossplaneScanner{}
	assert.Equal(t, "crossplane", s.Type())

	matches, err := s.Scan(context.Background(), dir, nil)
	require.NoError(t, err)
	require.NotEmpty(t, matches)
	assert.Equal(t, "1.29.3-20240531", matches[0].OldVersion)
}

func TestCrossplaneUnquoted(t *testing.T) {
	dir := t.TempDir()
	content := `
spec:
  forProvider:
    amiReleaseVersion: 1.29.3-20240531
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ng.yaml"), []byte(content), 0o600))

	s := &scanner.CrossplaneScanner{}
	matches, err := s.Scan(context.Background(), dir, nil)
	require.NoError(t, err)
	require.NotEmpty(t, matches)
	assert.Equal(t, "1.29.3-20240531", matches[0].OldVersion)
}
