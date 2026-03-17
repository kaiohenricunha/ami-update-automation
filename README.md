# ami-update-automation

A production-ready AWS Lambda that automates EKS node AMI release version updates across infrastructure repositories.

## What It Does

1. **Queries AWS SSM** for the latest recommended EKS-optimized AMI version (`/aws/service/eks/optimized-ami/<k8s-version>/<family>/recommended/release_version`)
2. **Scans infra repos** for `ami_release_version` occurrences across Terraform, Terragrunt, Pulumi, and Crossplane configurations
3. **Opens pull requests** updating the version in any repos where it has changed

## Supported IaC Formats

| Scanner | File Extensions | Pattern |
|---------|----------------|---------|
| Terraform | `.tf` | `ami_release_version = "..."` |
| Terragrunt | `.hcl` | `ami_release_version = "..."` |
| Pulumi | `.yaml`, `.yml`, `.go`, `.ts` | `amiReleaseVersion: "..."` / `ami_release_version = "..."` |
| Crossplane | `.yaml`, `.yml` | `ami_release_version: "..."` / `amiReleaseVersion: "..."` |

## Architecture

```
EventBridge (schedule)
    │
    └─► Lambda (ami-update-automation)
            │
            ├─► AWS SSM Parameter Store  (read latest AMI version)
            ├─► AWS Secrets Manager      (fetch GitHub token)
            ├─► GitHub API               (clone repo, open PR)
            └─► git (local exec)         (branch, commit, push)
```

## Configuration

The Lambda reads a `config.yaml` file at `/var/task/config.yaml` (or `$CONFIG_PATH`):

```yaml
github:
  token_secret_name: ami-automation/github-token  # Secrets Manager secret name

k8s_versions:
  - "1.29"
  - "1.30"

ami_family: amazon-linux-2  # default
concurrency: 5              # max parallel repo updates

repos:
  - owner: my-org
    repo: cloud-infra-prod
    branch: main
    scanners:
      - terraform
      - terragrunt
    paths:               # optional: only scan these subdirectories
      - modules/eks

pr_title: "chore: update EKS AMI to {{.NewVersion}} for k8s {{.K8sVersion}}"
```

## Deployment

### Prerequisites

- AWS account with Lambda, SSM, and Secrets Manager access
- GitHub token (classic PAT or fine-grained) with `repo` and `pull_request` permissions
- Terraform 1.5+

### Using the Terraform Module

```hcl
module "ami_updater" {
  source = "git::https://github.com/kaiohenricunha/ami-update-automation.git//deploy/terraform?ref=v1.0.0"

  function_name       = "ami-update-automation"
  lambda_zip_path     = "./lambda-arm64.zip"
  config_yaml_path    = "./config.yaml"
  github_token        = var.github_token
  lambda_architecture = "arm64"
}
```

Download the pre-built zip from [GitHub Releases](https://github.com/kaiohenricunha/ami-update-automation/releases) or build it yourself:

```bash
make zip-arm64  # arm64 (recommended for Lambda)
make zip        # amd64
```

### IAM Permissions Required

The Lambda execution role needs:

```json
{
  "Effect": "Allow",
  "Action": ["ssm:GetParameter"],
  "Resource": "arn:aws:ssm:*:*:parameter/aws/service/eks/optimized-ami/*"
}
```

```json
{
  "Effect": "Allow",
  "Action": ["secretsmanager:GetSecretValue"],
  "Resource": "<github-token-secret-arn>"
}
```

## Development

### Commands

```bash
# Build
make build          # linux/amd64
make build-arm64    # linux/arm64
make zip            # lambda.zip (amd64)
make zip-arm64      # lambda-arm64.zip

# Test
make test           # unit tests (fast, no infra)
make test-integration  # integration tests (real git + mock GitHub API, ~30s)
make test-security  # adversarial input tests (~15s)
make test-e2e       # e2e with LocalStack (needs Docker, ~60s)
make test-cover     # coverage report

# Lint
make lint           # golangci-lint
```

### Test Layers

| Layer | Speed | Infra | What's Real |
|-------|-------|-------|-------------|
| Unit | ~10s | None | Pure Go logic |
| Integration | ~30s | Local git | Real git ops, mock GitHub + SSM |
| Security | ~15s | None | Adversarial inputs to sanitize/scanner/vcs |
| E2E | ~60s | LocalStack (Docker) | Real AWS SDK wire format |

### Project Structure

```
cmd/lambda/         Lambda entrypoint
internal/
  ami/              AMI version resolver (SSM)
  config/           YAML config loader + validation
  handler/          Core orchestration logic
  logging/          Structured JSON logger (slog)
  sanitize/         Input validation + security checks
  scanner/          IaC file scanners (tf/hcl/yaml/go/ts)
  secrets/          AWS Secrets Manager client
  vcs/              Git operations + GitHub API
pkg/types/          Shared types + sentinel errors
test/
  fixtures/         Test IaC files
  integration/      Integration test helpers
  security/         Security/adversarial tests
  e2e/              End-to-end tests (LocalStack)
deploy/terraform/   Terraform module
```

## Security

- All external inputs (SSM values, config fields, file paths) are validated in `internal/sanitize/` before use
- GitHub tokens are passed via `GIT_ASKPASS` script — never as CLI arguments or in error messages
- File scanning uses `filepath.WalkDir` (does not follow symlinks) with symlink-escape validation
- Path traversal attempts are blocked by `ValidatePath` and `ValidateAbsPath`
- PR content is sanitized (control chars stripped, length limited) before calling the GitHub API

## License

Apache 2.0
