package pull_request

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/go-github/v69/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func toPtr(s string) *string {
	return &s
}

func mockGitHubPRListHandler(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)

	nonExistingPath := "/api/v3/repos/nonexistent/nonexistent/pulls"

	mux.HandleFunc(nonExistingPath, func(w http.ResponseWriter, _ *http.Request) {
		// Return 404 status to simulate repository not found
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message": "404 Project Not Found"}`))
	})

	path := "/api/v3/repos/octocat/Hello-World/pulls"

	mux.HandleFunc(path, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		writeGitHubPRListResponse(t, w)
	})

	return server
}

func writeGitHubPRListResponse(t *testing.T, w io.Writer) {
	t.Helper()
	f, err := os.Open("fixtures/github_pr_list_response.json")
	require.NoErrorf(t, err, "error opening fixture file: %v", err)

	_, err = io.Copy(w, f)
	require.NoErrorf(t, err, "error writing response: %v", err)
}

func TestContainLabels(t *testing.T) {
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
				{Name: toPtr("label1")},
				{Name: toPtr("label2")},
				{Name: toPtr("label3")},
			},
			Expect: true,
		},
		{
			Name:   "Not match labels",
			Labels: []string{"label1", "label4"},
			PullLabels: []*github.Label{
				{Name: toPtr("label1")},
				{Name: toPtr("label2")},
				{Name: toPtr("label3")},
			},
			Expect: false,
		},
		{
			Name:   "No specify",
			Labels: []string{},
			PullLabels: []*github.Label{
				{Name: toPtr("label1")},
				{Name: toPtr("label2")},
				{Name: toPtr("label3")},
			},
			Expect: true,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			got := containLabels(c.Labels, c.PullLabels)
			require.Equal(t, got, c.Expect)
		})
	}
}

func TestGetGitHubPRLabelNames(t *testing.T) {
	Tests := []struct {
		Name           string
		PullLabels     []*github.Label
		ExpectedResult []string
	}{
		{
			Name: "PR has labels",
			PullLabels: []*github.Label{
				{Name: toPtr("label1")},
				{Name: toPtr("label2")},
				{Name: toPtr("label3")},
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
			labels := getGithubPRLabelNames(test.PullLabels)
			require.Equal(t, test.ExpectedResult, labels)
		})
	}
}

func TestGithubList(t *testing.T) {
	server := mockGitHubPRListHandler(t)
	defer server.Close()

	svc, err := NewGithubService("", server.URL, "octocat", "Hello-World", []string{}, nil)
	require.NoError(t, err)

	prs, err := svc.List(t.Context())

	assert.Len(t, prs, 1)

	pr := prs[0]

	assert.Equal(t, 1347, pr.Number)
	assert.Equal(t, "Amazing new feature", pr.Title)
	assert.Equal(t, "master", pr.TargetBranch)
	assert.Equal(t, "6dcb09b5b57875f334f61aebed695e2e4193db5e", pr.HeadSHA)
	assert.Len(t, pr.Labels, 1)
	assert.Equal(t, "bug", pr.Labels[0])
	assert.Equal(t, "octocat", pr.Author)
	assert.Equal(t, "2011-01-26T19:01:12Z", pr.CreatedAt.Format(time.RFC3339))
	assert.Equal(t, "2011-01-26T19:01:12Z", pr.UpdatedAt.Format(time.RFC3339))

}

func TestGitHubListReturnsRepositoryNotFoundError(t *testing.T) {
	server := mockGitHubPRListHandler(t)
	defer server.Close()

	svc, err := NewGithubService("", server.URL, "nonexistent", "nonexistent", []string{}, nil)
	require.NoError(t, err)

	prs, err := svc.List(t.Context())

	// Should return empty pull requests list
	assert.Empty(t, prs)

	// Should return RepositoryNotFoundError
	require.Error(t, err)
	assert.True(t, IsRepositoryNotFoundError(err), "Expected RepositoryNotFoundError but got: %v", err)
}
