package vcs_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaiohenricunha/ami-update-automation/internal/vcs"
	"github.com/kaiohenricunha/ami-update-automation/pkg/types"
)

func TestGitHubProviderCreatePR(t *testing.T) {
	// go-github appends /api/v3/ when using enterprise URLs.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v3/repos/my-org/my-repo/pulls" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"number":   42,
				"html_url": "https://github.com/my-org/my-repo/pull/42",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	provider := vcs.NewGitHubProvider(srv.URL + "/")
	result, err := provider.CreatePR(context.Background(), types.PRRequest{
		Owner: "my-org",
		Repo:  "my-repo",
		Title: "chore: update AMI",
		Body:  "Updates AMI version",
		Head:  "ami-update/1.29",
		Base:  "main",
	}, "test-token")
	require.NoError(t, err)
	assert.Equal(t, 42, result.Number)
	assert.Contains(t, result.URL, "pull/42")
}

func TestGitHubProviderPRExists(t *testing.T) {
	// go-github appends /api/v3/ when using enterprise URLs.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v3/repos/my-org/my-repo/pulls" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"number": 10, "html_url": "https://github.com/my-org/my-repo/pull/10"},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	provider := vcs.NewGitHubProvider(srv.URL + "/")
	exists, err := provider.PRExists(context.Background(), "my-org", "my-repo", "ami-update/1.29", "tok")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestGitHubProviderCreatePRInvalidOwner(t *testing.T) {
	provider := vcs.NewGitHubProvider("")
	_, err := provider.CreatePR(context.Background(), types.PRRequest{
		Owner: "bad owner!",
		Repo:  "repo",
	}, "tok")
	require.Error(t, err)
}
