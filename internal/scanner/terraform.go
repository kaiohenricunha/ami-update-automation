package scanner

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/kaiohenricunha/ami-update-automation/pkg/types"
)

var tfAMIPattern = regexp.MustCompile(`(?m)(ami_release_version\s*=\s*")([^"]+)(")`)

// TerraformScanner scans Terraform .tf files for ami_release_version.
type TerraformScanner struct{}

// Type returns "terraform".
func (s *TerraformScanner) Type() string { return "terraform" }

// Scan finds all ami_release_version assignments in .tf files.
func (s *TerraformScanner) Scan(ctx context.Context, repoDir string, paths []string) ([]types.ScanMatch, error) {
	files, err := walkFiles(ctx, repoDir, paths, []string{".tf"})
	if err != nil {
		return nil, fmt.Errorf("terraform scan: %w", err)
	}
	var matches []types.ScanMatch
	for _, f := range files {
		data, err := readFile(repoDir, f)
		if err != nil {
			continue
		}
		ms := findMatches(f, data, tfAMIPattern)
		matches = append(matches, ms...)
	}
	return matches, nil
}

// Update rewrites all matches in .tf files with newVersion.
func (s *TerraformScanner) Update(ctx context.Context, repoDir string, matches []types.ScanMatch, newVersion string) ([]types.UpdateResult, error) {
	return updateMatches(ctx, repoDir, matches, newVersion, tfAMIPattern)
}

// findMatches extracts ScanMatch records from file content using a regex with 3 groups:
// group 1: prefix (e.g. `ami_release_version = "`), group 2: version, group 3: suffix.
func findMatches(filePath string, data []byte, re *regexp.Regexp) []types.ScanMatch {
	var matches []types.ScanMatch
	lines := bytes.Split(data, []byte("\n"))
	for lineNum, line := range lines {
		locs := re.FindAllSubmatchIndex(line, -1)
		for _, loc := range locs {
			if len(loc) < 6 {
				continue
			}
			version := string(line[loc[4]:loc[5]])
			matches = append(matches, types.ScanMatch{
				FilePath:   filePath,
				LineNumber: lineNum + 1,
				OldVersion: version,
				Column:     loc[4],
			})
		}
	}
	return matches
}

// updateMatches rewrites all matched files replacing oldVersion with newVersion.
func updateMatches(_ context.Context, repoDir string, matches []types.ScanMatch, newVersion string, re *regexp.Regexp) ([]types.UpdateResult, error) {
	// Group matches by file.
	byFile := make(map[string][]types.ScanMatch)
	for _, m := range matches {
		byFile[m.FilePath] = append(byFile[m.FilePath], m)
	}

	var results []types.UpdateResult
	for filePath, fileMatches := range byFile {
		data, err := readFile(repoDir, filePath)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", filePath, err)
		}
		original := string(data)
		updated := re.ReplaceAllStringFunc(original, func(s string) string {
			sub := re.FindStringSubmatch(s)
			if len(sub) < 4 {
				return s
			}
			return sub[1] + newVersion + sub[3]
		})
		if updated == original {
			continue
		}
		if err := writeFile(repoDir, filePath, []byte(updated)); err != nil {
			return nil, fmt.Errorf("writing %s: %w", filePath, err)
		}
		for _, m := range fileMatches {
			if strings.Contains(original, m.OldVersion) {
				results = append(results, types.UpdateResult{
					FilePath:   filePath,
					LineNumber: m.LineNumber,
					OldVersion: m.OldVersion,
					NewVersion: newVersion,
				})
			}
		}
	}
	return results, nil
}
