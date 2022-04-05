package scm_provider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/v2/pkg/apis/applicationset/v1alpha1"
)

func TestGitlabListRepos(t *testing.T) {
	cases := []struct {
		name, proto, url                        string
		hasError, allBranches, includeSubgroups bool
		branches                                []string
		filters                                 []v1alpha1.SCMProviderGeneratorFilter
	}{
		{
			name:     "blank protocol",
			url:      "git@gitlab.com:test-argocd-proton/argocd.git",
			branches: []string{"master"},
		},
		{
			name:  "ssh protocol",
			proto: "ssh",
			url:   "git@gitlab.com:test-argocd-proton/argocd.git",
		},
		{
			name:  "https protocol",
			proto: "https",
			url:   "https://gitlab.com/test-argocd-proton/argocd.git",
		},
		{
			name:     "other protocol",
			proto:    "other",
			hasError: true,
		},
		{
			name:        "all branches",
			allBranches: true,
			url:         "git@gitlab.com:test-argocd-proton/argocd.git",
			branches:    []string{"master", "pipeline-1310077506"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			provider, _ := NewGitlabProvider(context.Background(), "test-argocd-proton", "", "", c.allBranches, c.includeSubgroups)
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
					if r.Repository == "argocd" {
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

func TestGitlabHasPath(t *testing.T) {
	host, _ := NewGitlabProvider(context.Background(), "test-argocd-proton", "", "", false, true)
	repo := &Repository{
		Organization: "test-argocd-proton",
		Repository:   "argocd",
		Branch:       "master",
	}

	cases := []struct {
		name, path string
		exists     bool
	}{
		{
			name:   "directory exists",
			path:   "argocd",
			exists: true,
		},
		{
			name:   "file exists",
			path:   "argocd/install.yaml",
			exists: true,
		},
		{
			name:   "directory does not exist",
			path:   "notathing",
			exists: false,
		},
		{
			name:   "file does not exist",
			path:   "argocd/notathing.yaml",
			exists: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ok, err := host.RepoHasPath(context.Background(), repo, c.path)
			assert.Nil(t, err)
			assert.Equal(t, c.exists, ok)
		})
	}
}
