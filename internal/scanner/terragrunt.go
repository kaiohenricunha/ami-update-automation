package scanner

import (
	"context"
	"fmt"
	"regexp"

	"github.com/kaiohenricunha/ami-update-automation/pkg/types"
)

var hclAMIPattern = regexp.MustCompile(`(?m)(ami_release_version\s*=\s*")([^"]+)(")`)

// TerragruntScanner scans Terragrunt .hcl files.
type TerragruntScanner struct{}

// Type returns "terragrunt".
func (s *TerragruntScanner) Type() string { return "terragrunt" }

// Scan finds ami_release_version in .hcl files.
func (s *TerragruntScanner) Scan(ctx context.Context, repoDir string, paths []string) ([]types.ScanMatch, error) {
	files, err := walkFiles(ctx, repoDir, paths, []string{".hcl"})
	if err != nil {
		return nil, fmt.Errorf("terragrunt scan: %w", err)
	}
	var matches []types.ScanMatch
	for _, f := range files {
		data, err := readFile(repoDir, f)
		if err != nil {
			continue
		}
		ms := findMatches(f, data, hclAMIPattern)
		matches = append(matches, ms...)
	}
	return matches, nil
}

// Update rewrites matched .hcl files with newVersion.
func (s *TerragruntScanner) Update(ctx context.Context, repoDir string, matches []types.ScanMatch, newVersion string) ([]types.UpdateResult, error) {
	return updateMatches(ctx, repoDir, matches, newVersion, hclAMIPattern)
}
