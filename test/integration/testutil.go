//go:build integration

// Package integration provides test utilities for integration tests.
package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/stretchr/testify/require"
)

// RecordedRequest holds a captured HTTP request body and path.
type RecordedRequest struct {
	Method string
	Path   string
	Body   map[string]interface{}
}

// SetupBareRepo creates a bare git repo seeded with fixtureDir contents and returns its file:// URL.
func SetupBareRepo(t *testing.T, fixtureDir string) string {
	t.Helper()
	bare := t.TempDir()
	runGitCmd(t, bare, "init", "--bare")

	// Create a working copy to seed the bare repo.
	work := t.TempDir()
	runGitCmd(t, work, "init")
	runGitCmd(t, work, "remote", "add", "origin", bare)

	// Copy fixture files.
	if fixtureDir != "" {
		entries, err := os.ReadDir(fixtureDir)
		require.NoError(t, err)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			data, err := os.ReadFile(filepath.Join(fixtureDir, e.Name()))
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(work, e.Name()), data, 0o600))
		}
	} else {
		require.NoError(t, os.WriteFile(filepath.Join(work, "README.md"), []byte("# test\n"), 0o600))
	}

	runGitCmd(t, work, "-c", "user.email=test@test.com", "-c", "user.name=Test", "add", "-A")
	runGitCmd(t, work, "-c", "user.email=test@test.com", "-c", "user.name=Test", "commit", "-m", "initial")
	runGitCmd(t, work, "push", "origin", "HEAD:main")

	return "file://" + bare
}

// NewMockGitHubServer starts an httptest server that handles GitHub PR endpoints.
func NewMockGitHubServer(t *testing.T) (*httptest.Server, *[]RecordedRequest) {
	t.Helper()
	var requests []RecordedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if r.ContentLength > 0 {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		requests = append(requests, RecordedRequest{
			Method: r.Method,
			Path:   r.URL.Path,
			Body:   body,
		})

		w.Header().Set("Content-Type", "application/json")
		isPulls := strings.HasSuffix(r.URL.Path, "/pulls")
		switch {
		case r.Method == http.MethodGet && isPulls:
			// PRExists — return empty list (no existing PR).
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{})

		case r.Method == http.MethodPost && isPulls:
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"number":   1,
				"html_url": "http://github.example/pulls/1",
			})

		default:
			// Return 200 for any other endpoint (e.g., rate limit checks).
			_ = json.NewEncoder(w).Encode(map[string]interface{}{})
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &requests
}


// NewMockSSMClient returns a mock SSM client backed by a version map.
func NewMockSSMClient(versions map[string]string) *MockSSMClient {
	return &MockSSMClient{versions: versions}
}

// MockSSMClient implements ami.GetParameterAPIClient for testing.
type MockSSMClient struct {
	versions map[string]string
}

// GetParameter returns a version from the mock map.
func (m *MockSSMClient) GetParameter(_ context.Context, in *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	v, ok := m.versions[*in.Name]
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

func runGitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...) //nolint:gosec
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("git %v: %s", args, out)
	}
	require.NoError(t, err)
}
