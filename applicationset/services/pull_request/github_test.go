package pull_request

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-github/v69/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContainLabels(t *testing.T) {
	t.Parallel()
	cases := []struct {
		Name       string
		Labels     []string
		PullLabels []*github.Label
		Expect     bool
	}{
		{
			Name:   "Match labels",
			Labels: []string{"label1", "label2"},
			PullLabels: []*github.Label{
				{Name: new("label1")},
				{Name: new("label2")},
				{Name: new("label3")},
			},
			Expect: true,
		},
		{
			Name:   "Not match labels",
			Labels: []string{"label1", "label4"},
			PullLabels: []*github.Label{
				{Name: new("label1")},
				{Name: new("label2")},
				{Name: new("label3")},
			},
			Expect: false,
		},
		{
			Name:   "No specify",
			Labels: []string{},
			PullLabels: []*github.Label{
				{Name: new("label1")},
				{Name: new("label2")},
				{Name: new("label3")},
			},
			Expect: true,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			got := containLabels(c.Labels, c.PullLabels)
			require.Equal(t, got, c.Expect)
		})
	}
}

func TestGetGitHubPRLabelNames(t *testing.T) {
	t.Parallel()
	Tests := []struct {
		Name           string
		PullLabels     []*github.Label
		ExpectedResult []string
	}{
		{
			Name: "PR has labels",
			PullLabels: []*github.Label{
				{Name: new("label1")},
				{Name: new("label2")},
				{Name: new("label3")},
			},
			ExpectedResult: []string{"label1", "label2", "label3"},
		},
		{
			Name:           "PR does not have labels",
			PullLabels:     []*github.Label{},
			ExpectedResult: nil,
		},
	}
	for _, test := range Tests {
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()
			labels := getGithubPRLabelNames(test.PullLabels)
			require.Equal(t, test.ExpectedResult, labels)
		})
	}
}

func TestGitHubListDraft(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	path := "/api/v3/repos/test/repo/pulls"
	mux.HandleFunc(path, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[
			{
				"number": 1,
				"title": "Draft pull request",
				"draft": true,
				"head": {"ref": "draft", "sha": "draft-sha"},
				"base": {"ref": "main"},
				"user": {"login": "draft-author"},
				"labels": []
			},
			{
				"number": 2,
				"title": "Ready pull request",
				"draft": false,
				"head": {"ref": "ready", "sha": "ready-sha"},
				"base": {"ref": "main"},
				"user": {"login": "ready-author"},
				"labels": []
			}
		]`))
	})

	svc, err := NewGithubService("", server.URL, "test", "repo", []string{}, nil)
	require.NoError(t, err)

	prs, err := svc.List(t.Context())
	require.NoError(t, err)
	require.Len(t, prs, 2)
	assert.True(t, prs[0].Draft)
	assert.False(t, prs[1].Draft)
}

func TestGitHubListReturnsRepositoryNotFoundError(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	path := "/api/v3/repos/nonexistent/nonexistent/pulls"

	mux.HandleFunc(path, func(w http.ResponseWriter, _ *http.Request) {
		// Return 404 status to simulate repository not found
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message": "404 Project Not Found"}`))
	})

	svc, err := NewGithubService("", server.URL, "nonexistent", "nonexistent", []string{}, nil)
	require.NoError(t, err)

	prs, err := svc.List(t.Context())

	// Should return empty pull requests list
	assert.Empty(t, prs)

	// Should return RepositoryNotFoundError
	require.Error(t, err)
	assert.True(t, IsRepositoryNotFoundError(err), "Expected RepositoryNotFoundError but got: %v", err)
}
