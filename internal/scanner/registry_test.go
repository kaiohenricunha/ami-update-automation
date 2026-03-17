package scanner_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaiohenricunha/ami-update-automation/internal/scanner"
)

func TestRegistry(t *testing.T) {
	r := scanner.NewRegistry()

	for _, name := range []string{"terraform", "terragrunt", "pulumi", "crossplane"} {
		s, err := r.Get(name)
		require.NoError(t, err)
		assert.Equal(t, name, s.Type())
	}

	_, err := r.Get("unknown")
	require.Error(t, err)
}

func TestRegistryGetAll(t *testing.T) {
	r := scanner.NewRegistry()
	scanners, err := r.GetAll([]string{"terraform", "crossplane"})
	require.NoError(t, err)
	assert.Len(t, scanners, 2)

	_, err = r.GetAll([]string{"terraform", "nonexistent"})
	require.Error(t, err)
}
