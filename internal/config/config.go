// Package config handles loading and validating the YAML configuration file.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/kaiohenricunha/ami-update-automation/internal/sanitize"
	"github.com/kaiohenricunha/ami-update-automation/pkg/types"
)

// Config is the top-level configuration structure.
type Config struct {
	GitHub      GitHubConfig       `yaml:"github"`
	K8sVersions []string           `yaml:"k8s_versions"`
	AMIFamily   string             `yaml:"ami_family"`
	Repos       []types.RepoTarget `yaml:"repos"`
	PRTitle     string             `yaml:"pr_title,omitempty"`
	PRBodyTmpl  string             `yaml:"pr_body_template,omitempty"`
	Concurrency int                `yaml:"concurrency,omitempty"`
}

// GitHubConfig holds GitHub-specific settings.
type GitHubConfig struct {
	TokenSecretName string `yaml:"token_secret_name"`
	APIURL          string `yaml:"api_url,omitempty"`
}

// Load reads and parses a YAML config file.
func Load(path string) (*Config, error) {
	if path == "" {
		return nil, fmt.Errorf("%w: config path is empty", types.ErrConfigValidation)
	}

	// Validate path before opening.
	if err := validateConfigPath(path); err != nil {
		return nil, err
	}

	f, err := os.Open(path) //nolint:gosec // path is validated above
	if err != nil {
		return nil, fmt.Errorf("%w: cannot open config file: %w", types.ErrConfigValidation, err)
	}
	defer f.Close()

	var cfg Config
	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("%w: YAML parse error: %w", types.ErrConfigValidation, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// validateConfigPath ensures the path is safe before opening.
func validateConfigPath(path string) error {
	if len(path) > 4096 {
		return fmt.Errorf("%w: config path too long", types.ErrConfigValidation)
	}
	// Allow absolute paths for config files but disallow null bytes and traversal components.
	for _, c := range path {
		if c == 0 {
			return fmt.Errorf("%w: config path contains null byte", types.ErrConfigValidation)
		}
	}
	return nil
}

// Validate checks all required fields using sanitize functions.
func (c *Config) Validate() error {
	if c.GitHub.TokenSecretName == "" {
		return fmt.Errorf("%w: github.token_secret_name is required", types.ErrConfigValidation)
	}
	if len(c.K8sVersions) == 0 {
		return fmt.Errorf("%w: k8s_versions must not be empty", types.ErrConfigValidation)
	}
	for _, v := range c.K8sVersions {
		if err := sanitize.ValidateK8sVersion(v); err != nil {
			return fmt.Errorf("%w: invalid k8s_version %q: %w", types.ErrConfigValidation, v, err)
		}
	}
	if c.AMIFamily == "" {
		c.AMIFamily = "amazon-linux-2"
	}
	if len(c.Repos) == 0 {
		return fmt.Errorf("%w: repos must not be empty", types.ErrConfigValidation)
	}
	for i, r := range c.Repos {
		if err := sanitize.ValidateOwner(r.Owner); err != nil {
			return fmt.Errorf("%w: repos[%d].owner: %w", types.ErrConfigValidation, i, err)
		}
		if err := sanitize.ValidateRepoName(r.Repo); err != nil {
			return fmt.Errorf("%w: repos[%d].repo: %w", types.ErrConfigValidation, i, err)
		}
		if r.Branch != "" {
			if err := sanitize.ValidateBranchName(r.Branch); err != nil {
				return fmt.Errorf("%w: repos[%d].branch: %w", types.ErrConfigValidation, i, err)
			}
		} else {
			c.Repos[i].Branch = "main"
		}
		if len(r.Scanners) == 0 {
			return fmt.Errorf("%w: repos[%d].scanners must not be empty", types.ErrConfigValidation, i)
		}
	}
	if c.Concurrency <= 0 {
		c.Concurrency = 5
	}
	if c.PRTitle == "" {
		c.PRTitle = "chore: update EKS AMI release version to {{.NewVersion}}"
	}
	return nil
}
