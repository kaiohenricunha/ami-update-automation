// Package ami provides the AMIResolver interface and its AWS SSM implementation.
package ami

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/kaiohenricunha/ami-update-automation/internal/sanitize"
	"github.com/kaiohenricunha/ami-update-automation/pkg/types"
)

// AMIResolver resolves the latest EKS AMI release version for a given K8s version.
type AMIResolver interface {
	Resolve(ctx context.Context, k8sVersion, amiFamily string) (*types.AMIVersion, error)
}

// GetParameterAPIClient is the subset of SSM client used by SSMResolver.
type GetParameterAPIClient interface {
	GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
}

// SSMResolver fetches AMI versions from AWS SSM Parameter Store.
type SSMResolver struct {
	client GetParameterAPIClient
}

// NewSSMResolver creates an SSMResolver from an SSM client.
func NewSSMResolver(client GetParameterAPIClient) *SSMResolver {
	return &SSMResolver{client: client}
}

// Resolve queries SSM for the latest recommended AMI release version.
// Path: /aws/service/eks/optimized-ami/{k8sVersion}/{amiFamily}/recommended/release_version
func (r *SSMResolver) Resolve(ctx context.Context, k8sVersion, amiFamily string) (*types.AMIVersion, error) {
	if err := sanitize.ValidateK8sVersion(k8sVersion); err != nil {
		return nil, fmt.Errorf("resolve: %w", err)
	}
	if amiFamily == "" {
		amiFamily = "amazon-linux-2"
	}

	path := fmt.Sprintf("/aws/service/eks/optimized-ami/%s/%s/recommended/release_version", k8sVersion, amiFamily)

	out, err := r.client.GetParameter(ctx, &ssm.GetParameterInput{
		Name: &path,
	})
	if err != nil {
		// Wrap the error to match sentinel.
		return nil, fmt.Errorf("%w: %s: %w", types.ErrSSMParameterNotFound, path, err)
	}
	if out.Parameter == nil || out.Parameter.Value == nil {
		return nil, fmt.Errorf("%w: %s returned nil value", types.ErrSSMParameterNotFound, path)
	}

	version := *out.Parameter.Value
	if err := sanitize.ValidateAMIVersion(version); err != nil {
		return nil, fmt.Errorf("SSM returned invalid AMI version: %w", err)
	}

	return &types.AMIVersion{
		K8sVersion: k8sVersion,
		AMIFamily:  amiFamily,
		Version:    version,
	}, nil
}
