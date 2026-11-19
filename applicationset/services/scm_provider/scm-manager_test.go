package scm_provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/applicationset/services/scm_provider/testdata"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func scmManagerMockHandlerWithNormalRepository(t *testing.T) func(http.ResponseWriter, *http.Request) {
	t.Helper()
	return scmManagerMockHandler(t, "pr-test")
}

func scmManagerMockHandlerWithEmptyRepository(t *testing.T) func(http.ResponseWriter, *http.Request) {
	t.Helper()
	return scmManagerMockHandler(t, "empty")
}

func scmManagerMockHandler(t *testing.T, repositoryName string) func(http.ResponseWriter, *http.Request) {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.RequestURI {
		case "/api/v2/repositories?archived=true&pageSize=9999":
			handleRepositoryOverviewRequest(t, repositoryName, w)
		case "/api/v2/repositories/test-argocd/pr-test/branches/":
			handleBranchesRequestWithExistingBranches(t, w)
		case "/api/v2/repositories/test-argocd/empty/branches/":
			handleBranchesRequestWithoutBranches(t, w)
		case "/api/v2/repositories/test-argocd/pr-test/content/master/README.md":
			_, err := io.WriteString(w, `"my readme test"`)
			require.NoError(t, err)
		case "/api/v2/repositories/test-argocd/pr-test/content/master/build":
			_, err := io.WriteString(w, testdata.ReposGiteaGoSdkContentsGiteaResponse)
			require.NoError(t, err)
		case "/api/v2/repositories/test-argocd/pr-test/content/master/unknownFile":
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("unexpected request %s", r.RequestURI)
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func handleBranchesRequestWithoutBranches(t *testing.T, w http.ResponseWriter) {
	t.Helper()
	_, err := io.WriteString(w, `{
					"_embedded": {
						"branches": []
					}
				}`)
	if err != nil {
		t.Fail()
	}
}

func handleBranchesRequestWithExistingBranches(t *testing.T, w http.ResponseWriter) {
	t.Helper()
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
}

func handleRepositoryOverviewRequest(t *testing.T, repositoryName string, w http.ResponseWriter) {
	t.Helper()
	_, err := fmt.Fprintf(w, `{
			"page": 0,
			"pageTotal": 1,
			"_embedded": {
				"repositories":	[{
					"namespace": "test-argocd",
					"name": "%s",
					"type": "git",
					"description": "test",
					"contact": "eduard.heimbuch@cloudogu.com",
					"archived": false,
					"_links": {
						"protocol": [
							{ "name": "http", "href": "https://scm-manager.org/test-argocd/%s" },
							{ "name": "ssh", "href": "git@scm-manager.org:test-argocd/%s.git" }
						]
					}
				}]
			}
		}`, repositoryName, repositoryName, repositoryName)
	if err != nil {
		t.Fail()
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
		{
			name:        "all branches with https",
			allBranches: true,
			proto:       "https",
			url:         "https://scm-manager.org/test-argocd/pr-test",
			branches:    []string{"main"},
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scmManagerMockHandlerWithNormalRepository(t)(w, r)
	}))
	defer ts.Close()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			provider, _ := NewScmManagerProvider(context.Background(), "", ts.URL, c.allBranches, false, "", nil)
			rawRepos, err := ListRepos(context.Background(), provider, c.filters, c.proto)
			if c.hasError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				repos := []*Repository{}
				branches := []string{}
				for _, r := range rawRepos {
					if r.Repository == "pr-test" {
						repos = append(repos, r)
						branches = append(branches, r.Branch)
					}
				}
				require.NotEmpty(t, repos)
				assert.Equal(t, c.url, repos[0].URL)
				for _, b := range c.branches {
					assert.Contains(t, branches, b)
				}
			}
		})
	}
}

func TestScmManagerListEmptyRepos(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scmManagerMockHandlerWithEmptyRepository(t)(w, r)
	}))
	defer ts.Close()
	t.Run("empty repository", func(t *testing.T) {
		provider, _ := NewScmManagerProvider(context.Background(), "", ts.URL, false, false, "", nil)
		rawRepos, err := ListRepos(context.Background(), provider, make([]v1alpha1.SCMProviderGeneratorFilter, 0), "https")
		require.NoError(t, err)
		assert.Empty(t, rawRepos)
	})
}

func TestScmManagerHasPath(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scmManagerMockHandlerWithNormalRepository(t)(w, r)
	}))
	defer ts.Close()
	host, _ := NewScmManagerProvider(context.Background(), "", ts.URL, false, false, "", nil)
	repo := &Repository{
		Organization: "test-argocd",
		Repository:   "pr-test",
		Branch:       "master",
	}

	t.Run("file exists", func(t *testing.T) {
		ok, err := host.RepoHasPath(context.Background(), repo, "README.md")
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("directory exists", func(t *testing.T) {
		ok, err := host.RepoHasPath(context.Background(), repo, "build")
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("does not exist", func(t *testing.T) {
		ok, err := host.RepoHasPath(context.Background(), repo, "unknownFile")
		require.NoError(t, err)
		assert.False(t, ok)
	})
}

func TestScmManagerRepositoryPages(t *testing.T) {
	for _, failNextPage := range []bool{false, true} {
		name := "all pages"
		if failNextPage {
			name = "later page fails"
		}
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v2/repositories":
					switch r.URL.Query().Get("page") {
					case "":
						_, _ = io.WriteString(w, `{"page":0,"pageTotal":2,"_embedded":{"repositories":[{"namespace":"team","name":"first","_links":{"protocol":[{"name":"http","href":"https://example.org/first"}]}}]}}`)
					case "1":
						if failNextPage {
							w.WriteHeader(http.StatusServiceUnavailable)
							return
						}
						_, _ = io.WriteString(w, `{"page":1,"pageTotal":2,"_embedded":{"repositories":[{"namespace":"team","name":"second","_links":{"protocol":[{"name":"http","href":"https://example.org/second"}]}}]}}`)
					default:
						t.Errorf("unexpected page in %s", r.RequestURI)
						w.WriteHeader(http.StatusBadRequest)
					}
				case "/api/v2/repositories/team/first/branches/", "/api/v2/repositories/team/second/branches/":
					handleBranchesRequestWithExistingBranches(t, w)
				default:
					t.Errorf("unexpected request %s", r.RequestURI)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			provider, err := NewScmManagerProvider(t.Context(), "", server.URL, false, false, "", nil)
			require.NoError(t, err)
			repos, err := provider.ListRepos(t.Context(), "https")
			if failNextPage {
				require.ErrorContains(t, err, "HTTP 503")
				require.Nil(t, repos)
				return
			}
			require.NoError(t, err)
			require.Len(t, repos, 2)
			assert.Equal(t, "second", repos[1].Repository)
		})
	}
}

func TestScmManagerPrivateContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer private-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/v2/repositories/team/work/content/main/README.md":
			_, _ = io.WriteString(w, "readme")
		case "/api/v2/repositories/team/work/content/main/truncated":
			w.Header().Set("Content-Length", "20")
			_, _ = io.WriteString(w, "short")
		case "/api/v2/repositories/team/work/content/main/unavailable":
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			t.Errorf("unexpected request %s", r.RequestURI)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	provider, err := NewScmManagerProvider(t.Context(), "private-token", server.URL, false, false, "", nil)
	require.NoError(t, err)
	repo := &Repository{Organization: "team", Repository: "work", Branch: "main"}

	found, err := provider.RepoHasPath(t.Context(), repo, "README.md")
	require.NoError(t, err)
	assert.True(t, found)
	found, err = provider.RepoHasPath(t.Context(), repo, "unavailable")
	require.ErrorContains(t, err, "HTTP 503")
	assert.False(t, found)
	found, err = provider.RepoHasPath(t.Context(), repo, "truncated")
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
	assert.False(t, found)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = provider.RepoHasPath(ctx, repo, "README.md")
	require.ErrorIs(t, err, context.Canceled)
}
