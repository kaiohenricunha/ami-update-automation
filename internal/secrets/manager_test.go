package secrets_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaiohenricunha/ami-update-automation/internal/secrets"
	"github.com/kaiohenricunha/ami-update-automation/pkg/types"
)

type mockSMClient struct {
	values map[string]string
	err    error
	calls  int
}

func (m *mockSMClient) GetSecretValue(_ context.Context, in *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	v, ok := m.values[*in.SecretId]
	if !ok {
		return nil, errors.New("secret not found")
	}
	return &secretsmanager.GetSecretValueOutput{
		SecretString: aws.String(v),
	}, nil
}

func TestGetSecretFound(t *testing.T) {
	client := &mockSMClient{values: map[string]string{"my/token": "ghp_test123"}}
	mgr := secrets.NewAWSSecretsManager(client)
	val, err := mgr.GetSecret(context.Background(), "my/token")
	require.NoError(t, err)
	assert.Equal(t, "ghp_test123", val)
}

func TestGetSecretCached(t *testing.T) {
	client := &mockSMClient{values: map[string]string{"my/token": "ghp_test123"}}
	mgr := secrets.NewAWSSecretsManager(client)

	_, err := mgr.GetSecret(context.Background(), "my/token")
	require.NoError(t, err)
	_, err = mgr.GetSecret(context.Background(), "my/token")
	require.NoError(t, err)

	assert.Equal(t, 1, client.calls, "expected only one API call due to caching")
}

func TestGetSecretEmpty(t *testing.T) {
	mgr := secrets.NewAWSSecretsManager(&mockSMClient{})
	_, err := mgr.GetSecret(context.Background(), "")
	require.Error(t, err)
	assert.ErrorIs(t, err, types.ErrInputValidation)
}

func TestGetSecretInvalidName(t *testing.T) {
	mgr := secrets.NewAWSSecretsManager(&mockSMClient{})
	_, err := mgr.GetSecret(context.Background(), "secret\x00name")
	require.Error(t, err)
	assert.ErrorIs(t, err, types.ErrInputValidation)
}

func TestGetSecretAPIError(t *testing.T) {
	client := &mockSMClient{err: errors.New("access denied")}
	mgr := secrets.NewAWSSecretsManager(client)
	_, err := mgr.GetSecret(context.Background(), "my/token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting secret")
}
