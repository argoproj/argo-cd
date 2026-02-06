package pull_request

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writePRListResponse(t *testing.T, w io.Writer) {
	t.Helper()
	f, err := os.Open("fixtures/sourcecraft_pr_list_response.json")
	require.NoErrorf(t, err, "error opening fixture file: %v", err)
	defer f.Close()

	_, err = io.Copy(w, f)
	require.NoErrorf(t, err, "error writing response: %v", err)
}

func writeBranchResponse(t *testing.T, w io.Writer, branchName string) {
	t.Helper()
	var filename string
	switch branchName {
	case "feature-branch":
		filename = "fixtures/sourcecraft_branch_response.json"
	case "bugfix-auth":
		filename = "fixtures/sourcecraft_branch_response2.json"
	default:
		// Return 404 for unknown branches
		return
	}

	f, err := os.Open(filename)
	require.NoErrorf(t, err, "error opening fixture file: %v", err)
	defer f.Close()

	_, err = io.Copy(w, f)
	require.NoErrorf(t, err, "error writing response: %v", err)
}

func sourcecraftMockHandler(t *testing.T) func(http.ResponseWriter, *http.Request) {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.RequestURI {
		case "/repos/test-org/test-repo/pulls?":
			writePRListResponse(t, w)
		case "/repos/test-org/test-repo/branches?filter=feature-branch":
			writeBranchResponse(t, w, "feature-branch")
		case "/repos/test-org/test-repo/branches?filter=bugfix-auth":
			writeBranchResponse(t, w, "bugfix-auth")
		case "/repos/test-org/test-repo/branches?filter=old-feature":
			// Return empty response for closed PR branch
			writeBranchResponse(t, w, "old-feature")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func TestSourceCraftList(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sourcecraftMockHandler(t)(w, r)
	}))
	defer ts.Close()

	host, err := NewSourceCraftService("test-token", ts.URL, "test-org", "test-repo", false)
	require.NoError(t, err)

	prs, err := host.List(t.Context())
	require.NoError(t, err)
	assert.Len(t, prs, 2) // Only open PRs should be returned

	// First PR
	assert.Equal(t, int64(1), prs[0].Number)
	assert.Equal(t, "Add new feature", prs[0].Title)
	assert.Equal(t, "feature-branch", prs[0].Branch)
	assert.Equal(t, "main", prs[0].TargetBranch)
	assert.Equal(t, "abc123def456789012345678901234567890abcd", prs[0].HeadSHA)
	assert.Equal(t, "testuser", prs[0].Author)

	// Second PR
	assert.Equal(t, int64(2), prs[1].Number)
	assert.Equal(t, "Fix bug in authentication", prs[1].Title)
	assert.Equal(t, "bugfix-auth", prs[1].Branch)
	assert.Equal(t, "main", prs[1].TargetBranch)
	assert.Equal(t, "def456abc123789012345678901234567890defg", prs[1].HeadSHA)
	assert.Equal(t, "anotheruser", prs[1].Author)
}

func TestSourceCraftServiceCustomBaseURL(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	path := "/repos/test-org/test-repo/pulls"

	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, path+"?", r.URL.RequestURI())
		writePRListResponse(t, w)
	})

	mux.HandleFunc("/repos/test-org/test-repo/branches", func(w http.ResponseWriter, r *http.Request) {
		branchFilter := r.URL.Query().Get("filter")
		writeBranchResponse(t, w, branchFilter)
	})

	svc, err := NewSourceCraftService("test-token", server.URL, "test-org", "test-repo", false)
	require.NoError(t, err)

	prs, err := svc.List(t.Context())
	require.NoError(t, err)
	assert.Len(t, prs, 2)
}

func TestSourceCraftServiceToken(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	path := "/repos/test-org/test-repo/pulls"
	expectedToken := "test-token-123"

	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer "+expectedToken, r.Header.Get("Authorization"))
		writePRListResponse(t, w)
	})

	mux.HandleFunc("/repos/test-org/test-repo/branches", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer "+expectedToken, r.Header.Get("Authorization"))
		branchFilter := r.URL.Query().Get("filter")
		writeBranchResponse(t, w, branchFilter)
	})

	svc, err := NewSourceCraftService(expectedToken, server.URL, "test-org", "test-repo", false)
	require.NoError(t, err)

	prs, err := svc.List(t.Context())
	require.NoError(t, err)
	assert.NotEmpty(t, prs)
}

func TestSourceCraftServiceWithEnvToken(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	// Set environment variable
	envToken := "env-test-token"
	t.Setenv("SOURCECRAFT_TOKEN", envToken)

	path := "/repos/test-org/test-repo/pulls"

	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer "+envToken, r.Header.Get("Authorization"))
		writePRListResponse(t, w)
	})

	mux.HandleFunc("/repos/test-org/test-repo/branches", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer "+envToken, r.Header.Get("Authorization"))
		branchFilter := r.URL.Query().Get("filter")
		writeBranchResponse(t, w, branchFilter)
	})

	// Pass empty token to trigger env variable usage
	svc, err := NewSourceCraftService("", server.URL, "test-org", "test-repo", false)
	require.NoError(t, err)

	prs, err := svc.List(t.Context())
	require.NoError(t, err)
	assert.NotEmpty(t, prs)
}

func TestSourceCraftListFiltersClosed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sourcecraftMockHandler(t)(w, r)
	}))
	defer ts.Close()

	svc, err := NewSourceCraftService("test-token", ts.URL, "test-org", "test-repo", false)
	require.NoError(t, err)

	prs, err := svc.List(t.Context())
	require.NoError(t, err)

	// Verify closed PR is filtered out
	for _, pr := range prs {
		assert.NotEqual(t, int64(3), pr.Number, "Closed PR should not be in the list")
		assert.NotEqual(t, "Closed PR", pr.Title, "Closed PR should not be in the list")
	}
}

func TestSourceCraftListSkipsBranchNotFound(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	path := "/repos/test-org/test-repo/pulls"

	mux.HandleFunc(path, func(w http.ResponseWriter, _ *http.Request) {
		writePRListResponse(t, w)
	})

	mux.HandleFunc("/repos/test-org/test-repo/branches", func(w http.ResponseWriter, r *http.Request) {
		branchFilter := r.URL.Query().Get("filter")
		// Only return branch for feature-branch, return 404 for others
		if branchFilter == "feature-branch" {
			writeBranchResponse(t, w, branchFilter)
		} else {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message": "Branch not found"}`))
		}
	})

	svc, err := NewSourceCraftService("test-token", server.URL, "test-org", "test-repo", false)
	require.NoError(t, err)

	prs, err := svc.List(t.Context())
	require.NoError(t, err)

	// Should only return PR with valid branch
	assert.Len(t, prs, 1)
	assert.Equal(t, int64(1), prs[0].Number)
	assert.Equal(t, "feature-branch", prs[0].Branch)
}

func TestSourceCraftListReturnsRepositoryNotFoundError(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	path := "/repos/nonexistent/nonexistent/pulls"

	mux.HandleFunc(path, func(w http.ResponseWriter, _ *http.Request) {
		// Return 404 status to simulate repository not found
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message": "404 Repository Not Found"}`))
	})

	svc, err := NewSourceCraftService("test-token", server.URL, "nonexistent", "nonexistent", false)
	require.NoError(t, err)

	prs, err := svc.List(t.Context())

	// Should return empty pull requests list
	assert.Empty(t, prs)

	// Should return RepositoryNotFoundError
	require.Error(t, err)
	assert.True(t, IsRepositoryNotFoundError(err), "Expected RepositoryNotFoundError but got: %v", err)
}

func TestSourceCraftListHandlesInvalidPRSlug(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	path := "/repos/test-org/test-repo/pulls"

	mux.HandleFunc(path, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return PR with invalid slug (not a number)
		_, _ = w.Write([]byte(`{
			"pull_requests": [
				{
					"id": "test-id",
					"slug": "invalid-slug",
					"author": {"id": "user-id", "slug": "testuser"},
					"updated_by": {"id": "user-id", "slug": "testuser"},
					"title": "Test PR",
					"repository": {"id": "repo-id", "slug": "test-repo"},
					"source_branch": "test-branch",
					"target_branch": "main",
					"status": "open",
					"created_at": "2026-02-03T14:55:53.584174086Z",
					"updated_at": "2026-02-03T14:55:53.584174086Z"
				}
			],
			"next_page_token": ""
		}`))
	})

	svc, err := NewSourceCraftService("test-token", server.URL, "test-org", "test-repo", false)
	require.NoError(t, err)

	_, err = svc.List(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid syntax", "Should return error for invalid PR slug")
}

func TestSourceCraftServiceInsecure(t *testing.T) {
	// Create a test server with TLS
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sourcecraftMockHandler(t)(w, r)
	}))
	defer ts.Close()

	// Test with insecure=true (should work with self-signed cert)
	svc, err := NewSourceCraftService("test-token", ts.URL, "test-org", "test-repo", true)
	require.NoError(t, err)

	prs, err := svc.List(t.Context())
	require.NoError(t, err)
	assert.Len(t, prs, 2)
}

func TestSourceCraftServiceSecure(t *testing.T) {
	// Create a test server with TLS
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sourcecraftMockHandler(t)(w, r)
	}))
	defer ts.Close()

	// Test with insecure=false (should fail with self-signed cert)
	svc, err := NewSourceCraftService("test-token", ts.URL, "test-org", "test-repo", false)
	require.NoError(t, err)

	_, err = svc.List(t.Context())
	// Should fail due to certificate verification
	require.Error(t, err)
}
