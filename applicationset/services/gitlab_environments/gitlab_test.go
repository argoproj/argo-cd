package gitlab_environments

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func writeMRListResponse(t *testing.T, w io.Writer) {
	f, err := os.Open("fixtures/gitlab_environment_list_response.json")
	if err != nil {
		t.Fatalf("error opening fixture file: %v", err)
	}

	if _, err = io.Copy(w, f); err != nil {
		t.Fatalf("error writing response: %v", err)
	}
}

func TestGitLabServiceCustomBaseURL(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	path := "/api/v4/projects/1/environments"

	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, path+"?per_page=100", r.URL.RequestURI())
		writeMRListResponse(t, w)
	})

	svc, err := NewGitLabService(context.Background(), "", server.URL, "1", "", "", false)
	assert.NoError(t, err)

	_, err = svc.List(context.Background())
	assert.NoError(t, err)
}

func TestGitLabServiceToken(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	path := "/api/v4/projects/1/environments"

	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "token-123", r.Header.Get("Private-Token"))
		writeMRListResponse(t, w)
	})

	svc, err := NewGitLabService(context.Background(), "token-123", server.URL, "1", "stopped", "", false)
	assert.NoError(t, err)

	_, err = svc.List(context.Background())
	assert.NoError(t, err)
}

func TestList(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	path := "/api/v4/projects/1/environments"

	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, path+"?per_page=100", r.URL.RequestURI())
		writeMRListResponse(t, w)
	})

	svc, err := NewGitLabService(context.Background(), "", server.URL, "1", "", "", false)
	assert.NoError(t, err)

	prs, err := svc.List(context.Background())
	assert.NoError(t, err)
	assert.Len(t, prs, 1)
	assert.Equal(t, prs[0].Name, "review/fix-foo")
}

func TestListWithState(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	path := "/api/v4/projects/1/environments"

	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, path+"?per_page=100&states=opened", r.URL.RequestURI())
		writeMRListResponse(t, w)
	})
	svc, err := NewGitLabService(context.Background(), "", server.URL, "1", "opened", "", false)
	assert.NoError(t, err)

	_, err = svc.List(context.Background())
	assert.NoError(t, err)
}
