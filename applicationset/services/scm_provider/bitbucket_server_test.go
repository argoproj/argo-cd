package scm_provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func defaultHandler(t *testing.T) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var err error
		switch r.RequestURI {
		case "/rest/api/1.0/projects/PROJECT/repos?limit=100":
			_, err = io.WriteString(w, `{
				"size": 1,
				"limit": 100,
				"isLastPage": true,
				"values": [
					{
						"id": 1,
						"name": "REPO",
						"project": {
							"key": "PROJECT"
						},
						"links": {
							"clone": [
								{
									"href": "ssh://git@mycompany.bitbucket.org/PROJECT/REPO.git",
									"name": "ssh"
								},
								{
									"href": "https://mycompany.bitbucket.org/scm/PROJECT/REPO.git",
									"name": "http"
								}
							]
						}
					}
				],
				"start": 0
			}`)
		case "/rest/api/1.0/projects/PROJECT/repos/REPO/branches?limit=100":
			_, err = io.WriteString(w, `{
				"size": 1,
				"limit": 100,
				"isLastPage": true,
				"values": [
					{
						"id": "refs/heads/main",
						"displayId": "main",
						"type": "BRANCH",
						"latestCommit": "8d51122def5632836d1cb1026e879069e10a1e13",
						"latestChangeset": "8d51122def5632836d1cb1026e879069e10a1e13",
						"isDefault": true
					}
				],
				"start": 0
			}`)
		case "/rest/api/1.0/projects/PROJECT/repos/REPO/branches/default":
			_, err = io.WriteString(w, `{
				"id": "refs/heads/main",
				"displayId": "main",
				"type": "BRANCH",
				"latestCommit": "8d51122def5632836d1cb1026e879069e10a1e13",
				"latestChangeset": "8d51122def5632836d1cb1026e879069e10a1e13",
				"isDefault": true
			}`)
		default:
			t.Fail()
		}
		if err != nil {
			t.Fail()
		}
	}
}

func verifyDefaultRepo(t *testing.T, err error, repos []*Repository) {
	assert.NoError(t, err)
	assert.Equal(t, 1, len(repos))
	assert.Equal(t, Repository{
		Organization: "PROJECT",
		Repository:   "REPO",
		URL:          "ssh://git@mycompany.bitbucket.org/PROJECT/REPO.git",
		Branch:       "main",
		SHA:          "8d51122def5632836d1cb1026e879069e10a1e13",
		Labels:       []string{},
		RepositoryId: 1,
	}, *repos[0])
}

func TestListReposNoAuth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))
		defaultHandler(t)(w, r)
	}))
	defer ts.Close()
	provider, err := NewBitbucketServerProviderNoAuth(context.Background(), ts.URL, "PROJECT", true)
	assert.NoError(t, err)
	repos, err := provider.ListRepos(context.Background(), "ssh")
	verifyDefaultRepo(t, err, repos)
}

func TestListReposPagination(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))
		var err error
		switch r.RequestURI {
		case "/rest/api/1.0/projects/PROJECT/repos?limit=100":
			_, err = io.WriteString(w, `{
				"size": 1,
				"limit": 100,
				"isLastPage": false,
				"values": [
					{
						"id": 100,
						"name": "REPO",
						"project": {
							"key": "PROJECT"
						},
						"links": {
							"clone": [
								{
									"href": "ssh://git@mycompany.bitbucket.org/PROJECT/REPO.git",
									"name": "ssh"
								},
								{
									"href": "https://mycompany.bitbucket.org/scm/PROJECT/REPO.git",
									"name": "http"
								}
							]
						}
					}
				],
				"start": 0,
				"nextPageStart": 200
			}`)
		case "/rest/api/1.0/projects/PROJECT/repos?limit=100&start=200":
			_, err = io.WriteString(w, `{
				"size": 1,
				"limit": 100,
				"isLastPage": true,
				"values": [
					{
						"id": 200,
						"name": "REPO2",
						"project": {
							"key": "PROJECT"
						},
						"links": {
							"clone": [
								{
									"href": "ssh://git@mycompany.bitbucket.org/PROJECT/REPO2.git",
									"name": "ssh"
								},
								{
									"href": "https://mycompany.bitbucket.org/scm/PROJECT/REPO2.git",
									"name": "http"
								}
							]
						}
					}
				],
				"start": 200
			}`)
		case "/rest/api/1.0/projects/PROJECT/repos/REPO/branches/default":
			_, err = io.WriteString(w, `{
				"id": "refs/heads/main",
				"displayId": "main",
				"type": "BRANCH",
				"latestCommit": "8d51122def5632836d1cb1026e879069e10a1e13",
				"isDefault": true
			}`)
		case "/rest/api/1.0/projects/PROJECT/repos/REPO2/branches/default":
			_, err = io.WriteString(w, `{
				"id": "refs/heads/development",
				"displayId": "development",
				"type": "BRANCH",
				"latestCommit": "2d51122def5632836d1cb1026e879069e10a1e13",
				"isDefault": true
			}`)
		default:
			t.Fail()
		}
		if err != nil {
			t.Fail()
		}
	}))
	defer ts.Close()
	provider, err := NewBitbucketServerProviderNoAuth(context.Background(), ts.URL, "PROJECT", true)
	assert.NoError(t, err)
	repos, err := provider.ListRepos(context.Background(), "ssh")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(repos))
	assert.Equal(t, Repository{
		Organization: "PROJECT",
		Repository:   "REPO",
		URL:          "ssh://git@mycompany.bitbucket.org/PROJECT/REPO.git",
		Branch:       "main",
		SHA:          "8d51122def5632836d1cb1026e879069e10a1e13",
		Labels:       []string{},
		RepositoryId: 100,
	}, *repos[0])

	assert.Equal(t, Repository{
		Organization: "PROJECT",
		Repository:   "REPO2",
		URL:          "ssh://git@mycompany.bitbucket.org/PROJECT/REPO2.git",
		Branch:       "development",
		SHA:          "2d51122def5632836d1cb1026e879069e10a1e13",
		Labels:       []string{},
		RepositoryId: 200,
	}, *repos[1])
}

func TestGetBranchesBranchPagination(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))
		switch r.RequestURI {
		case "/rest/api/1.0/projects/PROJECT/repos/REPO/branches?limit=100":
			_, err := io.WriteString(w, `{
				"size": 1,
				"limit": 100,
				"isLastPage": false,
				"values": [
					{
						"id": "refs/heads/main",
						"displayId": "main",
						"type": "BRANCH",
						"latestCommit": "8d51122def5632836d1cb1026e879069e10a1e13",
						"latestChangeset": "8d51122def5632836d1cb1026e879069e10a1e13",
						"isDefault": true
					}
				],
				"start": 0,
				"nextPageStart": 200
			}`)
			if err != nil {
				t.Fail()
			}
			return
		case "/rest/api/1.0/projects/PROJECT/repos/REPO/branches?limit=100&start=200":
			_, err := io.WriteString(w, `{
				"size": 1,
				"limit": 100,
				"isLastPage": true,
				"values": [
					{
						"id": "refs/heads/feature",
						"displayId": "feature",
						"type": "BRANCH",
						"latestCommit": "9d51122def5632836d1cb1026e879069e10a1e13",
						"latestChangeset": "9d51122def5632836d1cb1026e879069e10a1e13",
						"isDefault": true
					}
				],
				"start": 200
			}`)
			if err != nil {
				t.Fail()
			}
			return
		}
		defaultHandler(t)(w, r)
	}))
	defer ts.Close()
	provider, err := NewBitbucketServerProviderNoAuth(context.Background(), ts.URL, "PROJECT", true)
	assert.NoError(t, err)
	repos, err := provider.GetBranches(context.Background(), &Repository{
		Organization: "PROJECT",
		Repository:   "REPO",
		URL:          "ssh://git@mycompany.bitbucket.org/PROJECT/REPO.git",
		Labels:       []string{},
		RepositoryId: 1,
	})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(repos))
	assert.Equal(t, Repository{
		Organization: "PROJECT",
		Repository:   "REPO",
		URL:          "ssh://git@mycompany.bitbucket.org/PROJECT/REPO.git",
		Branch:       "main",
		SHA:          "8d51122def5632836d1cb1026e879069e10a1e13",
		Labels:       []string{},
		RepositoryId: 1,
	}, *repos[0])

	assert.Equal(t, Repository{
		Organization: "PROJECT",
		Repository:   "REPO",
		URL:          "ssh://git@mycompany.bitbucket.org/PROJECT/REPO.git",
		Branch:       "feature",
		SHA:          "9d51122def5632836d1cb1026e879069e10a1e13",
		Labels:       []string{},
		RepositoryId: 1,
	}, *repos[1])
}

func TestGetBranchesDefaultOnly(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))
		switch r.RequestURI {
		case "/rest/api/1.0/projects/PROJECT/repos/REPO/branches/default":
			_, err := io.WriteString(w, `{
				"id": "refs/heads/default",
				"displayId": "default",
				"type": "BRANCH",
				"latestCommit": "ab51122def5632836d1cb1026e879069e10a1e13",
				"latestChangeset": "ab51122def5632836d1cb1026e879069e10a1e13",
				"isDefault": true
			}`)
			if err != nil {
				t.Fail()
			}
			return
		}
		defaultHandler(t)(w, r)
	}))
	defer ts.Close()
	provider, err := NewBitbucketServerProviderNoAuth(context.Background(), ts.URL, "PROJECT", false)
	assert.NoError(t, err)
	repos, err := provider.GetBranches(context.Background(), &Repository{
		Organization: "PROJECT",
		Repository:   "REPO",
		URL:          "ssh://git@mycompany.bitbucket.org/PROJECT/REPO.git",
		Labels:       []string{},
		RepositoryId: 1,
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(repos))
	assert.Equal(t, Repository{
		Organization: "PROJECT",
		Repository:   "REPO",
		URL:          "ssh://git@mycompany.bitbucket.org/PROJECT/REPO.git",
		Branch:       "default",
		SHA:          "ab51122def5632836d1cb1026e879069e10a1e13",
		Labels:       []string{},
		RepositoryId: 1,
	}, *repos[0])
}

func TestGetBranchesMissingDefault(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))
		switch r.RequestURI {
		case "/rest/api/1.0/projects/PROJECT/repos/REPO/branches/default":
			http.Error(w, "Not found", http.StatusNotFound)
		}
		defaultHandler(t)(w, r)
	}))
	defer ts.Close()
	provider, err := NewBitbucketServerProviderNoAuth(context.Background(), ts.URL, "PROJECT", false)
	assert.NoError(t, err)
	repos, err := provider.GetBranches(context.Background(), &Repository{
		Organization: "PROJECT",
		Repository:   "REPO",
		URL:          "ssh://git@mycompany.bitbucket.org/PROJECT/REPO.git",
		Labels:       []string{},
		RepositoryId: 1,
	})
	assert.NoError(t, err)
	assert.Empty(t, repos)
}

func TestGetBranchesEmptyRepo(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))
		switch r.RequestURI {
		case "/rest/api/1.0/projects/PROJECT/repos/REPO/branches/default":
			return
		}
	}))
	defer ts.Close()
	provider, err := NewBitbucketServerProviderNoAuth(context.Background(), ts.URL, "PROJECT", false)
	assert.NoError(t, err)
	repos, err := provider.GetBranches(context.Background(), &Repository{
		Organization: "PROJECT",
		Repository:   "REPO",
		URL:          "ssh://git@mycompany.bitbucket.org/PROJECT/REPO.git",
		Labels:       []string{},
		RepositoryId: 1,
	})
	assert.Empty(t, repos)
	assert.NoError(t, err)
}

func TestGetBranchesErrorDefaultBranch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))
		switch r.RequestURI {
		case "/rest/api/1.0/projects/PROJECT/repos/REPO/branches/default":
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		defaultHandler(t)(w, r)
	}))
	defer ts.Close()
	provider, err := NewBitbucketServerProviderNoAuth(context.Background(), ts.URL, "PROJECT", false)
	assert.NoError(t, err)
	_, err = provider.GetBranches(context.Background(), &Repository{
		Organization: "PROJECT",
		Repository:   "REPO",
		URL:          "ssh://git@mycompany.bitbucket.org/PROJECT/REPO.git",
		Labels:       []string{},
		RepositoryId: 1,
	})
	assert.Error(t, err)
}

func TestListReposBasicAuth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Basic dXNlcjpwYXNzd29yZA==", r.Header.Get("Authorization"))
		assert.Equal(t, "no-check", r.Header.Get("X-Atlassian-Token"))
		defaultHandler(t)(w, r)
	}))
	defer ts.Close()
	provider, err := NewBitbucketServerProviderBasicAuth(context.Background(), "user", "password", ts.URL, "PROJECT", true)
	assert.NoError(t, err)
	repos, err := provider.ListRepos(context.Background(), "ssh")
	verifyDefaultRepo(t, err, repos)
}

func TestListReposDefaultBranch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))
		switch r.RequestURI {
		case "/rest/api/1.0/projects/PROJECT/repos/REPO/branches/default":
			_, err := io.WriteString(w, `{
				"id": "refs/heads/default",
				"displayId": "default",
				"type": "BRANCH",
				"latestCommit": "1d51122def5632836d1cb1026e879069e10a1e13",
				"latestChangeset": "1d51122def5632836d1cb1026e879069e10a1e13",
				"isDefault": true
			}`)
			if err != nil {
				t.Fail()
			}
			return
		}
		defaultHandler(t)(w, r)
	}))
	defer ts.Close()
	provider, err := NewBitbucketServerProviderNoAuth(context.Background(), ts.URL, "PROJECT", false)
	assert.NoError(t, err)
	repos, err := provider.ListRepos(context.Background(), "ssh")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(repos))
	assert.Equal(t, Repository{
		Organization: "PROJECT",
		Repository:   "REPO",
		URL:          "ssh://git@mycompany.bitbucket.org/PROJECT/REPO.git",
		Branch:       "default",
		SHA:          "1d51122def5632836d1cb1026e879069e10a1e13",
		Labels:       []string{},
		RepositoryId: 1,
	}, *repos[0])
}

func TestListReposMissingDefaultBranch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))
		switch r.RequestURI {
		case "/rest/api/1.0/projects/PROJECT/repos/REPO/branches/default":
			http.Error(w, "Not found", http.StatusNotFound)
		}
		defaultHandler(t)(w, r)
	}))
	defer ts.Close()
	provider, err := NewBitbucketServerProviderNoAuth(context.Background(), ts.URL, "PROJECT", false)
	assert.NoError(t, err)
	repos, err := provider.ListRepos(context.Background(), "ssh")
	assert.NoError(t, err)
	assert.Empty(t, repos)
}

func TestListReposErrorDefaultBranch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))
		switch r.RequestURI {
		case "/rest/api/1.0/projects/PROJECT/repos/REPO/branches/default":
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		defaultHandler(t)(w, r)
	}))
	defer ts.Close()
	provider, err := NewBitbucketServerProviderNoAuth(context.Background(), ts.URL, "PROJECT", false)
	assert.NoError(t, err)
	_, err = provider.ListRepos(context.Background(), "ssh")
	assert.Error(t, err)
}

func TestListReposCloneProtocol(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))
		defaultHandler(t)(w, r)
	}))
	defer ts.Close()
	provider, err := NewBitbucketServerProviderNoAuth(context.Background(), ts.URL, "PROJECT", true)
	assert.NoError(t, err)
	repos, err := provider.ListRepos(context.Background(), "https")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(repos))
	assert.Equal(t, Repository{
		Organization: "PROJECT",
		Repository:   "REPO",
		URL:          "https://mycompany.bitbucket.org/scm/PROJECT/REPO.git",
		Branch:       "main",
		SHA:          "8d51122def5632836d1cb1026e879069e10a1e13",
		Labels:       []string{},
		RepositoryId: 1,
	}, *repos[0])
}

func TestListReposUnknownProtocol(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))
		defaultHandler(t)(w, r)
	}))
	defer ts.Close()
	provider, err := NewBitbucketServerProviderNoAuth(context.Background(), ts.URL, "PROJECT", true)
	assert.NoError(t, err)
	_, errProtocol := provider.ListRepos(context.Background(), "http")
	assert.NotNil(t, errProtocol)
}

func TestBitbucketServerHasPath(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		switch r.RequestURI {
		case "/rest/api/1.0/projects/PROJECT/repos/REPO/browse/pkg?at=main&limit=100&type=true":
			_, err = io.WriteString(w, `{"type":"DIRECTORY"}`)
		case "/rest/api/1.0/projects/PROJECT/repos/REPO/browse/pkg/?at=main&limit=100&type=true":
			_, err = io.WriteString(w, `{"type":"DIRECTORY"}`)
		case "/rest/api/1.0/projects/PROJECT/repos/REPO/browse/anotherpkg/file.txt?at=main&limit=100&type=true":
			_, err = io.WriteString(w, `{"type":"FILE"}`)

		case "/rest/api/1.0/projects/PROJECT/repos/REPO/browse/anotherpkg/missing.txt?at=main&limit=100&type=true":
			http.Error(w, "The path \"anotherpkg/missing.txt\" does not exist at revision \"main\"", http.StatusNotFound)
		case "/rest/api/1.0/projects/PROJECT/repos/REPO/browse/notathing?at=main&limit=100&type=true":
			http.Error(w, "The path \"notathing\" does not exist at revision \"main\"", http.StatusNotFound)

		case "/rest/api/1.0/projects/PROJECT/repos/REPO/browse/return-redirect?at=main&limit=100&type=true":
			http.Redirect(w, r, "http://"+r.Host+"/rest/api/1.0/projects/PROJECT/repos/REPO/browse/redirected?at=main&limit=100&type=true", http.StatusMovedPermanently)
		case "/rest/api/1.0/projects/PROJECT/repos/REPO/browse/redirected?at=main&limit=100&type=true":
			_, err = io.WriteString(w, `{"type":"DIRECTORY"}`)

		case "/rest/api/1.0/projects/PROJECT/repos/REPO/browse/unauthorized-response?at=main&limit=100&type=true":
			http.Error(w, "Authentication failed", http.StatusUnauthorized)

		default:
			t.Fail()
		}
		if err != nil {
			t.Fail()
		}
	}))
	defer ts.Close()
	provider, err := NewBitbucketServerProviderNoAuth(context.Background(), ts.URL, "PROJECT", true)
	assert.NoError(t, err)
	repo := &Repository{
		Organization: "PROJECT",
		Repository:   "REPO",
		Branch:       "main",
	}
	ok, err := provider.RepoHasPath(context.Background(), repo, "pkg")
	assert.NoError(t, err)
	assert.True(t, ok)

	ok, err = provider.RepoHasPath(context.Background(), repo, "pkg/")
	assert.NoError(t, err)
	assert.True(t, ok)

	ok, err = provider.RepoHasPath(context.Background(), repo, "anotherpkg/file.txt")
	assert.NoError(t, err)
	assert.True(t, ok)

	ok, err = provider.RepoHasPath(context.Background(), repo, "anotherpkg/missing.txt")
	assert.NoError(t, err)
	assert.False(t, ok)

	ok, err = provider.RepoHasPath(context.Background(), repo, "notathing")
	assert.NoError(t, err)
	assert.False(t, ok)

	ok, err = provider.RepoHasPath(context.Background(), repo, "return-redirect")
	assert.NoError(t, err)
	assert.True(t, ok)

	_, err = provider.RepoHasPath(context.Background(), repo, "unauthorized-response")
	assert.Error(t, err)
}
