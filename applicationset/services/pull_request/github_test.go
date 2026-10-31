package pull_request

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestGitHubListFiltersLabels(t *testing.T) {
	t.Parallel()
	cases := []struct {
		Name           string
		Labels         []string
		ExcludedLabels []string
		ExpectedPRs    []int64
	}{
		{
			Name:           "No filters returns all PRs",
			Labels:         []string{},
			ExcludedLabels: []string{},
			ExpectedPRs:    []int64{1, 2, 3},
		},
		{
			Name:           "Filter by required label",
			Labels:         []string{"ready"},
			ExcludedLabels: []string{},
			ExpectedPRs:    []int64{1, 2}, // PR 3 doesn't have "ready"
		},
		{
			Name:           "Exclude by label",
			Labels:         []string{},
			ExcludedLabels: []string{"stale"},
			ExpectedPRs:    []int64{1, 3}, // PR 2 has "stale"
		},
		{
			Name:           "Both include and exclude",
			Labels:         []string{"ready"},
			ExcludedLabels: []string{"stale"},
			ExpectedPRs:    []int64{1}, // PR 1 has "ready" but not "stale"
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			mux := http.NewServeMux()
			server := httptest.NewServer(mux)
			defer server.Close()

			path := "/api/v3/repos/myorg/myrepo/pulls"
			mux.HandleFunc(path, func(w http.ResponseWriter, _ *http.Request) {
				// Return 3 PRs with different labels:
				// PR 1: ready
				// PR 2: ready, stale
				// PR 3: wip
				response := `[
					{"number": 1, "title": "PR 1", "head": {"ref": "branch1", "sha": "abc123"}, "base": {"ref": "main"}, "labels": [{"name": "ready"}], "user": {"login": "user1"}},
					{"number": 2, "title": "PR 2", "head": {"ref": "branch2", "sha": "def456"}, "base": {"ref": "main"}, "labels": [{"name": "ready"}, {"name": "stale"}], "user": {"login": "user2"}},
					{"number": 3, "title": "PR 3", "head": {"ref": "branch3", "sha": "ghi789"}, "base": {"ref": "main"}, "labels": [{"name": "wip"}], "user": {"login": "user3"}}
				]`
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(response))
			})

			svc, err := NewGithubService("", server.URL, "myorg", "myrepo", c.Labels, c.ExcludedLabels, nil)
			require.NoError(t, err)

			prs, err := svc.List(t.Context())
			require.NoError(t, err)

			var gotPRNumbers []int64
			for _, pr := range prs {
				gotPRNumbers = append(gotPRNumbers, pr.Number)
			}

			assert.Equal(t, c.ExpectedPRs, gotPRNumbers)
		})
	}
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

	svc, err := NewGithubService("", server.URL, "nonexistent", "nonexistent", []string{}, []string{}, nil)
	require.NoError(t, err)

	prs, err := svc.List(t.Context())

	// Should return empty pull requests list
	assert.Empty(t, prs)

	// Should return RepositoryNotFoundError
	require.Error(t, err)
	assert.True(t, IsRepositoryNotFoundError(err), "Expected RepositoryNotFoundError but got: %v", err)
}

type mockRoundTripper struct {
	lastRequest *http.Request
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	m.lastRequest = req
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`[]`)),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

// The token must be sent with or without an enterprise URL (#21292 dropped it when a URL was set).
func TestNewGithubService(t *testing.T) {
	// NewGithubService falls back to GITHUB_TOKEN when no token is given.
	t.Setenv("GITHUB_TOKEN", "")

	tests := []struct {
		name         string
		token        string
		url          string
		expectedHost string
	}{
		{"No token, no URL", "", "", "api.github.com"},
		{"Token, no URL", "dummy-token", "", "api.github.com"},
		{"No token, URL", "", "https://github.example.com", "github.example.com"},
		{"Token and URL", "dummy-token", "https://github.example.com", "github.example.com"},
	}

	owner := "test-owner"
	repo := "test-repo"
	labels := []string{"test-label-1", "test-label-2"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTransport := &mockRoundTripper{}
			httpClient := &http.Client{Transport: mockTransport}

			service, err := NewGithubService(tt.token, tt.url, owner, repo, labels, nil, httpClient)
			require.NoError(t, err)

			gs, ok := service.(*GithubService)
			require.True(t, ok)
			assert.Equal(t, owner, gs.owner)
			assert.Equal(t, repo, gs.repo)
			assert.Equal(t, labels, gs.labels)

			_, err = gs.List(t.Context())
			require.NoError(t, err)

			require.NotNil(t, mockTransport.lastRequest)
			assert.Equal(t, tt.expectedHost, mockTransport.lastRequest.URL.Host)
			if tt.token != "" {
				assert.Equal(t, "Bearer "+tt.token, mockTransport.lastRequest.Header.Get("Authorization"))
			} else {
				assert.Empty(t, mockTransport.lastRequest.Header.Get("Authorization"))
			}
		})
	}
}
