//go:build e2e

// Package e2e contains end-to-end tests using LocalStack via testcontainers-go.
// Run with: go test -tags=e2e ./test/e2e/... -v
package e2e_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/localstack"

	amiresolver "github.com/kaiohenricunha/ami-update-automation/internal/ami"
	appconfig "github.com/kaiohenricunha/ami-update-automation/internal/config"
	"github.com/kaiohenricunha/ami-update-automation/internal/handler"
	"github.com/kaiohenricunha/ami-update-automation/internal/scanner"
	"github.com/kaiohenricunha/ami-update-automation/internal/secrets"
	"github.com/kaiohenricunha/ami-update-automation/internal/vcs"
	"github.com/kaiohenricunha/ami-update-automation/pkg/types"
)

func TestE2EFullFlow(t *testing.T) {
	ctx := context.Background()

	// Start LocalStack container.
	lsContainer, err := localstack.Run(ctx, "localstack/localstack:3.0")
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := lsContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate localstack: %v", err)
		}
	})

	// Build endpoint URL.
	host, err := lsContainer.Host(ctx)
	require.NoError(t, err)
	mappedPort, err := lsContainer.MappedPort(ctx, "4566/tcp")
	require.NoError(t, err)
	endpoint := "http://" + host + ":" + mappedPort.Port()

	// Build AWS config pointing at LocalStack.
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
		awsconfig.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(_, _ string, _ ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: endpoint, HostnameImmutable: true}, nil
			},
		)),
	)
	require.NoError(t, err)

	// Seed SSM parameter.
	ssmClient := ssm.NewFromConfig(awsCfg)
	paramPath := "/aws/service/eks/optimized-ami/1.29/amazon-linux-2/recommended/release_version"
	_, err = ssmClient.PutParameter(ctx, &ssm.PutParameterInput{
		Name:      aws.String(paramPath),
		Value:     aws.String("1.29.5-20240701"),
		Type:      "String",
		Overwrite: aws.Bool(true),
	})
	require.NoError(t, err)

	// Seed GitHub token in Secrets Manager.
	smClient := secretsmanager.NewFromConfig(awsCfg)
	_, err = smClient.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String("test/github-token"),
		SecretString: aws.String("ghp_localstack_test"),
	})
	require.NoError(t, err)

	// Setup local bare git repo with terraform fixture.
	bareURL := setupBareRepo(t)

	// Mock GitHub API server.
	ghSrv, requests := newMockGitHubServer(t)

	cfg := &appconfig.Config{
		GitHub:      appconfig.GitHubConfig{TokenSecretName: "test/github-token", APIURL: ghSrv.URL + "/"},
		K8sVersions: []string{"1.29"},
		AMIFamily:   "amazon-linux-2",
		Repos: []types.RepoTarget{{
			Owner: "my-org", Repo: "infra", Branch: "main",
			Scanners: []string{"terraform"},
		}},
		Concurrency: 1,
		PRTitle:     "chore: update AMI to {{.NewVersion}}",
	}

	fvcs := &fileVCS{bareURL: bareURL, gh: vcs.NewGitHubProvider(ghSrv.URL + "/")}

	h := handler.New(
		cfg,
		amiresolver.NewSSMResolver(ssmClient),
		scanner.NewRegistry(),
		fvcs,
		secrets.NewAWSSecretsManager(smClient),
		slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	)

	result, err := h.HandleEvent(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Updated)
	assert.Len(t, result.PRURLs, 1)

	// Verify a PR was created against the mock GitHub server.
	prCreated := false
	for _, r := range *requests {
		if r.Method == "POST" && r.Path != "" {
			prCreated = true
			break
		}
	}
	assert.True(t, prCreated, "expected a PR creation request")
}

// fileVCS clones via file:// URL; delegates all other ops to GitHubProvider.
type fileVCS struct {
	bareURL string
	gh      *vcs.GitHubProvider
}

func (f *fileVCS) Clone(ctx context.Context, _, _, _ string) (string, error) {
	dir, err := os.MkdirTemp("", "e2e-clone-*")
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth=1", f.bareURL, dir) //nolint:gosec
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(dir)
		return "", fmt.Errorf("clone: %w: %s", err, out)
	}
	return dir, nil
}
func (f *fileVCS) CreateBranch(ctx context.Context, d, b string) error {
	return f.gh.CreateBranch(ctx, d, b)
}
func (f *fileVCS) CommitAndPush(ctx context.Context, d, b, m, t string) error {
	return f.gh.CommitAndPush(ctx, d, b, m, t)
}
func (f *fileVCS) CreatePR(ctx context.Context, r types.PRRequest, t string) (*types.PRResult, error) {
	return f.gh.CreatePR(ctx, r, t)
}
func (f *fileVCS) PRExists(ctx context.Context, o, r, b, t string) (bool, error) {
	return f.gh.PRExists(ctx, o, r, b, t)
}
func (f *fileVCS) Cleanup(d string) error { return os.RemoveAll(d) }
