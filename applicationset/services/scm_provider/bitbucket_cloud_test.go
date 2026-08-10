package scm_provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestBitbucketHasRepo(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/repositories/test-owner/testmike/src/dc1edb6c7d650d8ba67719ddf7b662ad8f8fb798/.gitignore2" {
			res.WriteHeader(http.StatusNotFound)
			_, err := res.Write([]byte(""))
			if err != nil {
				require.NoError(t, fmt.Errorf("Error in mock response %w", err))
			}
		}
		if req.URL.Path == "/repositories/test-owner/testmike/src/dc1edb6c7d650d8ba67719ddf7b662ad8f8fb798/.gitignore" {
			res.WriteHeader(http.StatusOK)
			_, err := res.Write([]byte(`{
				"mimetype": null,
				"links": {
					"self": {
						"href": "https://api.bitbucket.org/2.0/repositories/test-owner/testmike/src/dc1edb6c7d650d8ba67719ddf7b662ad8f8fb798/.gitignore"
					},
					"meta": {
						"href": "https://api.bitbucket.org/2.0/repositories/test-owner/testmike/src/dc1edb6c7d650d8ba67719ddf7b662ad8f8fb798/.gitignore?format=meta"
					},
					"history": {
						"href": "https://api.bitbucket.org/2.0/repositories/test-owner/testmike/filehistory/dc1edb6c7d650d8ba67719ddf7b662ad8f8fb798/.gitignore"
					}
				},
				"escaped_path": ".gitignore",
				"path": ".gitignore",
				"commit": {
					"type": "commit",
					"hash": "dc1edb6c7d650d8ba67719ddf7b662ad8f8fb798",
					"links": {
						"self": {
							"href": "https://api.bitbucket.org/2.0/repositories/test-owner/testmike/commit/dc1edb6c7d650d8ba67719ddf7b662ad8f8fb798"
						},
						"html": {
							"href": "https://bitbucket.org/test-owner/testmike/commits/dc1edb6c7d650d8ba67719ddf7b662ad8f8fb798"
						}
					}
				},
				"attributes": [],
				"type": "commit_file",
				"size": 624
			}`))
			if err != nil {
				require.NoError(t, fmt.Errorf("Error in mock response %w", err))
			}
		}
	}))
	defer func() { testServer.Close() }()

	t.Setenv("BITBUCKET_API_BASE_URL", testServer.URL)
	cases := []struct {
		name, path, repo, owner, sha string
		status                       int
	}{
		{
			name:   "exists",
			owner:  "test-owner",
			repo:   "testmike",
			path:   ".gitignore",
			sha:    "dc1edb6c7d650d8ba67719ddf7b662ad8f8fb798",
			status: http.StatusOK,
		},
		{
			name:   "not exists",
			owner:  "test-owner",
			repo:   "testmike",
			path:   ".gitignore2",
			sha:    "dc1edb6c7d650d8ba67719ddf7b662ad8f8fb798",
			status: http.StatusNotFound,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			provider, _ := NewBitBucketCloudProvider(c.owner, "user", "password", false)
			repo := &Repository{
				Organization: c.owner,
				Repository:   c.repo,
				SHA:          c.sha,
				Branch:       "main",
			}
			hasPath, err := provider.RepoHasPath(t.Context(), repo, c.path)
			if err != nil {
				require.Error(t, fmt.Errorf("Error in test %w", err))
			}
			if c.status != http.StatusOK {
				assert.False(t, hasPath)
			} else {
				assert.True(t, hasPath)
			}
		})
	}
}

func TestBitbucketListRepos(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusOK)
		if req.URL.Path == "/repositories/test-owner/testmike/refs/branches" {
			_, err := res.Write([]byte(`{
				"pagelen": 10,
				"values": [
					{
						"name": "main",
						"links": {
							"commits": {
								"href": "https://api.bitbucket.org/2.0/repositories/test-owner/testmike/commits/main"
							},
							"self": {
								"href": "https://api.bitbucket.org/2.0/repositories/test-owner/testmike/refs/branches/main"
							},
							"html": {
								"href": "https://bitbucket.org/test-owner/testmike/branch/main"
							}
						},
						"default_merge_strategy": "merge_commit",
						"merge_strategies": [
							"merge_commit",
							"squash",
							"fast_forward"
						],
						"type": "branch",
						"target": {
							"hash": "dc1edb6c7d650d8ba67719ddf7b662ad8f8fb798",
							"repository": {
								"links": {
									"self": {
										"href": "https://api.bitbucket.org/2.0/repositories/test-owner/testmike"
									},
									"html": {
										"href": "https://bitbucket.org/test-owner/testmike"
									},
									"avatar": {
										"href": "https://bytebucket.org/ravatar/%7B76606e75-8aeb-4a87-9396-4abee652ec63%7D?ts=default"
									}
								},
								"type": "repository",
								"name": "testMike",
								"full_name": "test-owner/testmike",
								"uuid": "{76606e75-8aeb-4a87-9396-4abee652ec63}"
							},
							"links": {
								"self": {
									"href": "https://api.bitbucket.org/2.0/repositories/test-owner/testmike/commit/dc1edb6c7d650d8ba67719ddf7b662ad8f8fb798"
								},
								"comments": {
									"href": "https://api.bitbucket.org/2.0/repositories/test-owner/testmike/commit/dc1edb6c7d650d8ba67719ddf7b662ad8f8fb798/comments"
								},
								"patch": {
									"href": "https://api.bitbucket.org/2.0/repositories/test-owner/testmike/patch/dc1edb6c7d650d8ba67719ddf7b662ad8f8fb798"
								},
								"html": {
									"href": "https://bitbucket.org/test-owner/testmike/commits/dc1edb6c7d650d8ba67719ddf7b662ad8f8fb798"
								},
								"diff": {
									"href": "https://api.bitbucket.org/2.0/repositories/test-owner/testmike/diff/dc1edb6c7d650d8ba67719ddf7b662ad8f8fb798"
								},
								"approve": {
									"href": "https://api.bitbucket.org/2.0/repositories/test-owner/testmike/commit/dc1edb6c7d650d8ba67719ddf7b662ad8f8fb798/approve"
								},
								"statuses": {
									"href": "https://api.bitbucket.org/2.0/repositories/test-owner/testmike/commit/dc1edb6c7d650d8ba67719ddf7b662ad8f8fb798/statuses"
								}
							},
							"author": {
								"raw": "Mike Tester <tester@gmail.com>",
								"type": "author",
								"user": {
									"display_name": "Mike Tester",
									"uuid": "{ca84788f-050b-456b-5cac-93fb4484a686}",
									"links": {
										"self": {
											"href": "https://api.bitbucket.org/2.0/users/%7Bca84788f-050b-456b-5cac-93fb4484a686%7D"
										},
										"html": {
											"href": "https://bitbucket.org/%7Bca84788f-050b-456b-5cac-93fb4484a686%7D/"
										},
										"avatar": {
											"href": "https://secure.gravatar.com/avatar/03450fe11788d0dbb39b804110c07b9f?d=https%3A%2F%2Favatar-management--avatars.us-west-2.prod.public.atl-paas.net%2Finitials%2FMM-4.png"
										}
									},
									"type": "user",
									"nickname": "Mike Tester",
									"account_id": "61ec57859d174000690f702b"
								}
							},
							"parents": [],
							"date": "2022-03-07T19:37:58+00:00",
							"message": "Initial commit",
							"type": "commit"
						}
					}
				],
				"page": 1,
				"size": 1
			}`))
			if err != nil {
				require.NoError(t, fmt.Errorf("Error in mock response %w", err))
			}
		}
		if req.URL.Path == "/repositories/test-owner/testmike/refs/branches/main" {
			_, err := res.Write([]byte(`{
				"name": "main",
				"links": {
					"commits": {
						"href": "https://api.bitbucket.org/2.0/repositories/test-owner/testmike/commits/main"
					},
					"self": {
						"href": "https://api.bitbucket.org/2.0/repositories/test-owner/testmike/refs/branches/main"
					},
					"html": {
						"href": "https://bitbucket.org/test-owner/testmike/branch/main"
					}
				},
				"default_merge_strategy": "merge_commit",
				"merge_strategies": [
					"merge_commit",
					"squash",
					"fast_forward"
				],
				"type": "branch",
				"target": {
					"hash": "dc1edb6c7d650d8ba67719ddf7b662ad8f8fb798",
					"repository": {
						"links": {
							"self": {
								"href": "https://api.bitbucket.org/2.0/repositories/test-owner/testmike"
							},
							"html": {
								"href": "https://bitbucket.org/test-owner/testmike"
							},
							"avatar": {
								"href": "https://bytebucket.org/ravatar/%7B76606e75-8aeb-4a87-9396-4abee652ec63%7D?ts=default"
							}
						},
						"type": "repository",
						"name": "testMike",
						"full_name": "test-owner/testmike",
						"uuid": "{76606e75-8aeb-4a87-9396-4abee652ec63}"
					},
					"links": {
						"self": {
							"href": "https://api.bitbucket.org/2.0/repositories/test-owner/testmike/commit/dc1edb6c7d650d8ba67719ddf7b662ad8f8fb798"
						},
						"comments": {
							"href": "https://api.bitbucket.org/2.0/repositories/test-owner/testmike/commit/dc1edb6c7d650d8ba67719ddf7b662ad8f8fb798/comments"
						},
						"patch": {
							"href": "https://api.bitbucket.org/2.0/repositories/test-owner/testmike/patch/dc1edb6c7d650d8ba67719ddf7b662ad8f8fb798"
						},
						"html": {
							"href": "https://bitbucket.org/test-owner/testmike/commits/dc1edb6c7d650d8ba67719ddf7b662ad8f8fb798"
						},
						"diff": {
							"href": "https://api.bitbucket.org/2.0/repositories/test-owner/testmike/diff/dc1edb6c7d650d8ba67719ddf7b662ad8f8fb798"
						},
						"approve": {
							"href": "https://api.bitbucket.org/2.0/repositories/test-owner/testmike/commit/dc1edb6c7d650d8ba67719ddf7b662ad8f8fb798/approve"
						},
						"statuses": {
							"href": "https://api.bitbucket.org/2.0/repositories/test-owner/testmike/commit/dc1edb6c7d650d8ba67719ddf7b662ad8f8fb798/statuses"
						}
					},
					"author": {
						"raw": "Mike Tester <tester@gmail.com>",
						"type": "author",
						"user": {
							"display_name": "Mike Tester",
							"uuid": "{ca84788f-050b-456b-5cac-93fb4484a686}",
							"links": {
								"self": {
									"href": "https://api.bitbucket.org/2.0/users/%7Bca84788f-050b-456b-5cac-93fb4484a686%7D"
								},
								"html": {
									"href": "https://bitbucket.org/%7Bca84788f-050b-456b-5cac-93fb4484a686%7D/"
								},
								"avatar": {
									"href": "https://secure.gravatar.com/avatar/03450fe11788d0dbb39b804110c07b9f?d=https%3A%2F%2Favatar-management--avatars.us-west-2.prod.public.atl-paas.net%2Finitials%2FMM-4.png"
								}
							},
							"type": "user",
							"nickname": "Mike Tester",
							"account_id": "61ec57859d174000690f702b"
						}
					},
					"parents": [],
					"date": "2022-03-07T19:37:58+00:00",
					"message": "Initial commit",
					"type": "commit"
				}
			}`))
			if err != nil {
				require.NoError(t, fmt.Errorf("Error in mock response %w", err))
			}
		}
		if req.URL.Path == "/repositories/test-owner" {
			_, err := res.Write([]byte(`{
			"pagelen": 10,
			"values": [
				{
					"scm": "git",
					"has_wiki": false,
					"links": {
						"watchers": {
							"href": "https://api.bitbucket.org/2.0/repositories/test-owner/testmike/watchers"
						},
						"branches": {
							"href": "https://api.bitbucket.org/2.0/repositories/test-owner/testmike/refs/branches"
						},
						"tags": {
							"href": "https://api.bitbucket.org/2.0/repositories/test-owner/testmike/refs/tags"
						},
						"commits": {
							"href": "https://api.bitbucket.org/2.0/repositories/test-owner/testmike/commits"
						},
						"clone": [
							{
								"href": "https://test-owner@bitbucket.org/test-owner/testmike.git",
								"name": "https"
							},
							{
								"href": "git@bitbucket.org:test-owner/testmike.git",
								"name": "ssh"
							}
						],
						"self": {
							"href": "https://api.bitbucket.org/2.0/repositories/test-owner/testmike"
						},
						"source": {
							"href": "https://api.bitbucket.org/2.0/repositories/test-owner/testmike/src"
						},
						"html": {
							"href": "https://bitbucket.org/test-owner/testmike"
						},
						"avatar": {
							"href": "https://bytebucket.org/ravatar/%7B76606e75-8aeb-4a87-9396-4abee652ec63%7D?ts=default"
						},
						"hooks": {
							"href": "https://api.bitbucket.org/2.0/repositories/test-owner/testmike/hooks"
						},
						"forks": {
							"href": "https://api.bitbucket.org/2.0/repositories/test-owner/testmike/forks"
						},
						"downloads": {
							"href": "https://api.bitbucket.org/2.0/repositories/test-owner/testmike/downloads"
						},
						"pullrequests": {
							"href": "https://api.bitbucket.org/2.0/repositories/test-owner/testmike/pullrequests"
						}
					},
					"created_on": "2022-03-07T19:37:58.199968+00:00",
					"full_name": "test-owner/testmike",
					"owner": {
						"display_name": "Mike Tester",
						"uuid": "{ca84788f-050b-456b-5cac-93fb4484a686}",
						"links": {
							"self": {
								"href": "https://api.bitbucket.org/2.0/users/%7Bca84788f-050b-456b-5cac-93fb4484a686%7D"
							},
							"html": {
								"href": "https://bitbucket.org/%7Bca84788f-050b-456b-5cac-93fb4484a686%7D/"
							},
							"avatar": {
								"href": "https://secure.gravatar.com/avatar/03450fe11788d0dbb39b804110c07b9f?d=https%3A%2F%2Favatar-management--avatars.us-west-2.prod.public.atl-paas.net%2Finitials%2FMM-4.png"
							}
						},
						"type": "user",
						"nickname": "Mike Tester",
						"account_id": "61ec57859d174000690f702b"
					},
					"size": 58894,
					"uuid": "{76606e75-8aeb-4a87-9396-4abee652ec63}",
					"type": "repository",
					"website": null,
					"override_settings": {
						"branching_model": true,
						"default_merge_strategy": true,
						"branch_restrictions": true
					},
					"description": "",
					"has_issues": false,
					"slug": "testmike",
					"is_private": false,
					"name": "testMike",
					"language": "",
					"fork_policy": "allow_forks",
					"project": {
						"links": {
							"self": {
								"href": "https://api.bitbucket.org/2.0/workspaces/test-owner/projects/TEST"
							},
							"html": {
								"href": "https://bitbucket.org/test-owner/workspace/projects/TEST"
							},
							"avatar": {
								"href": "https://bitbucket.org/account/user/test-owner/projects/TEST/avatar/32?ts=1642881431"
							}
						},
						"type": "project",
						"name": "test",
						"key": "TEST",
						"uuid": "{603a1564-1509-4c97-b2a6-300a3fad2758}"
					},
					"mainbranch": {
						"type": "branch",
						"name": "main"
					},
					"workspace": {
						"slug": "test-owner",
						"type": "workspace",
						"name": "Mike Tester",
						"links": {
							"self": {
								"href": "https://api.bitbucket.org/2.0/workspaces/test-owner"
							},
							"html": {
								"href": "https://bitbucket.org/test-owner/"
							},
							"avatar": {
								"href": "https://bitbucket.org/workspaces/test-owner/avatar/?ts=1642878863"
							}
						},
						"uuid": "{ca84788f-050b-456b-5cac-93fb4484a686}"
					},
					"updated_on": "2022-03-07T19:37:59.933133+00:00"
				}
			],
			"page": 1,
			"size": 1
		}`))
			if err != nil {
				require.NoError(t, fmt.Errorf("Error in mock response %w", err))
			}
		}
	}))
	defer func() { testServer.Close() }()

	t.Setenv("BITBUCKET_API_BASE_URL", testServer.URL)
	cases := []struct {
		name, proto, owner    string
		hasError, allBranches bool
		branches              []string
		filters               []v1alpha1.SCMProviderGeneratorFilter
	}{
		{
			name:     "blank protocol",
			owner:    "test-owner",
			branches: []string{"main"},
		},
		{
			name:  "ssh protocol",
			proto: "ssh",
			owner: "test-owner",
		},
		{
			name:  "https protocol",
			proto: "https",
			owner: "test-owner",
		},
		{
			name:     "other protocol",
			proto:    "other",
			owner:    "test-owner",
			hasError: true,
		},
		{
			name:        "all branches",
			allBranches: true,
			owner:       "test-owner",
			branches:    []string{"main"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			provider, _ := NewBitBucketCloudProvider(c.owner, "user", "password", c.allBranches)
			rawRepos, err := ListRepos(t.Context(), provider, c.filters, c.proto)
			if c.hasError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				repos := []*Repository{}
				branches := []string{}
				for _, r := range rawRepos {
					if r.Repository == "testmike" {
						repos = append(repos, r)
						branches = append(branches, r.Branch)
					}
				}
				assert.NotEmpty(t, repos)
				for _, b := range c.branches {
					assert.Contains(t, branches, b)
				}
			}
		})
	}
}

// bitbucketCloudBranchesPage renders a Bitbucket Cloud `refs/branches` response for the given
// branch names. `next` is emitted only when non-empty, mirroring the API omitting the field on the
// final page. Only `name` and `target.hash` are populated, since those are the only fields
// listBranches consumes -- the fixtures above carry the full shape already.
func bitbucketCloudBranchesPage(page int, next string, names ...string) string {
	values := make([]string, 0, len(names))
	for i, name := range names {
		values = append(values, fmt.Sprintf(`{"name": %q, "type": "branch", "target": {"hash": %q}}`,
			name, fmt.Sprintf("%040d", page*100+i)))
	}
	nextField := ""
	if next != "" {
		nextField = fmt.Sprintf(`"next": %q,`, next)
	}
	return fmt.Sprintf(`{"pagelen": %d, "page": %d, "size": %d, %s "values": [%s]}`,
		bitbucketCloudMaxPageLen, page, len(names), nextField, strings.Join(values, ","))
}

// bitbucketCloudBranchServer serves the branch-listing endpoint and records every request's query
// string so tests can assert on the pagination parameters that were actually sent. respond is
// called with the requested page number (1 when unset) and returns the body to write.
func bitbucketCloudBranchServer(t *testing.T, respond func(page string) string) (*httptest.Server, func() []url.Values) {
	t.Helper()

	var mu sync.Mutex
	var queries []url.Values

	server := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if !strings.HasPrefix(req.URL.Path, "/repositories/test-owner/testmike/refs/branches") {
			res.WriteHeader(http.StatusNotFound)
			return
		}

		mu.Lock()
		queries = append(queries, req.URL.Query())
		mu.Unlock()

		page := req.URL.Query().Get("page")
		if page == "" {
			page = "1"
		}
		body := respond(page)
		if body == "" {
			res.WriteHeader(http.StatusInternalServerError)
			return
		}
		res.WriteHeader(http.StatusOK)
		// assert rather than require: this runs on the server's goroutine, where FailNow is illegal.
		_, err := res.Write([]byte(body))
		assert.NoError(t, err)
	}))

	return server, func() []url.Values {
		mu.Lock()
		defer mu.Unlock()
		return queries
	}
}

func bitbucketCloudBranchNames(t *testing.T, repos []*Repository) []string {
	t.Helper()
	names := make([]string, 0, len(repos))
	for _, r := range repos {
		names = append(names, r.Branch)
	}
	return names
}

// https://github.com/argoproj/argo-cd/issues/24081
//
// Bitbucket Cloud's refs/branches endpoint is paginated and defaults to a short page. Without
// following the `next` link the SCM provider generator only ever sees the first page, so an
// ApplicationSet silently generates Applications for a subset of the repository's branches.
func TestBitbucketCloudGetBranchesPagination(t *testing.T) {
	t.Run("follows next links across every page", func(t *testing.T) {
		server, queries := bitbucketCloudBranchServer(t, func(page string) string {
			switch page {
			case "1":
				return bitbucketCloudBranchesPage(1, "https://next/page/2", "main", "feature-a")
			case "2":
				return bitbucketCloudBranchesPage(2, "https://next/page/3", "feature-b", "feature-c")
			case "3":
				// Final page: no next link.
				return bitbucketCloudBranchesPage(3, "", "release-1.0")
			default:
				t.Errorf("requested unexpected page %q", page)
				return ""
			}
		})
		defer server.Close()
		t.Setenv("BITBUCKET_API_BASE_URL", server.URL)

		provider, err := NewBitBucketCloudProvider("test-owner", "user", "password", true)
		require.NoError(t, err)

		repos, err := provider.GetBranches(t.Context(), &Repository{
			Organization: "test-owner",
			Repository:   "testmike",
		})
		require.NoError(t, err)

		// Every branch from all three pages, in the order the API returned them.
		assert.Equal(t,
			[]string{"main", "feature-a", "feature-b", "feature-c", "release-1.0"},
			bitbucketCloudBranchNames(t, repos))

		// Requests pages 1..3 and stops -- it does not keep going past the page with no next link.
		sent := queries()
		require.Len(t, sent, 3)
		for i, q := range sent {
			assert.Equal(t, fmt.Sprint(i+1), q.Get("page"), "request %d asked for the wrong page", i)
			assert.Equal(t, "100", q.Get("pagelen"), "request %d did not ask for the max page length", i)
		}
	})

	t.Run("stops after a single request when there is no next link", func(t *testing.T) {
		server, queries := bitbucketCloudBranchServer(t, func(page string) string {
			if page != "1" {
				t.Errorf("expected exactly one request, got a request for page %q", page)
				return ""
			}
			return bitbucketCloudBranchesPage(1, "", "main")
		})
		defer server.Close()
		t.Setenv("BITBUCKET_API_BASE_URL", server.URL)

		provider, err := NewBitBucketCloudProvider("test-owner", "user", "password", true)
		require.NoError(t, err)

		repos, err := provider.GetBranches(t.Context(), &Repository{
			Organization: "test-owner",
			Repository:   "testmike",
		})
		require.NoError(t, err)

		assert.Equal(t, []string{"main"}, bitbucketCloudBranchNames(t, repos))
		assert.Len(t, queries(), 1)
	})

	t.Run("terminates when a next link is served with an empty page", func(t *testing.T) {
		// A next link that never clears would spin forever against a live API. The mock stops
		// advertising next after a handful of requests so a regression fails the length assertion
		// below instead of hanging the suite.
		const bailOut = 6
		var served int
		var mu sync.Mutex

		server, queries := bitbucketCloudBranchServer(t, func(page string) string {
			mu.Lock()
			served++
			count := served
			mu.Unlock()

			if count >= bailOut {
				return bitbucketCloudBranchesPage(count, "", "escape-hatch")
			}
			if page == "1" {
				return bitbucketCloudBranchesPage(1, "https://next/page/2", "main")
			}
			// Still advertising a next page, but with nothing on it.
			return bitbucketCloudBranchesPage(2, "https://next/page/3")
		})
		defer server.Close()
		t.Setenv("BITBUCKET_API_BASE_URL", server.URL)

		provider, err := NewBitBucketCloudProvider("test-owner", "user", "password", true)
		require.NoError(t, err)

		repos, err := provider.GetBranches(t.Context(), &Repository{
			Organization: "test-owner",
			Repository:   "testmike",
		})
		require.NoError(t, err)

		assert.Equal(t, []string{"main"}, bitbucketCloudBranchNames(t, repos))
		assert.Len(t, queries(), 2, "an empty page must end the loop rather than being requested again")
	})

	t.Run("propagates an error from a later page", func(t *testing.T) {
		server, _ := bitbucketCloudBranchServer(t, func(page string) string {
			if page == "1" {
				return bitbucketCloudBranchesPage(1, "https://next/page/2", "main")
			}
			return "" // handler turns this into a 500
		})
		defer server.Close()
		t.Setenv("BITBUCKET_API_BASE_URL", server.URL)

		provider, err := NewBitBucketCloudProvider("test-owner", "user", "password", true)
		require.NoError(t, err)

		_, err = provider.GetBranches(t.Context(), &Repository{
			Organization: "test-owner",
			Repository:   "testmike",
		})
		require.Error(t, err)
		// The failure must name the repository, not just bubble up a bare transport error.
		assert.Contains(t, err.Error(), "test-owner/testmike")
	})

	t.Run("does not paginate when allBranches is disabled", func(t *testing.T) {
		var mu sync.Mutex
		var paths []string
		var queries []url.Values

		server := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			mu.Lock()
			paths = append(paths, req.URL.Path)
			queries = append(queries, req.URL.Query())
			mu.Unlock()

			res.WriteHeader(http.StatusOK)
			_, err := res.Write([]byte(`{"name": "main", "type": "branch",
				"target": {"hash": "dc1edb6c7d650d8ba67719ddf7b662ad8f8fb798"}}`))
			assert.NoError(t, err)
		}))
		defer server.Close()
		t.Setenv("BITBUCKET_API_BASE_URL", server.URL)

		provider, err := NewBitBucketCloudProvider("test-owner", "user", "password", false)
		require.NoError(t, err)

		repos, err := provider.GetBranches(t.Context(), &Repository{
			Organization: "test-owner",
			Repository:   "testmike",
			Branch:       "main",
		})
		require.NoError(t, err)

		assert.Equal(t, []string{"main"}, bitbucketCloudBranchNames(t, repos))
		// Single-branch lookups hit the per-branch endpoint and carry no pagination parameters.
		require.Len(t, paths, 1)
		assert.Equal(t, "/repositories/test-owner/testmike/refs/branches/main", paths[0])
		assert.Empty(t, queries[0].Get("pagelen"))
		assert.Empty(t, queries[0].Get("page"))
	})
}
