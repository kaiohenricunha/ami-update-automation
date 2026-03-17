// Package handler contains the core orchestration logic for the AMI update Lambda.
package handler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/kaiohenricunha/ami-update-automation/internal/ami"
	"github.com/kaiohenricunha/ami-update-automation/internal/config"
	"github.com/kaiohenricunha/ami-update-automation/internal/logging"
	"github.com/kaiohenricunha/ami-update-automation/internal/sanitize"
	"github.com/kaiohenricunha/ami-update-automation/internal/scanner"
	"github.com/kaiohenricunha/ami-update-automation/internal/secrets"
	"github.com/kaiohenricunha/ami-update-automation/internal/vcs"
	"github.com/kaiohenricunha/ami-update-automation/pkg/types"
)

// Handler orchestrates AMI version discovery, scanning, and PR creation.
type Handler struct {
	cfg      *config.Config
	resolver ami.AMIResolver
	registry *scanner.Registry
	vcs      vcs.VCSProvider
	secrets  secrets.SecretsProvider
	logger   *slog.Logger
}

// New constructs a Handler with all required dependencies.
func New(
	cfg *config.Config,
	resolver ami.AMIResolver,
	registry *scanner.Registry,
	vcsProvider vcs.VCSProvider,
	secretsProvider secrets.SecretsProvider,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		cfg:      cfg,
		resolver: resolver,
		registry: registry,
		vcs:      vcsProvider,
		secrets:  secretsProvider,
		logger:   logger,
	}
}

// HandleEvent runs the full AMI update workflow for all configured repos and K8s versions.
func (h *Handler) HandleEvent(ctx context.Context) (*types.HandlerResult, error) {
	result := &types.HandlerResult{}

	// Fetch the GitHub token once — fatal if unavailable.
	token, err := h.secrets.GetSecret(ctx, h.cfg.GitHub.TokenSecretName)
	if err != nil {
		return nil, fmt.Errorf("fetching GitHub token: %w", err)
	}

	// Resolve all K8s versions — fatal if SSM is unreachable.
	amiVersions := make(map[string]*types.AMIVersion, len(h.cfg.K8sVersions))
	for _, k8sVer := range h.cfg.K8sVersions {
		v, err := h.resolver.Resolve(ctx, k8sVer, h.cfg.AMIFamily)
		if err != nil {
			return nil, fmt.Errorf("resolving AMI for k8s %s: %w", k8sVer, err)
		}
		amiVersions[k8sVer] = v
		h.logger.InfoContext(ctx, "resolved AMI version",
			slog.String("k8s_version", k8sVer),
			slog.String("ami_version", v.Version),
		)
	}

	// Process repos in parallel, bounded by concurrency setting.
	sem := make(chan struct{}, h.cfg.Concurrency)
	eg, egCtx := errgroup.WithContext(ctx)

	type repoOutcome struct {
		updated bool
		prURL   string
		err     error
	}
	outcomes := make(chan repoOutcome, len(h.cfg.Repos)*len(h.cfg.K8sVersions))

	for _, repo := range h.cfg.Repos {
		for _, k8sVer := range h.cfg.K8sVersions {
			repo := repo
			amiVer := amiVersions[k8sVer]

			eg.Go(func() error {
				sem <- struct{}{}
				defer func() { <-sem }()

				outcome := h.processRepo(egCtx, repo, amiVer, token)
				outcomes <- outcome
				return nil // fail-open: errors are collected in outcomes
			})
		}
	}

	// Wait for all goroutines then close channel.
	go func() {
		_ = eg.Wait()
		close(outcomes)
	}()

	for o := range outcomes {
		result.Processed++
		if o.err != nil {
			result.Failed++
			result.Errors = append(result.Errors, o.err.Error())
		} else if o.updated {
			result.Updated++
			if o.prURL != "" {
				result.PRURLs = append(result.PRURLs, o.prURL)
			}
		} else {
			result.Skipped++
		}
	}

	h.logger.InfoContext(ctx, "handler complete",
		slog.Int("processed", result.Processed),
		slog.Int("updated", result.Updated),
		slog.Int("skipped", result.Skipped),
		slog.Int("failed", result.Failed),
	)
	return result, nil
}

// processRepo handles a single (repo, amiVersion) combination.
func (h *Handler) processRepo(ctx context.Context, repo types.RepoTarget, amiVer *types.AMIVersion, token string) (outcome struct {
	updated bool
	prURL   string
	err     error
}) {
	ctx = logging.WithRepo(ctx, repo.Owner+"/"+repo.Repo)
	ctx = logging.WithK8sVersion(ctx, amiVer.K8sVersion)

	log := h.logger.With(
		slog.String("repo", repo.Owner+"/"+repo.Repo),
		slog.String("k8s_version", amiVer.K8sVersion),
	)

	branchName := fmt.Sprintf("ami-update/%s-%s", amiVer.K8sVersion, time.Now().UTC().Format("20060102"))
	if err := sanitize.ValidateBranchName(branchName); err != nil {
		outcome.err = err
		return
	}

	// Check if PR already exists.
	exists, err := h.vcs.PRExists(ctx, repo.Owner, repo.Repo, branchName, token)
	if err != nil {
		log.WarnContext(ctx, "checking PR existence failed", slog.String("error", err.Error()))
		// Continue — we'll catch duplicate PR error on creation if needed.
	}
	if exists {
		log.InfoContext(ctx, "PR already exists, skipping")
		outcome.updated = false
		return
	}

	// Clone the repository.
	repoDir, err := h.vcs.Clone(ctx, repo.Owner, repo.Repo, token)
	if err != nil {
		outcome.err = fmt.Errorf("cloning %s/%s: %w", repo.Owner, repo.Repo, err)
		return
	}
	defer func() {
		if cleanErr := h.vcs.Cleanup(repoDir); cleanErr != nil {
			log.WarnContext(ctx, "cleanup failed", slog.String("error", cleanErr.Error()))
		}
	}()

	// Scan with all configured scanners.
	scanners, err := h.registry.GetAll(repo.Scanners)
	if err != nil {
		outcome.err = fmt.Errorf("loading scanners for %s/%s: %w", repo.Owner, repo.Repo, err)
		return
	}

	var allMatches []types.ScanMatch
	for _, s := range scanners {
		matches, err := s.Scan(ctx, repoDir, repo.Paths)
		if err != nil {
			log.WarnContext(ctx, "scan error", slog.String("scanner", s.Type()), slog.String("error", err.Error()))
			continue
		}
		// Filter to only stale matches.
		for _, m := range matches {
			if m.OldVersion != amiVer.Version {
				allMatches = append(allMatches, m)
			}
		}
	}

	if len(allMatches) == 0 {
		log.InfoContext(ctx, "no updates needed")
		return
	}

	log.InfoContext(ctx, "found stale AMI versions",
		slog.Int("count", len(allMatches)),
		slog.String("new_version", amiVer.Version),
	)

	// Create branch.
	if err := h.vcs.CreateBranch(ctx, repoDir, branchName); err != nil {
		outcome.err = fmt.Errorf("creating branch: %w", err)
		return
	}

	// Update files.
	for _, s := range scanners {
		var scannerMatches []types.ScanMatch
		for _, m := range allMatches {
			scannerMatches = append(scannerMatches, m)
		}
		if _, err := s.Update(ctx, repoDir, scannerMatches, amiVer.Version); err != nil {
			log.WarnContext(ctx, "update error", slog.String("scanner", s.Type()), slog.String("error", err.Error()))
		}
	}

	// Commit and push.
	msg := fmt.Sprintf("chore: update EKS AMI to %s for k8s %s", amiVer.Version, amiVer.K8sVersion)
	if err := h.vcs.CommitAndPush(ctx, repoDir, branchName, msg, token); err != nil {
		outcome.err = fmt.Errorf("commit+push: %w", err)
		return
	}

	// Build PR title and body.
	title := buildPRTitle(h.cfg.PRTitle, amiVer)
	body := buildPRBody(h.cfg.PRBodyTmpl, repo, amiVer, allMatches)

	// Create PR.
	pr, err := h.vcs.CreatePR(ctx, types.PRRequest{
		Owner: repo.Owner,
		Repo:  repo.Repo,
		Title: title,
		Body:  body,
		Head:  branchName,
		Base:  repo.Branch,
	}, token)
	if err != nil {
		outcome.err = fmt.Errorf("creating PR: %w", err)
		return
	}

	log.InfoContext(ctx, "PR created", slog.String("url", pr.URL))
	outcome.updated = true
	outcome.prURL = pr.URL
	return
}

func buildPRTitle(tmpl string, v *types.AMIVersion) string {
	r := strings.NewReplacer(
		"{{.NewVersion}}", v.Version,
		"{{.K8sVersion}}", v.K8sVersion,
	)
	return sanitize.SanitizePRContent(r.Replace(tmpl), 256)
}

func buildPRBody(tmpl string, repo types.RepoTarget, v *types.AMIVersion, matches []types.ScanMatch) string {
	if tmpl != "" {
		r := strings.NewReplacer(
			"{{.NewVersion}}", v.Version,
			"{{.K8sVersion}}", v.K8sVersion,
			"{{.Repo}}", repo.Owner+"/"+repo.Repo,
		)
		return sanitize.SanitizePRContent(r.Replace(tmpl), 65536)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "## AMI Version Update\n\n")
	fmt.Fprintf(&sb, "Updates EKS AMI release version to `%s` for Kubernetes %s.\n\n", v.Version, v.K8sVersion)
	fmt.Fprintf(&sb, "### Files Updated\n\n")
	for _, m := range matches {
		fmt.Fprintf(&sb, "- `%s` (line %d): `%s` → `%s`\n", m.FilePath, m.LineNumber, m.OldVersion, v.Version)
	}
	fmt.Fprintf(&sb, "\n---\n_Automated update by ami-update-automation_\n")
	return sanitize.SanitizePRContent(sb.String(), 65536)
}
