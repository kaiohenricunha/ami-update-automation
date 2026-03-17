// Package sanitize provides input validation and sanitization functions.
// All validation for external inputs flows through this package.
package sanitize

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/kaiohenricunha/ami-update-automation/pkg/types"
)

var (
	// k8sVersionRe matches "1.29", "1.30", etc.
	k8sVersionRe = regexp.MustCompile(`^\d+\.\d+$`)
	// amiVersionRe matches EKS AMI release versions like "1.29.3-20240531".
	amiVersionRe = regexp.MustCompile(`^\d+\.\d+[\w.\-]+$`)
	// repoNameRe allows alphanumeric, hyphens, underscores, dots.
	repoNameRe = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9._-]*[a-zA-Z0-9])?$`)
	// ownerRe matches GitHub owner names.
	ownerRe = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`)
	// branchNameRe allows common branch naming conventions.
	branchNameRe = regexp.MustCompile(`^[a-zA-Z0-9._/\-]+$`)
)

const (
	maxStringLength = 256
	maxPRBodyLength = 65536
	maxPathLength   = 4096
)

// ValidateK8sVersion checks that a Kubernetes version string is well-formed.
func ValidateK8sVersion(v string) error {
	if v == "" {
		return fmt.Errorf("%w: k8s version is empty", types.ErrInputValidation)
	}
	if len(v) > 16 {
		return fmt.Errorf("%w: k8s version too long", types.ErrInputValidation)
	}
	if !k8sVersionRe.MatchString(v) {
		return fmt.Errorf("%w: k8s version %q does not match \\d+\\.\\d+", types.ErrInputValidation, v)
	}
	return nil
}

// ValidateAMIVersion checks that an AMI release version string is safe.
func ValidateAMIVersion(v string) error {
	if v == "" {
		return fmt.Errorf("%w: AMI version is empty", types.ErrInputValidation)
	}
	if len(v) > 64 {
		return fmt.Errorf("%w: AMI version too long", types.ErrInputValidation)
	}
	if strings.ContainsAny(v, " \t\n\r;|&`$(){}[]<>\"'\\") {
		return fmt.Errorf("%w: AMI version contains illegal characters", types.ErrInputValidation)
	}
	if !amiVersionRe.MatchString(v) {
		return fmt.Errorf("%w: AMI version %q does not match expected pattern", types.ErrInputValidation, v)
	}
	return nil
}

// ValidateOwner checks that a GitHub org/user name is safe.
func ValidateOwner(owner string) error {
	if owner == "" {
		return fmt.Errorf("%w: owner is empty", types.ErrInputValidation)
	}
	if len(owner) > maxStringLength {
		return fmt.Errorf("%w: owner name too long", types.ErrInputValidation)
	}
	if !ownerRe.MatchString(owner) {
		return fmt.Errorf("%w: owner %q contains invalid characters", types.ErrInputValidation, owner)
	}
	return nil
}

// ValidateRepoName checks that a GitHub repository name is safe.
func ValidateRepoName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: repo name is empty", types.ErrInputValidation)
	}
	if len(name) > maxStringLength {
		return fmt.Errorf("%w: repo name too long", types.ErrInputValidation)
	}
	if !repoNameRe.MatchString(name) {
		return fmt.Errorf("%w: repo name %q contains invalid characters", types.ErrInputValidation, name)
	}
	return nil
}

// ValidateBranchName checks that a git branch name is safe.
func ValidateBranchName(branch string) error {
	if branch == "" {
		return fmt.Errorf("%w: branch name is empty", types.ErrInputValidation)
	}
	if len(branch) > maxStringLength {
		return fmt.Errorf("%w: branch name too long", types.ErrInputValidation)
	}
	if strings.Contains(branch, "\x00") {
		return fmt.Errorf("%w: branch name contains null byte", types.ErrInputValidation)
	}
	if !branchNameRe.MatchString(branch) {
		return fmt.Errorf("%w: branch name %q contains invalid characters", types.ErrInputValidation, branch)
	}
	return nil
}

// ValidatePath checks a file path for traversal and injection.
// rootDir is the repository root; path must be relative and within rootDir.
func ValidatePath(rootDir, path string) error {
	if path == "" {
		return fmt.Errorf("%w: path is empty", types.ErrInputValidation)
	}
	if len(path) > maxPathLength {
		return fmt.Errorf("%w: path too long", types.ErrInputValidation)
	}
	if strings.Contains(path, "\x00") {
		return fmt.Errorf("%w: path contains null byte", types.ErrInputValidation)
	}
	if filepath.IsAbs(path) {
		return fmt.Errorf("%w: absolute path not allowed: %s", types.ErrInputValidation, path)
	}

	cleaned := filepath.Clean(filepath.Join(rootDir, path))
	if !strings.HasPrefix(cleaned, filepath.Clean(rootDir)+string(os.PathSeparator)) &&
		cleaned != filepath.Clean(rootDir) {
		return fmt.Errorf("%w: path %q escapes root directory", types.ErrInputValidation, path)
	}
	return nil
}

// ValidateAbsPath ensures an absolute path doesn't escape a root via symlinks.
// It resolves symlinks and checks the result stays within rootDir.
func ValidateAbsPath(rootDir, absPath string) error {
	resolved, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: path does not exist: %s", types.ErrInputValidation, absPath)
		}
		return fmt.Errorf("%w: cannot resolve path: %s", types.ErrSymlinkEscape, absPath)
	}
	rootResolved, err := filepath.EvalSymlinks(rootDir)
	if err != nil {
		return fmt.Errorf("%w: cannot resolve root: %s", types.ErrSymlinkEscape, rootDir)
	}
	if !strings.HasPrefix(resolved, rootResolved+string(os.PathSeparator)) &&
		resolved != rootResolved {
		return fmt.Errorf("%w: %s -> %s escapes %s", types.ErrSymlinkEscape, absPath, resolved, rootDir)
	}
	return nil
}

// SanitizePRContent strips control characters and limits length for PR title/body.
func SanitizePRContent(s string, maxLen int) string {
	if maxLen <= 0 {
		maxLen = maxPRBodyLength
	}
	var b strings.Builder
	for _, r := range s {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			continue
		}
		b.WriteRune(r)
	}
	result := b.String()
	if len(result) > maxLen {
		result = result[:maxLen]
	}
	return result
}

// RedactToken replaces occurrences of token in s with [REDACTED].
func RedactToken(s, token string) string {
	if token == "" {
		return s
	}
	return strings.ReplaceAll(s, token, "[REDACTED]")
}
