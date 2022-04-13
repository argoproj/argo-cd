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
			url:      "git@gitea.com:gitea/go-sdk.git",
			branches: []string{"master"},
		},
		{
			name:  "ssh protocol",
			allBranches: false,
			proto: "ssh",
			url:   "git@gitea.com:gitea/go-sdk.git",
		},
		{
			name:  "https protocol",
			allBranches: false,
			proto: "https",
			url:   "https://gitea.com/gitea/go-sdk",
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
			url:         "git@gitea.com:gitea/go-sdk.git",
			branches:    []string{"master", "release/v0.11", "release/v0.12", "release/v0.13", "release/v0.14", "release/v0.15"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			provider, _ := NewGiteaProvider(context.Background(), "gitea", "", "https://gitea.com/", c.allBranches, false)
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
					if r.Repository == "go-sdk" {
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
