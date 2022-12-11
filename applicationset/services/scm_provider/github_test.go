package scm_provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/test"
)

func TestGithubListRepos(t *testing.T) {
	cases := []struct {
		name, proto, url      string
		hasError, allBranches bool
		branches              []string
		filters               []v1alpha1.SCMProviderGeneratorFilter
	}{
		{
			name:     "blank protocol",
			url:      "git@github.com:argoproj/argo-cd.git",
			branches: []string{"master"},
		},
		{
			name:  "ssh protocol",
			proto: "ssh",
			url:   "git@github.com:argoproj/argo-cd.git",
		},
		{
			name:  "https protocol",
			proto: "https",
			url:   "https://github.com/argoproj/argo-cd.git",
		},
		{
			name:     "other protocol",
			proto:    "other",
			hasError: true,
		},
		{
			name:        "all branches",
			allBranches: true,
			url:         "git@github.com:argoproj/argo-cd.git",
			branches:    []string{"master"},
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		test.GitHubMockHandler(t)(w, r)
	}))
	defer ts.Close()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			provider, _ := NewGithubProvider(context.Background(), "argoproj", "", ts.URL, c.allBranches)
			rawRepos, err := ListRepos(context.Background(), provider, c.filters, c.proto)
			if c.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// Just check that this one project shows up. Not a great test but better thing nothing?
				repos := []*Repository{}
				branches := []string{}
				for _, r := range rawRepos {
					if r.Repository == "argo-cd" {
						repos = append(repos, r)
						branches = append(branches, r.Branch)
					}
				}
				assert.NotEmpty(t, repos)
				assert.Equal(t, c.url, repos[0].URL)
				for _, b := range c.branches {
					assert.Contains(t, branches, b)
				}
			}
		})
	}
}

func TestGithubHasPath(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		test.GitHubMockHandler(t)(w, r)
	}))
	defer ts.Close()
	host, _ := NewGithubProvider(context.Background(), "argoproj", "", ts.URL, false)
	repo := &Repository{
		Organization: "argoproj",
		Repository:   "argo-cd",
		Branch:       "master",
	}
	ok, err := host.RepoHasPath(context.Background(), repo, "pkg/")
	assert.Nil(t, err)
	assert.True(t, ok)

	ok, err = host.RepoHasPath(context.Background(), repo, "notathing/")
	assert.Nil(t, err)
	assert.False(t, ok)
}

func TestGithubGetBranches(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		test.GitHubMockHandler(t)(w, r)
	}))
	defer ts.Close()
	host, _ := NewGithubProvider(context.Background(), "argoproj", "", ts.URL, false)
	repo := &Repository{
		Organization: "argoproj",
		Repository:   "argo-cd",
		Branch:       "master",
	}
	repos, err := host.GetBranches(context.Background(), repo)
	if err != nil {
		assert.NoError(t, err)
	} else {
		assert.Equal(t, repos[0].Branch, "master")
	}
	//Branch Doesn't exists instead of error will return no error
	repo2 := &Repository{
		Organization: "argoproj",
		Repository:   "applicationset",
		Branch:       "main",
	}
	_, err = host.GetBranches(context.Background(), repo2)
	assert.NoError(t, err)

	// Get all branches
	host.allBranches = true
	repos, err = host.GetBranches(context.Background(), repo)
	if err != nil {
		assert.NoError(t, err)
	} else {
		// considering master  branch to  exist.
		assert.Equal(t, len(repos), 1)
	}
}
