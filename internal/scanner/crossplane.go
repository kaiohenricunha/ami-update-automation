package scanner

import (
	"context"
	"fmt"
	"regexp"

	"github.com/kaiohenricunha/ami-update-automation/pkg/types"
)

var (
	crossplaneSnakePattern = regexp.MustCompile(`(?m)(ami_release_version:\s*")([^"]+)(")`)
	crossplaneCamelPattern = regexp.MustCompile(`(?m)(amiReleaseVersion:\s*")([^"]+)(")`)
	// Also handle unquoted YAML values.
	crossplaneSnakeUnquotedPattern = regexp.MustCompile(`(?m)(ami_release_version:\s*)([^\s"#\n]+)`)
	crossplaneCamelUnquotedPattern = regexp.MustCompile(`(?m)(amiReleaseVersion:\s*)([^\s"#\n]+)`)
)

// CrossplaneScanner scans Crossplane YAML manifests.
type CrossplaneScanner struct{}

// Type returns "crossplane".
func (s *CrossplaneScanner) Type() string { return "crossplane" }

// Scan finds AMI version references in Crossplane YAML files.
func (s *CrossplaneScanner) Scan(ctx context.Context, repoDir string, paths []string) ([]types.ScanMatch, error) {
	files, err := walkFiles(ctx, repoDir, paths, []string{".yaml", ".yml"})
	if err != nil {
		return nil, fmt.Errorf("crossplane scan: %w", err)
	}
	var matches []types.ScanMatch
	for _, f := range files {
		data, err := readFile(repoDir, f)
		if err != nil {
			continue
		}
		matches = append(matches, findMatches(f, data, crossplaneSnakePattern)...)
		matches = append(matches, findMatches(f, data, crossplaneCamelPattern)...)
		matches = append(matches, findMatchesUnquoted(f, data, crossplaneSnakeUnquotedPattern)...)
		matches = append(matches, findMatchesUnquoted(f, data, crossplaneCamelUnquotedPattern)...)
	}
	return matches, nil
}

// Update rewrites matched Crossplane YAML files.
func (s *CrossplaneScanner) Update(ctx context.Context, repoDir string, matches []types.ScanMatch, newVersion string) ([]types.UpdateResult, error) {
	var results []types.UpdateResult
	for _, re := range []*regexp.Regexp{crossplaneSnakePattern, crossplaneCamelPattern} {
		r, err := updateMatches(ctx, repoDir, matches, newVersion, re)
		if err != nil {
			return nil, err
		}
		results = append(results, r...)
	}
	for _, re := range []*regexp.Regexp{crossplaneSnakeUnquotedPattern, crossplaneCamelUnquotedPattern} {
		r, err := updateMatchesUnquoted(ctx, repoDir, matches, newVersion, re)
		if err != nil {
			return nil, err
		}
		results = append(results, r...)
	}
	return results, nil
}

// findMatchesUnquoted handles YAML patterns without surrounding quotes (2 groups).
func findMatchesUnquoted(filePath string, data []byte, re *regexp.Regexp) []types.ScanMatch {
	var matches []types.ScanMatch
	lines := splitLines(data)
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

// updateMatchesUnquoted handles YAML patterns without surrounding quotes.
func updateMatchesUnquoted(_ context.Context, repoDir string, matches []types.ScanMatch, newVersion string, re *regexp.Regexp) ([]types.UpdateResult, error) {
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
			if len(sub) < 3 {
				return s
			}
			return sub[1] + newVersion
		})
		if updated == original {
			continue
		}
		if err := writeFile(repoDir, filePath, []byte(updated)); err != nil {
			return nil, fmt.Errorf("writing %s: %w", filePath, err)
		}
		for _, m := range fileMatches {
			results = append(results, types.UpdateResult{
				FilePath:   filePath,
				LineNumber: m.LineNumber,
				OldVersion: m.OldVersion,
				NewVersion: newVersion,
			})
		}
	}
	return results, nil
}

func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			lines = append(lines, data[start:i])
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}
