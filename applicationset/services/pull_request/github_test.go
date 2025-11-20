package pull_request

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetGitHubPRLabelNames(t *testing.T) {
	tests := []struct {
		Name           string
		PullLabels     []githubLabel
		ExpectedResult []string
	}{
		{
			Name: "PR has labels",
			PullLabels: []githubLabel{
				{Name: githubv4.String("label1")},
				{Name: githubv4.String("label2")},
				{Name: githubv4.String("label3")},
			},
			ExpectedResult: []string{"label1", "label2", "label3"},
		},
		{
			Name:           "PR does not have labels",
			PullLabels:     []githubLabel{},
			ExpectedResult: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			labels := getGithubPRLabelNames(test.PullLabels)
			require.Equal(t, test.ExpectedResult, labels)
		})
	}
}

func TestGitHubListReturnsRepositoryNotFoundError(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/graphql", func(w http.ResponseWriter, _ *http.Request) {
		// Return error message to simulate repository not found
		_, _ = w.Write([]byte(`{"errors":[{"message":"Could not resolve to a Repository with the name 'nonexistent/nonexistent'."}]}`))
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
