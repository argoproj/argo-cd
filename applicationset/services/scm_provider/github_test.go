package scm_provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	github "github.com/google/go-github/v66/github"
)

func githubMockHandler(t *testing.T) func(http.ResponseWriter, *http.Request) {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.RequestURI {
		case "/api/v3/orgs/argoproj/repos?per_page=100":
			repo := &github.Repository{
				ID:              github.Int64(1296269),
				NodeID:          github.String("MDEwOlJlcG9zaXRvcnkxMjk2MjY5"),
				Name:            github.String("argo-cd"),
				FullName:        github.String("argoproj/argo-cd"),
				Description:     github.String("This your first repo!"),
				Homepage:        github.String("https://github.com"),
				DefaultBranch:   github.String("master"),
				ForksCount:      github.Int(9),
				StargazersCount: github.Int(80),
				WatchersCount:   github.Int(80),
				Topics:          []string{"argoproj", "atom", "electron", "api"},
				Owner: &github.User{
					Login:     github.String("argoproj"),
					ID:        github.Int64(1),
					NodeID:    github.String("MDQ6VXNlcjE="),
					Type:      github.String("User"),
					SiteAdmin: github.Bool(false),
				},
				CreatedAt: &github.Timestamp{Time: time.Date(2011, 1, 26, 19, 1, 12, 0, time.UTC)},
				UpdatedAt: &github.Timestamp{Time: time.Date(2011, 1, 26, 19, 14, 43, 0, time.UTC)},
				PushedAt:  &github.Timestamp{Time: time.Date(2011, 1, 26, 19, 6, 43, 0, time.UTC)},
			}

			if err := json.NewEncoder(w).Encode([]*github.Repository{repo}); err != nil {
				t.Errorf("Failed to encode repository response: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

		case "/api/v3/repos/argoproj/argo-cd/branches?per_page=100":
			branch := &github.Branch{
				Name: github.String("master"),
				Commit: &github.RepositoryCommit{
					SHA: github.String("c5b97d5ae6c19d5c5df71a34c7fbeeda2479ccbc"),
				},
				Protected: github.Bool(true),
				Protection: &github.Protection{
					RequiredStatusChecks: &github.RequiredStatusChecks{
						// Enforcement: github.String("non_admins"),
						Contexts: &[]string{"ci-test", "linter"},
					},
				},
			}

			if err := json.NewEncoder(w).Encode([]*github.Branch{branch}); err != nil {
				t.Errorf("Failed to encode branch response: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

		case "/api/v3/repos/argoproj/argo-cd/contents/pkg?ref=master":
			content := &github.RepositoryContent{
				Type:        github.String("file"),
				Encoding:    github.String("base64"),
				Size:        github.Int(5362),
				Name:        github.String("pkg/"),
				Path:        github.String("pkg/"),
				Content:     github.String("encoded content ..."),
				SHA:         github.String("3d21ec53a331a6f037a91c368710b99387d012c1"),
				URL:         github.String("https://api.github.com/repos/octokit/octokit.rb/contents/README.md"),
				GitURL:      github.String("https://api.github.com/repos/octokit/octokit.rb/git/blobs/3d21ec53a331a6f037a91c368710b99387d012c1"),
				HTMLURL:     github.String("https://github.com/octokit/octokit.rb/blob/master/README.md"),
				DownloadURL: github.String("https://raw.githubusercontent.com/octokit/octokit.rb/master/README.md"),
			}

			if err := json.NewEncoder(w).Encode(content); err != nil {
				t.Errorf("Failed to encode content response: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

		case "/api/v3/repos/argoproj/argo-cd/branches/master":
			branch := &github.Branch{
				Name: github.String("master"),
				Commit: &github.RepositoryCommit{
					SHA: github.String("c5b97d5ae6c19d5c5df71a34c7fbeeda2479ccbc"),
				},
				Protected: github.Bool(true),
				Protection: &github.Protection{
					RequiredStatusChecks: &github.RequiredStatusChecks{
						//Enforcement: github.String("non_admins"),
						Contexts: &[]string{"ci-test", "linter"},
					},
				},
			}

			if err := json.NewEncoder(w).Encode(branch); err != nil {
				t.Errorf("Failed to encode branch response: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}
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
		githubMockHandler(t)(w, r)
	}))
	defer ts.Close()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			provider, _ := NewGithubProvider("argoproj", "", ts.URL, c.allBranches)
			rawRepos, err := ListRepos(context.Background(), provider, c.filters, c.proto)
			if c.hasError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
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
		githubMockHandler(t)(w, r)
	}))
	defer ts.Close()
	host, _ := NewGithubProvider("argoproj", "", ts.URL, false)
	repo := &Repository{
		Organization: "argoproj",
		Repository:   "argo-cd",
		Branch:       "master",
	}
	ok, err := host.RepoHasPath(context.Background(), repo, "pkg/")
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = host.RepoHasPath(context.Background(), repo, "notathing/")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestGithubGetBranches(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		githubMockHandler(t)(w, r)
	}))
	defer ts.Close()
	host, _ := NewGithubProvider("argoproj", "", ts.URL, false)
	repo := &Repository{
		Organization: "argoproj",
		Repository:   "argo-cd",
		Branch:       "master",
	}
	repos, err := host.GetBranches(context.Background(), repo)
	if err != nil {
		require.NoError(t, err)
	} else {
		assert.Equal(t, "master", repos[0].Branch)
	}
	// Branch Doesn't exists instead of error will return no error
	repo2 := &Repository{
		Organization: "argoproj",
		Repository:   "applicationset",
		Branch:       "main",
	}
	_, err = host.GetBranches(context.Background(), repo2)
	require.NoError(t, err)

	// Get all branches
	host.allBranches = true
	repos, err = host.GetBranches(context.Background(), repo)
	if err != nil {
		require.NoError(t, err)
	} else {
		// considering master  branch to  exist.
		assert.Len(t, repos, 1)
	}
}
