package scm_provider

import (
	"context"
	"github.com/argoproj/argo-cd/v2/applicationset/services/scm_provider/testdata"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func scmManagerMockHandler(t *testing.T) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.RequestURI {
		case "/api/v2/repositories?pageSize=9999&archived=true":
			_, err := io.WriteString(w, `{
		"page": 0,
		"pageTotal": 1,
		"_embedded": {
			"repositories":	[{
				"namespace": "test-argocd",
				"name": "pr-test",
				"type": "git",
				"description": "test",
				"contact": "eduard.heimbuch@cloudogu.com",
				"archived": false,
				"_links": {
					"protocol": [
						{ "name": "http", "href": "https://scm-manager.org/test-argocd/pr-test" },
						{ "name": "ssh", "href": "git@scm-manager.org:test-argocd/pr-test.git" }
					]
				}
			}]
		}
}`)
			if err != nil {
				t.Fail()
			}
		case "/api/v2/repositories/test-argocd/pr-test/branches/":
			_, err := io.WriteString(w, `{
				"_embedded": {
				    "branches": [{
							"name": "main",
							"defaultBranch": true,
							"revision": "72687815ccba81ef014a96201cc2e846a68789d8",
							"stale": false,
							"lastCommitDate": "2022-04-05T14:29:51-04:00",
							"lastCommitter": {
								"name": "Eduard Heimbuch",
								"mail": "eduard.heimbuch@cloudogu.com"
							}
				    }]
				}
			}`)
			if err != nil {
				t.Fail()
			}
		case "/api/v2/repositories/test-argocd/pr-test":
			_, err := io.WriteString(w, `{
				"namespace": "test-argocd",
				"name": "pr-test",
				"type": "git",
				"description": "test",
				"contact": "eduard.heimbuch@cloudogu.com",
				"archived": false,
				"_links": {
					"protocol": [
						{ "name": "ssh", "href": "git@scm-manager.org:test-argocd/pr-test.git" },
						{ "name": "http", "href": "https://scm-manager.org/test-argocd/pr-test" }
					]
				}
			}`)
			if err != nil {
				t.Fail()
			}
		case "/api/v2/repositories/test-argocd/pr-test/content/master/README.md?ref=master":
			_, err := io.WriteString(w, `"my readme test"`)
			require.NoError(t, err)
		case "/api/v2/repositories/test-argocd/pr-test/content/master/build":
			_, err := io.WriteString(w, testdata.ReposGiteaGoSdkContentsGiteaResponse)
			require.NoError(t, err)
		case "/api/v2/repositories/test-argocd/pr-test/content/master/unknownFile":
			w.WriteHeader(404)
		default:
			_, err := io.WriteString(w, `[]`)
			if err != nil {
				t.Fail()
			}
		}
	}
}
func TestScmManagerListRepos(t *testing.T) {
	cases := []struct {
		name, proto, url                        string
		hasError, allBranches, includeSubgroups bool
		branches                                []string
		filters                                 []v1alpha1.SCMProviderGeneratorFilter
	}{
		{
			name:        "blank protocol",
			allBranches: false,
			url:         "git@scm-manager.org:test-argocd/pr-test.git",
			branches:    []string{"main"},
		},
		{
			name:        "ssh protocol",
			allBranches: false,
			proto:       "ssh",
			url:         "git@scm-manager.org:test-argocd/pr-test.git",
		},
		{
			name:        "https protocol",
			allBranches: false,
			proto:       "https",
			url:         "https://scm-manager.org/test-argocd/pr-test",
		},
		{
			name:        "other protocol",
			allBranches: false,
			proto:       "other",
			hasError:    true,
		},
		{
			name:        "all branches",
			allBranches: true,
			url:         "git@scm-manager.org:test-argocd/pr-test.git",
			branches:    []string{"main"},
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scmManagerMockHandler(t)(w, r)
	}))
	defer ts.Close()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			provider, _ := NewScmManagerProvider(context.Background(), "", ts.URL, c.allBranches, false, "")
			rawRepos, err := ListRepos(context.Background(), provider, c.filters, c.proto)
			if c.hasError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
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

func TestScmManagerHasPath(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scmManagerMockHandler(t)(w, r)
	}))
	defer ts.Close()
	host, _ := NewScmManagerProvider(context.Background(), "", ts.URL, false, false, "")
	repo := &Repository{
		Organization: "test-argocd",
		Repository:   "pr-test",
		Branch:       "master",
	}

	t.Run("file exists", func(t *testing.T) {
		ok, err := host.RepoHasPath(context.Background(), repo, "README.md")
		assert.Nil(t, err)
		assert.True(t, ok)
	})

	t.Run("directory exists", func(t *testing.T) {
		ok, err := host.RepoHasPath(context.Background(), repo, "build")
		assert.Nil(t, err)
		assert.True(t, ok)
	})

	t.Run("does not exists", func(t *testing.T) {
		ok, err := host.RepoHasPath(context.Background(), repo, "unknownFile")
		assert.Nil(t, err)
		assert.False(t, ok)
	})
}
