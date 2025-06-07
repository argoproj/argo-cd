package pull_request

import (
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/google/go-github/v69/github"
	"github.com/stretchr/testify/require"
)

func toPtr(s string) *string {
	return &s
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

type mockRoundTripper struct {
	lastRequest *http.Request
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	m.lastRequest = req

	// Return a dummy response
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{}`)),
		Header:     make(http.Header),
	}, nil
}

func TestNewGithubService(t *testing.T) {
	originalToken := os.Getenv("GITHUB_TOKEN")
	os.Unsetenv("GITHUB_TOKEN")
	defer t.Setenv("GITHUB_TOKEN", originalToken)

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

			service, err := NewGithubService(tt.token, tt.url, owner, repo, labels, httpClient)
			require.NoError(t, err)

			gs, ok := service.(*GithubService)
			require.True(t, ok)

			require.Equal(t, owner, gs.owner)
			require.Equal(t, repo, gs.repo)
			require.Equal(t, labels, gs.labels)

			_, _ = gs.List(t.Context())

			require.NotNil(t, mockTransport.lastRequest)
			require.Equal(t, tt.expectedHost, mockTransport.lastRequest.URL.Host)

			if tt.token != "" {
				require.Equal(t, "Bearer "+tt.token, mockTransport.lastRequest.Header.Get("Authorization"))
			} else {
				require.Empty(t, mockTransport.lastRequest.Header.Get("Authorization"))
			}
		})
	}
}
