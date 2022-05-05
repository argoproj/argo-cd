package scm_provider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/v2/pkg/apis/applicationset/v1alpha1"
)

func TestGiteaListRepos(t *testing.T) {
	cases := []struct {
		name, proto, url                        string
		hasError, allBranches, includeSubgroups bool
		branches                                []string
		filters                                 []v1alpha1.SCMProviderGeneratorFilter
	}{
		{
			name:     "blank protocol",
			allBranches: false,
			url:      "git@gitea.com:test-argocd/pr-test.git",
			branches: []string{"main"},
		},
		{
			name:  "ssh protocol",
			allBranches: false,
			proto: "ssh",
			url:   "git@gitea.com:test-argocd/pr-test.git",
		},
		{
			name:  "https protocol",
			allBranches: false,
			proto: "https",
			url:   "https://gitea.com/test-argocd/pr-test",
		},
		{
			name:     "other protocol",
			allBranches: false,
			proto:    "other",
			hasError: true,
		},
		{
			name:        "all branches",
			allBranches: true,
			url:         "git@gitea.com:test-argocd/pr-test.git",
			branches:    []string{"main"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			provider, _ := NewGiteaProvider(context.Background(), "test-argocd", "", "https://gitea.com/", c.allBranches, false)
			rawRepos, err := ListRepos(context.Background(), provider, c.filters, c.proto)
			if c.hasError {
				assert.NotNil(t, err)
			} else {
				checkRateLimit(t, err)
				assert.Nil(t, err)
				// Just check that this one project shows up. Not a great test but better thing nothing?
				repos := []*Repository{}
				branches := []string{}
				for _, r := range rawRepos {
					if r.Repository == "pr-test" {
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

func TestGiteaHasPath(t *testing.T) {
	host, _ := NewGiteaProvider(context.Background(), "gitea", "", "https://gitea.com/", false, false)
	repo := &Repository{
		Organization: "gitea",
		Repository:   "go-sdk",
		Branch:       "master",
	}
	ok, err := host.RepoHasPath(context.Background(), repo, "README.md")
	assert.Nil(t, err)
	assert.True(t, ok)

	// directory
	ok, err = host.RepoHasPath(context.Background(), repo, "gitea")
	assert.Nil(t, err)
	assert.True(t, ok)

	ok, err = host.RepoHasPath(context.Background(), repo, "notathing")
	assert.Nil(t, err)
	assert.False(t, ok)
}
