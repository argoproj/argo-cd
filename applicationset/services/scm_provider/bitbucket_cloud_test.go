package scm_provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
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

// The shape below mirrors a real response from
// https://api.bitbucket.org/2.0/repositories/atlassian/aui/refs/branches?pagelen=2 -- a public
// repository with 356 branches. `next` is an absolute URL that repeats the query parameters of the
// request that produced it, and is omitted entirely on the final page.
//
// Only `name` and `target.hash` are populated here, since those are the only fields listBranches
// consumes; the fixtures earlier in this file carry the full branch shape already.
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

const bitbucketCloudBranchesPath = "/repositories/test-owner/testmike/refs/branches"

// bitbucketCloudBranchServer serves the branch-listing endpoint and records the full request URI of
// every call, so tests can assert that pagination followed the URLs the API handed back rather than
// URLs it built itself. respond receives the recorded request and returns the body to write; an
// empty body becomes a 500.
func bitbucketCloudBranchServer(t *testing.T, respond func(req *http.Request) string) (*httptest.Server, func() []string) {
	t.Helper()

	var mu sync.Mutex
	var requests []string

	server := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if !strings.HasPrefix(req.URL.Path, bitbucketCloudBranchesPath) {
			res.WriteHeader(http.StatusNotFound)
			return
		}

		mu.Lock()
		requests = append(requests, req.URL.RequestURI())
		mu.Unlock()

		body := respond(req)
		if body == "" {
			res.WriteHeader(http.StatusInternalServerError)
			return
		}
		res.WriteHeader(http.StatusOK)
		// assert rather than require: this runs on the server's goroutine, where FailNow is illegal.
		_, err := res.Write([]byte(body))
		assert.NoError(t, err)
	}))

	return server, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return requests
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

func bitbucketCloudProvider(t *testing.T, serverURL string, allBranches bool) *BitBucketCloudProvider {
	t.Helper()
	t.Setenv("BITBUCKET_API_BASE_URL", serverURL)
	provider, err := NewBitBucketCloudProvider("test-owner", "user", "password", allBranches)
	require.NoError(t, err)
	return provider
}

func bitbucketCloudGetBranches(t *testing.T, provider *BitBucketCloudProvider) ([]*Repository, error) {
	t.Helper()
	return provider.GetBranches(t.Context(), &Repository{
		Organization: "test-owner",
		Repository:   "testmike",
	})
}

// https://github.com/argoproj/argo-cd/issues/24081
//
// Bitbucket Cloud's refs/branches endpoint is paginated and defaults to a short page. Without
// following the `next` link the SCM provider generator only ever sees the first page, so an
// ApplicationSet silently generates Applications for a subset of the repository's branches.
func TestBitbucketCloudGetBranchesPagination(t *testing.T) {
	t.Run("follows the next link the api returned on every page", func(t *testing.T) {
		var serverURL string
		server, requests := bitbucketCloudBranchServer(t, func(req *http.Request) string {
			switch req.URL.RequestURI() {
			case bitbucketCloudBranchesPath + "?pagelen=100":
				return bitbucketCloudBranchesPage(1, serverURL+bitbucketCloudBranchesPath+"?pagelen=100&page=2", "main", "feature-a")
			case bitbucketCloudBranchesPath + "?pagelen=100&page=2":
				return bitbucketCloudBranchesPage(2, serverURL+bitbucketCloudBranchesPath+"?pagelen=100&page=3", "feature-b", "feature-c")
			case bitbucketCloudBranchesPath + "?pagelen=100&page=3":
				// Final page: no next link.
				return bitbucketCloudBranchesPage(3, "", "release-1.0")
			default:
				t.Errorf("unexpected request %q", req.URL.RequestURI())
				return ""
			}
		})
		defer server.Close()
		serverURL = server.URL

		repos, err := bitbucketCloudGetBranches(t, bitbucketCloudProvider(t, server.URL, true))
		require.NoError(t, err)

		// Every branch from all three pages, in the order the API returned them.
		assert.Equal(t,
			[]string{"main", "feature-a", "feature-b", "feature-c", "release-1.0"},
			bitbucketCloudBranchNames(t, repos))

		// Exactly the first page plus the two URLs the API advertised, and nothing after the page
		// that carried no next link.
		assert.Equal(t, []string{
			bitbucketCloudBranchesPath + "?pagelen=100",
			bitbucketCloudBranchesPath + "?pagelen=100&page=2",
			bitbucketCloudBranchesPath + "?pagelen=100&page=3",
		}, requests())
	})

	// Bitbucket documents two pagination styles. List-based collections expose a numeric `page`,
	// but iterator-based ones put an unpredictable token in the next link, and the docs are explicit
	// that clients should not construct these links themselves. Reconstructing the URL as page+1
	// would re-request page 2 forever here; go-bitbucket cannot even represent a token page, since
	// RepositoryBranchOptions.PageNum is an int.
	t.Run("follows a next link carrying an opaque token instead of a page number", func(t *testing.T) {
		const tokenPage = bitbucketCloudBranchesPath + "?pagelen=100&ctx=9c2c0a1f&page=eyJ0b2tlbiI6ICJhYmMifQ%3D%3D"

		var serverURL string
		server, requests := bitbucketCloudBranchServer(t, func(req *http.Request) string {
			switch req.URL.RequestURI() {
			case bitbucketCloudBranchesPath + "?pagelen=100":
				return bitbucketCloudBranchesPage(1, serverURL+tokenPage, "main")
			case tokenPage:
				return bitbucketCloudBranchesPage(2, "", "feature-a")
			default:
				t.Errorf("unexpected request %q", req.URL.RequestURI())
				return ""
			}
		})
		defer server.Close()
		serverURL = server.URL

		repos, err := bitbucketCloudGetBranches(t, bitbucketCloudProvider(t, server.URL, true))
		require.NoError(t, err)

		assert.Equal(t, []string{"main", "feature-a"}, bitbucketCloudBranchNames(t, repos))
		// The token URL is passed through byte for byte, query parameters and all.
		assert.Equal(t, []string{bitbucketCloudBranchesPath + "?pagelen=100", tokenPage}, requests())
	})

	t.Run("sends credentials when following a next link", func(t *testing.T) {
		var serverURL string
		var mu sync.Mutex
		var authHeaders []string

		server, _ := bitbucketCloudBranchServer(t, func(req *http.Request) string {
			mu.Lock()
			authHeaders = append(authHeaders, req.Header.Get("Authorization"))
			mu.Unlock()

			if req.URL.Query().Get("page") == "" {
				return bitbucketCloudBranchesPage(1, serverURL+bitbucketCloudBranchesPath+"?pagelen=100&page=2", "main")
			}
			return bitbucketCloudBranchesPage(2, "", "feature-a")
		})
		defer server.Close()
		serverURL = server.URL

		_, err := bitbucketCloudGetBranches(t, bitbucketCloudProvider(t, server.URL, true))
		require.NoError(t, err)

		// Private repositories are the common case, so the followed page must carry the same
		// credentials the first page did.
		mu.Lock()
		defer mu.Unlock()
		require.Len(t, authHeaders, 2)
		assert.NotEmpty(t, authHeaders[1])
		assert.Equal(t, authHeaders[0], authHeaders[1])
	})

	// The followed URL comes out of a response body and the request carries the provider's
	// credentials, so a next link pointing somewhere else must not be followed.
	t.Run("refuses a next link pointing at another host", func(t *testing.T) {
		server, requests := bitbucketCloudBranchServer(t, func(_ *http.Request) string {
			return bitbucketCloudBranchesPage(1, "https://attacker.example.com/repositories/test-owner/testmike/refs/branches?page=2", "main")
		})
		defer server.Close()

		_, err := bitbucketCloudGetBranches(t, bitbucketCloudProvider(t, server.URL, true))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "refusing to follow next page url")
		assert.Contains(t, err.Error(), "attacker.example.com")
		// It stopped at the first page rather than reaching out to the other host.
		assert.Len(t, requests(), 1)
	})

	// A next link that never advances would otherwise spin against the API forever.
	t.Run("fails instead of looping when a next link repeats", func(t *testing.T) {
		var serverURL string
		server, requests := bitbucketCloudBranchServer(t, func(_ *http.Request) string {
			return bitbucketCloudBranchesPage(1, serverURL+bitbucketCloudBranchesPath+"?pagelen=100&page=2", "main")
		})
		defer server.Close()
		serverURL = server.URL

		_, err := bitbucketCloudGetBranches(t, bitbucketCloudProvider(t, server.URL, true))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "repeated next page url")
		// First page, then the repeated URL once -- it is not requested a third time.
		assert.Len(t, requests(), 2)
	})

	t.Run("stops after a single request when there is no next link", func(t *testing.T) {
		server, requests := bitbucketCloudBranchServer(t, func(req *http.Request) string {
			if req.URL.RequestURI() != bitbucketCloudBranchesPath+"?pagelen=100" {
				t.Errorf("expected exactly one request, also got %q", req.URL.RequestURI())
				return ""
			}
			return bitbucketCloudBranchesPage(1, "", "main")
		})
		defer server.Close()

		repos, err := bitbucketCloudGetBranches(t, bitbucketCloudProvider(t, server.URL, true))
		require.NoError(t, err)

		assert.Equal(t, []string{"main"}, bitbucketCloudBranchNames(t, repos))
		assert.Len(t, requests(), 1)
	})

	t.Run("propagates an error from a followed page", func(t *testing.T) {
		var serverURL string
		server, _ := bitbucketCloudBranchServer(t, func(req *http.Request) string {
			if req.URL.Query().Get("page") == "" {
				return bitbucketCloudBranchesPage(1, serverURL+bitbucketCloudBranchesPath+"?pagelen=100&page=2", "main")
			}
			return "" // handler turns this into a 500
		})
		defer server.Close()
		serverURL = server.URL

		_, err := bitbucketCloudGetBranches(t, bitbucketCloudProvider(t, server.URL, true))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "500 Internal Server Error")
		// The failure must name the repository, not just bubble up a bare transport error.
		assert.Contains(t, err.Error(), "test-owner/testmike")
	})

	t.Run("does not paginate when allBranches is disabled", func(t *testing.T) {
		var mu sync.Mutex
		var requests []string

		server := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			mu.Lock()
			requests = append(requests, req.URL.RequestURI())
			mu.Unlock()

			res.WriteHeader(http.StatusOK)
			_, err := res.Write([]byte(`{"name": "main", "type": "branch",
				"target": {"hash": "dc1edb6c7d650d8ba67719ddf7b662ad8f8fb798"}}`))
			assert.NoError(t, err)
		}))
		defer server.Close()

		provider := bitbucketCloudProvider(t, server.URL, false)
		repos, err := provider.GetBranches(t.Context(), &Repository{
			Organization: "test-owner",
			Repository:   "testmike",
			Branch:       "main",
		})
		require.NoError(t, err)

		assert.Equal(t, []string{"main"}, bitbucketCloudBranchNames(t, repos))
		// Single-branch lookups hit the per-branch endpoint and carry no pagination parameters.
		assert.Equal(t, []string{bitbucketCloudBranchesPath + "/main"}, requests)
	})
}
