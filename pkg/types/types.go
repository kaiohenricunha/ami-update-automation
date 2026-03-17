// Package types defines shared domain types and sentinel errors for ami-update-automation.
package types

import "errors"

// AMIVersion represents a resolved EKS AMI release version.
type AMIVersion struct {
	K8sVersion string
	AMIFamily  string
	Version    string
}

// RepoTarget describes a repository to scan and update.
type RepoTarget struct {
	Owner    string   `yaml:"owner"`
	Repo     string   `yaml:"repo"`
	Branch   string   `yaml:"branch"`
	Scanners []string `yaml:"scanners"`
	Paths    []string `yaml:"paths,omitempty"`
}

// ScanMatch records a single occurrence of an AMI version in a file.
type ScanMatch struct {
	FilePath   string
	LineNumber int
	OldVersion string
	Column     int
}

// ScanResult holds all matches found by a scanner in a repository.
type ScanResult struct {
	RepoDir     string
	ScannerType string
	Matches     []ScanMatch
}

// UpdateResult records the outcome of updating a single file.
type UpdateResult struct {
	FilePath   string
	LineNumber int
	OldVersion string
	NewVersion string
}

// PRRequest contains the data needed to open a pull request.
type PRRequest struct {
	Owner string
	Repo  string
	Title string
	Body  string
	Head  string
	Base  string
	Draft bool
}

// PRResult holds the result of PR creation.
type PRResult struct {
	URL    string
	Number int
	Exists bool
}

// HandlerResult aggregates outcomes for all repos processed in one invocation.
type HandlerResult struct {
	Processed int
	Updated   int
	Skipped   int
	Failed    int
	PRURLs    []string
	Errors    []string
}

// Sentinel errors.
var (
	ErrSSMParameterNotFound = errors.New("SSM parameter not found")
	ErrRepoCloneFailed      = errors.New("repository clone failed")
	ErrPRAlreadyExists      = errors.New("pull request already exists")
	ErrConfigValidation     = errors.New("config validation error")
	ErrInputValidation      = errors.New("input validation error")
	ErrNoMatchesFound       = errors.New("no AMI version matches found in repository")
	ErrSymlinkEscape        = errors.New("path escapes repository root via symlink")
	ErrTokenLeaked          = errors.New("credential detected in output")
)
