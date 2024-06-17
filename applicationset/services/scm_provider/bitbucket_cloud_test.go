package scm_provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
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
			provider, _ := NewBitBucketCloudProvider(context.Background(), c.owner, "user", "password", false)
			repo := &Repository{
				Organization: c.owner,
				Repository:   c.repo,
				SHA:          c.sha,
				Branch:       "main",
			}
			hasPath, err := provider.RepoHasPath(context.Background(), repo, c.path)
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
			provider, _ := NewBitBucketCloudProvider(context.Background(), c.owner, "user", "password", c.allBranches)
			rawRepos, err := ListRepos(context.Background(), provider, c.filters, c.proto)
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
