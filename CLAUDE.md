# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Purpose

This project automates the update of EKS node AMI release versions across infrastructure repositories. A Go AWS Lambda function periodically:

1. Queries AWS SSM Parameter Store for the latest recommended EKS-optimized AMI version for a given Kubernetes version (path: `/aws/service/eks/optimized-ami/<k8s-version>/amazon-linux-2/recommended/release_version`)
2. Scans target infra repositories for `ami_release_version` occurrences across Terraform, Terragrunt, Pulumi, and Crossplane files
3. If the version has changed, opens a pull request updating the value

## Commands

```bash
# Build
make build          # linux/amd64 binary
make build-arm64    # linux/arm64 binary (recommended for Lambda)
make zip            # lambda.zip (amd64)
make zip-arm64      # lambda-arm64.zip

# Test
make test               # unit tests (no infra required, ~10s)
make test-integration   # integration tests (real git + mock GitHub, ~30s)
make test-security      # adversarial input tests (~15s)
make test-e2e           # e2e tests with LocalStack (needs Docker, ~60s)
make test-cover         # unit tests with coverage report

# Lint
make lint           # golangci-lint run

# Clean
make clean
```

## Architecture

```
cmd/lambda/         Lambda entrypoint — wires all dependencies, calls lambda.Start
internal/
  ami/              AMIResolver interface + SSM implementation
  config/           YAML config loader + field validation
  handler/          Core orchestration: SSM lookup → scan → diff → PR
  logging/          Structured JSON logger (slog) with context helpers
  sanitize/         All input validation/sanitization — central security layer
  scanner/          Scanner interface + Terraform/Terragrunt/Pulumi/Crossplane impls
  secrets/          SecretsProvider interface + AWS Secrets Manager impl
  vcs/              VCSProvider interface + git CLI ops + GitHub API impl
pkg/types/          Shared domain types and sentinel errors
test/
  fixtures/         IaC test fixture files
  integration/      Integration tests (build tag: integration)
  security/         Adversarial input tests (build tag: security)
  e2e/              End-to-end tests with LocalStack (build tag: e2e)
deploy/terraform/   Terraform module for Lambda + IAM + EventBridge
```

### Key Data Flow

1. **Trigger**: EventBridge rule invokes the Lambda on a schedule
2. **Secret fetch**: GitHub token retrieved from AWS Secrets Manager
3. **SSM lookup**: Latest AMI version resolved for each configured K8s version
4. **Repo scan**: Clone target repo (shallow), walk files for `ami_release_version` patterns
5. **Diff check**: Skip repos where current version already matches SSM version
6. **PR creation**: Create branch → update files → commit+push → open GitHub PR

### Security Properties

- All external inputs (SSM values, config fields, file paths, repo names) validated in `internal/sanitize/`
- GitHub tokens passed via `GIT_ASKPASS` script — never appear in process args or error messages
- File scanning uses `filepath.WalkDir` (symlinks not followed) + `ValidateAbsPath` for escape detection
- PR content sanitized (control chars stripped, length limited) before GitHub API calls

### AWS Integrations

- **SSM Parameter Store**: `ssm:GetParameter` on `/aws/service/eks/optimized-ami/*`
- **Secrets Manager**: `secretsmanager:GetSecretValue` on the configured token secret ARN
- **Lambda**: Deployed as zip or container image, triggered by EventBridge

### GitHub Integration (go-github v60)

Uses `WithEnterpriseURLs` for both GitHub.com and GitHub Enterprise. The mock GitHub server in tests uses `/api/v3/` path prefix — match with `strings.HasSuffix(r.URL.Path, "/pulls")`.
