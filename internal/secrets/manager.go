// Package secrets provides the SecretsProvider interface and AWS Secrets Manager implementation.
package secrets

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	"github.com/kaiohenricunha/ami-update-automation/pkg/types"
)

// SecretsProvider retrieves secret values by name.
type SecretsProvider interface {
	GetSecret(ctx context.Context, secretName string) (string, error)
}

// GetSecretValueAPIClient is the subset of Secrets Manager client used here.
type GetSecretValueAPIClient interface {
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

// AWSSecretsManager fetches secrets from AWS Secrets Manager with in-memory caching.
type AWSSecretsManager struct {
	client GetSecretValueAPIClient
	mu     sync.Mutex
	cache  map[string]string
}

// NewAWSSecretsManager creates a secrets manager wrapping the given client.
func NewAWSSecretsManager(client GetSecretValueAPIClient) *AWSSecretsManager {
	return &AWSSecretsManager{
		client: client,
		cache:  make(map[string]string),
	}
}

// GetSecret returns the secret string value, using the cache for repeat calls.
func (m *AWSSecretsManager) GetSecret(ctx context.Context, secretName string) (string, error) {
	if secretName == "" {
		return "", fmt.Errorf("%w: secret name is empty", types.ErrInputValidation)
	}
	if strings.ContainsAny(secretName, "\x00\n\r") {
		return "", fmt.Errorf("%w: secret name contains invalid characters", types.ErrInputValidation)
	}

	m.mu.Lock()
	if v, ok := m.cache[secretName]; ok {
		m.mu.Unlock()
		return v, nil
	}
	m.mu.Unlock()

	out, err := m.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: &secretName,
	})
	if err != nil {
		return "", fmt.Errorf("getting secret %q: %w", secretName, err)
	}
	if out.SecretString == nil {
		return "", fmt.Errorf("secret %q has no string value", secretName)
	}

	value := *out.SecretString
	m.mu.Lock()
	m.cache[secretName] = value
	m.mu.Unlock()

	return value, nil
}
