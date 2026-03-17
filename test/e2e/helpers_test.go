//go:build e2e

package e2e_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type recordedRequest struct {
	Method string
	Path   string
	Body   map[string]interface{}
}

func newMockGitHubServer(t *testing.T) (*httptest.Server, *[]recordedRequest) {
	t.Helper()
	var requests []recordedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if r.ContentLength > 0 {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		requests = append(requests, recordedRequest{Method: r.Method, Path: r.URL.Path, Body: body})

		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode([]interface{}{})
			return
		}
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"number":   1,
				"html_url": "http://github.example/pulls/1",
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{})
	}))
	t.Cleanup(srv.Close)
	return srv, &requests
}

// setupBareRepo creates a bare git repo seeded with the terraform fixture and returns its file:// URL.
func setupBareRepo(t *testing.T) string {
	t.Helper()
	fixtureDir := filepath.Join("..", "fixtures", "terraform")

	bare := t.TempDir()
	mustGit(t, bare, "init", "--bare")
	work := t.TempDir()
	mustGit(t, work, "init")
	mustGit(t, work, "remote", "add", "origin", bare)

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
	mustGit(t, work, "-c", "user.email=t@t.com", "-c", "user.name=T", "add", "-A")
	mustGit(t, work, "-c", "user.email=t@t.com", "-c", "user.name=T", "commit", "-m", "init")
	mustGit(t, work, "push", "origin", "HEAD:main")
	return "file://" + bare
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...) //nolint:gosec
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("git %v: %s", args, out)
		require.NoError(t, err)
	}
}
