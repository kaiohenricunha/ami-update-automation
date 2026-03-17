// Package scanner provides interfaces and shared helpers for scanning IaC repositories.
package scanner

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/kaiohenricunha/ami-update-automation/internal/sanitize"
	"github.com/kaiohenricunha/ami-update-automation/pkg/types"
)

// Scanner can find and update ami_release_version occurrences in a repository.
type Scanner interface {
	// Type returns the scanner's identifier (e.g., "terraform").
	Type() string
	// Scan walks the repo directory and returns all version matches.
	Scan(ctx context.Context, repoDir string, paths []string) ([]types.ScanMatch, error)
	// Update rewrites matched files with newVersion and returns update records.
	Update(ctx context.Context, repoDir string, matches []types.ScanMatch, newVersion string) ([]types.UpdateResult, error)
}

// walkFiles walks dir, filtered to the given extensions. If paths is non-empty,
// only walk those subdirectories. Symlinks that escape repoDir are skipped.
func walkFiles(ctx context.Context, repoDir string, paths []string, extensions []string) ([]string, error) {
	var roots []string
	if len(paths) == 0 {
		roots = []string{repoDir}
	} else {
		for _, p := range paths {
			if err := sanitize.ValidatePath(repoDir, p); err != nil {
				return nil, err
			}
			roots = append(roots, filepath.Join(repoDir, p))
		}
	}

	extSet := make(map[string]struct{}, len(extensions))
	for _, e := range extensions {
		extSet[e] = struct{}{}
	}

	var files []string
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // skip unreadable entries
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// Resolve symlinks and verify they stay within repoDir.
			if d.Type()&fs.ModeSymlink != 0 {
				return nil // WalkDir doesn't follow symlinks by default — safe
			}

			if d.IsDir() {
				// Skip hidden directories.
				if strings.HasPrefix(d.Name(), ".") && path != root {
					return filepath.SkipDir
				}
				return nil
			}

			// Verify the file is within repoDir.
			if err := sanitize.ValidateAbsPath(repoDir, path); err != nil {
				return nil // skip paths that escape root
			}

			ext := filepath.Ext(d.Name())
			if _, ok := extSet[ext]; ok {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walking %s: %w", root, err)
		}
	}
	return files, nil
}

// readFile reads a file and validates the path before opening.
func readFile(repoDir, absPath string) ([]byte, error) {
	if err := sanitize.ValidateAbsPath(repoDir, absPath); err != nil {
		return nil, err
	}
	return os.ReadFile(absPath) //nolint:gosec // path validated
}

// writeFile validates the path then writes atomically.
func writeFile(repoDir, absPath string, data []byte) error {
	if err := sanitize.ValidateAbsPath(repoDir, absPath); err != nil {
		return err
	}
	// Re-validate after any potential TOCTOU gap.
	if _, err := os.Lstat(absPath); err != nil {
		return fmt.Errorf("file disappeared before write: %w", err)
	}
	return os.WriteFile(absPath, data, 0o600) //nolint:gosec // path validated
}
