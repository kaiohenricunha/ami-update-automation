package scanner

import (
	"context"
	"fmt"
	"regexp"

	"github.com/kaiohenricunha/ami-update-automation/pkg/types"
)

var (
	pulumiYAMLPattern = regexp.MustCompile(`(?m)(amiReleaseVersion:\s*")([^"]+)(")`)
	pulumiCodePattern = regexp.MustCompile(`(?m)(ami_release_version\s*(?::=|=|:)\s*")([^"]+)(")`)
)

// PulumiScanner scans Pulumi YAML and Go/TypeScript files.
type PulumiScanner struct{}

// Type returns "pulumi".
func (s *PulumiScanner) Type() string { return "pulumi" }

// Scan finds AMI version references in Pulumi project files.
func (s *PulumiScanner) Scan(ctx context.Context, repoDir string, paths []string) ([]types.ScanMatch, error) {
	yamlFiles, err := walkFiles(ctx, repoDir, paths, []string{".yaml", ".yml"})
	if err != nil {
		return nil, fmt.Errorf("pulumi scan yaml: %w", err)
	}
	codeFiles, err := walkFiles(ctx, repoDir, paths, []string{".go", ".ts"})
	if err != nil {
		return nil, fmt.Errorf("pulumi scan code: %w", err)
	}

	var matches []types.ScanMatch
	for _, f := range yamlFiles {
		data, err := readFile(repoDir, f)
		if err != nil {
			continue
		}
		matches = append(matches, findMatches(f, data, pulumiYAMLPattern)...)
	}
	for _, f := range codeFiles {
		data, err := readFile(repoDir, f)
		if err != nil {
			continue
		}
		matches = append(matches, findMatches(f, data, pulumiCodePattern)...)
	}
	return matches, nil
}

// Update rewrites Pulumi files with newVersion.
func (s *PulumiScanner) Update(ctx context.Context, repoDir string, matches []types.ScanMatch, newVersion string) ([]types.UpdateResult, error) {
	var results []types.UpdateResult
	yamlResults, err := updateMatches(ctx, repoDir, matches, newVersion, pulumiYAMLPattern)
	if err != nil {
		return nil, err
	}
	results = append(results, yamlResults...)
	codeResults, err := updateMatches(ctx, repoDir, matches, newVersion, pulumiCodePattern)
	if err != nil {
		return nil, err
	}
	results = append(results, codeResults...)
	return results, nil
}
