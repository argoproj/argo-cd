package pull_request

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func defaultHandlerCloud(t *testing.T) func(http.ResponseWriter, *http.Request) {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var err error
		switch r.RequestURI {
		case "/repositories/OWNER/REPO/pullrequests/":
			_, err = io.WriteString(w, `{
					"size": 1,
					"pagelen": 10,
					"page": 1,
					"values": [
						{
							"id": 101,
							"title": "feat(foo-bar)",
							"source": {
								"branch": {
									"name": "feature/foo-bar"
								},
								"commit": {
									"type": "commit",
									"hash": "1a8dd249c04a"
								}
							},
							"author": {
								"nickname": "testName"
							}
						}
					]
				}`)
		case "/repositories/OWNER/REPO/commit/1a8dd249c04a":
			// commit reachability check — 200 so the PR is not filtered out
			_, err = io.WriteString(w, `{"hash": "1a8dd249c04a"}`)
		default:
			t.Fail()
		}
		if err != nil {
			t.Fail()
		}
	}
}

func TestParseUrlEmptyUrl(t *testing.T) {
	url, err := parseURL("")
	bitbucketURL, _ := url.Parse("https://api.bitbucket.org/2.0")

	require.NoError(t, err)
	assert.Equal(t, bitbucketURL, url)
}

func TestInvalidBaseUrlBasicAuthCloud(t *testing.T) {
	_, err := NewBitbucketCloudServiceBasicAuth("http:// example.org", "user", "password", "OWNER", "REPO", nil)

	require.Error(t, err)
}

func TestInvalidBaseUrlBearerTokenCloud(t *testing.T) {
	_, err := NewBitbucketCloudServiceBearerToken("http:// example.org", "TOKEN", "OWNER", "REPO", nil)

	require.Error(t, err)
}

func TestInvalidBaseUrlNoAuthCloud(t *testing.T) {
	_, err := NewBitbucketCloudServiceNoAuth("http:// example.org", "OWNER", "REPO", nil)

	require.Error(t, err)
}

func TestListPullRequestBearerTokenCloud(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer TOKEN", r.Header.Get("Authorization"))
		defaultHandlerCloud(t)(w, r)
	}))
	defer ts.Close()
	svc, err := NewBitbucketCloudServiceBearerToken(ts.URL, "TOKEN", "OWNER", "REPO", nil)
	require.NoError(t, err)
	pullRequests, err := ListPullRequests(t.Context(), svc, []v1alpha1.PullRequestGeneratorFilter{})
	require.NoError(t, err)
	assert.Len(t, pullRequests, 1)
	assert.Equal(t, int64(101), pullRequests[0].Number)
	assert.Equal(t, "feat(foo-bar)", pullRequests[0].Title)
	assert.Equal(t, "feature/foo-bar", pullRequests[0].Branch)
	assert.Equal(t, "1a8dd249c04a", pullRequests[0].HeadSHA)
	assert.Equal(t, "testName", pullRequests[0].Author)
}

func TestListPullRequestNoAuthCloud(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))
		defaultHandlerCloud(t)(w, r)
	}))
	defer ts.Close()
	svc, err := NewBitbucketCloudServiceNoAuth(ts.URL, "OWNER", "REPO", nil)
	require.NoError(t, err)
	pullRequests, err := ListPullRequests(t.Context(), svc, []v1alpha1.PullRequestGeneratorFilter{})
	require.NoError(t, err)
	assert.Len(t, pullRequests, 1)
	assert.Equal(t, int64(101), pullRequests[0].Number)
	assert.Equal(t, "feat(foo-bar)", pullRequests[0].Title)
	assert.Equal(t, "feature/foo-bar", pullRequests[0].Branch)
	assert.Equal(t, "1a8dd249c04a", pullRequests[0].HeadSHA)
	assert.Equal(t, "testName", pullRequests[0].Author)
}

func TestListPullRequestBasicAuthCloud(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Basic dXNlcjpwYXNzd29yZA==", r.Header.Get("Authorization"))
		defaultHandlerCloud(t)(w, r)
	}))
	defer ts.Close()
	svc, err := NewBitbucketCloudServiceBasicAuth(ts.URL, "user", "password", "OWNER", "REPO", nil)
	require.NoError(t, err)
	pullRequests, err := ListPullRequests(t.Context(), svc, []v1alpha1.PullRequestGeneratorFilter{})
	require.NoError(t, err)
	assert.Len(t, pullRequests, 1)
	assert.Equal(t, int64(101), pullRequests[0].Number)
	assert.Equal(t, "feat(foo-bar)", pullRequests[0].Title)
	assert.Equal(t, "feature/foo-bar", pullRequests[0].Branch)
	assert.Equal(t, "1a8dd249c04a", pullRequests[0].HeadSHA)
	assert.Equal(t, "testName", pullRequests[0].Author)
}

func TestListPullRequestPaginationCloud(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var err error
		switch r.RequestURI {
		case "/repositories/OWNER/REPO/pullrequests/":
			_, err = fmt.Fprintf(w, `{
				"size": 2,
				"pagelen": 1,
				"page": 1,
				"next": "http://%s/repositories/OWNER/REPO/pullrequests/?pagelen=1&page=2",
				"values": [
					{
						"id": 101,
						"title": "feat(101)",
						"source": {
							"branch": {
								"name": "feature-101"
							},
							"commit": {
								"type": "commit",
								"hash": "1a8dd249c04a"
							}
						},
						"author": {
							"nickname": "testName"
						}
					},
					{
						"id": 102,
						"title": "feat(102)",
						"source": {
							"branch": {
								"name": "feature-102"
							},
							"commit": {
								"type": "commit",
								"hash": "4cf807e67a6d"
							}
						},
						"author": {
							"nickname": "testName"
						}
					}
				]
			}`, r.Host)
		case "/repositories/OWNER/REPO/pullrequests/?pagelen=1&page=2":
			_, err = fmt.Fprintf(w, `{
				"size": 2,
				"pagelen": 1,
				"page": 2,
				"previous": "http://%s/repositories/OWNER/REPO/pullrequests/?pagelen=1&page=1",
				"values": [
					{
						"id": 103,
						"title": "feat(103)",
						"source": {
							"branch": {
								"name": "feature-103"
							},
							"commit": {
								"type": "commit",
								"hash": "6344d9623e3b"
							}
						},
						"author": {
							"nickname": "testName"
						}
					}
				]
			}`, r.Host)
		case "/repositories/OWNER/REPO/commit/1a8dd249c04a", "/repositories/OWNER/REPO/commit/4cf807e67a6d", "/repositories/OWNER/REPO/commit/6344d9623e3b", "/repositories/OWNER/REPO/commit/4a8dd249c04a", "/repositories/OWNER/REPO/commit/2a8dd249c04a":
			w.WriteHeader(http.StatusOK)
		default:
			t.Fail()
		}
		if err != nil {
			t.Fail()
		}
	}))
	defer ts.Close()
	svc, err := NewBitbucketCloudServiceNoAuth(ts.URL, "OWNER", "REPO", nil)
	require.NoError(t, err)
	pullRequests, err := ListPullRequests(t.Context(), svc, []v1alpha1.PullRequestGeneratorFilter{})
	require.NoError(t, err)
	assert.Len(t, pullRequests, 3)
	assert.Equal(t, PullRequest{
		Number:  101,
		Title:   "feat(101)",
		Branch:  "feature-101",
		HeadSHA: "1a8dd249c04a",
		Author:  "testName",
	}, *pullRequests[0])
	assert.Equal(t, PullRequest{
		Number:  102,
		Title:   "feat(102)",
		Branch:  "feature-102",
		HeadSHA: "4cf807e67a6d",
		Author:  "testName",
	}, *pullRequests[1])
	assert.Equal(t, PullRequest{
		Number:  103,
		Title:   "feat(103)",
		Branch:  "feature-103",
		HeadSHA: "6344d9623e3b",
		Author:  "testName",
	}, *pullRequests[2])
}

func TestListResponseErrorCloud(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()
	svc, _ := NewBitbucketCloudServiceNoAuth(ts.URL, "OWNER", "REPO", nil)
	_, err := ListPullRequests(t.Context(), svc, []v1alpha1.PullRequestGeneratorFilter{})
	require.Error(t, err)
}

func TestListResponseMalformedCloud(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.RequestURI {
		case "/repositories/OWNER/REPO/pullrequests/":
			_, err := io.WriteString(w, `[{
				"size": 1,
				"pagelen": 10,
				"page": 1,
				"values": [{ "id": 101 }]
			}]`)
			if err != nil {
				t.Fail()
			}
		default:
			t.Fail()
		}
	}))
	defer ts.Close()
	svc, _ := NewBitbucketCloudServiceNoAuth(ts.URL, "OWNER", "REPO", nil)
	_, err := ListPullRequests(t.Context(), svc, []v1alpha1.PullRequestGeneratorFilter{})
	require.Error(t, err)
}

func TestListResponseMalformedValuesCloud(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.RequestURI {
		case "/repositories/OWNER/REPO/pullrequests/":
			_, err := io.WriteString(w, `{
				"size": 1,
				"pagelen": 10,
				"page": 1,
				"values": { "id": 101 }
			}`)
			if err != nil {
				t.Fail()
			}
		default:
			t.Fail()
		}
	}))
	defer ts.Close()
	svc, _ := NewBitbucketCloudServiceNoAuth(ts.URL, "OWNER", "REPO", nil)
	_, err := ListPullRequests(t.Context(), svc, []v1alpha1.PullRequestGeneratorFilter{})
	require.Error(t, err)
}

func TestListResponseEmptyCloud(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.RequestURI {
		case "/repositories/OWNER/REPO/pullrequests/":
			_, err := io.WriteString(w, `{
				"size": 1,
				"pagelen": 10,
				"page": 1,
				"values": []
			}`)
			if err != nil {
				t.Fail()
			}
		default:
			t.Fail()
		}
	}))
	defer ts.Close()
	svc, err := NewBitbucketCloudServiceNoAuth(ts.URL, "OWNER", "REPO", nil)
	require.NoError(t, err)
	pullRequests, err := ListPullRequests(t.Context(), svc, []v1alpha1.PullRequestGeneratorFilter{})
	require.NoError(t, err)
	assert.Empty(t, pullRequests)
}

func TestListPullRequestBranchMatchCloud(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var err error
		switch r.RequestURI {
		case "/repositories/OWNER/REPO/pullrequests/":
			_, err = fmt.Fprintf(w, `{
				"size": 2,
				"pagelen": 1,
				"page": 1,
				"next": "http://%s/repositories/OWNER/REPO/pullrequests/?pagelen=1&page=2",
				"values": [
					{
						"id": 101,
						"title": "feat(101)",
						"source": {
							"branch": {
								"name": "feature-101"
							},
							"commit": {
								"type": "commit",
								"hash": "1a8dd249c04a"
							}
						},
						"author": {
							"nickname": "testName"
						},
						"destination": {
							"branch": {
								"name": "master"
							}
						}
					},
					{
						"id": 200,
						"title": "feat(200)",
						"source": {
							"branch": {
								"name": "feature-200"
							},
							"commit": {
								"type": "commit",
								"hash": "4cf807e67a6d"
							}
						},
						"author": {
							"nickname": "testName"
						},
						"destination": {
							"branch": {
								"name": "branch-200"
							}
						}
					}
				]
			}`, r.Host)
		case "/repositories/OWNER/REPO/pullrequests/?pagelen=1&page=2":
			_, err = fmt.Fprintf(w, `{
				"size": 2,
				"pagelen": 1,
				"page": 2,
				"previous": "http://%s/repositories/OWNER/REPO/pullrequests/?pagelen=1&page=1",
				"values": [
					{
						"id": 102,
						"title": "feat(102)",
						"source": {
							"branch": {
								"name": "feature-102"
							},
							"commit": {
								"type": "commit",
								"hash": "6344d9623e3b"
							}
						},
						"author": {
							"nickname": "testName"
						},
						"destination": {
							"branch": {
								"name": "master"
							}
						}
					}
				]
			}`, r.Host)
		case "/repositories/OWNER/REPO/commit/1a8dd249c04a", "/repositories/OWNER/REPO/commit/4cf807e67a6d", "/repositories/OWNER/REPO/commit/6344d9623e3b", "/repositories/OWNER/REPO/commit/4a8dd249c04a", "/repositories/OWNER/REPO/commit/2a8dd249c04a":
			w.WriteHeader(http.StatusOK)
		default:
			t.Fail()
		}
		if err != nil {
			t.Fail()
		}
	}))
	defer ts.Close()
	regexp := `feature-1[\d]{2}`
	svc, err := NewBitbucketCloudServiceNoAuth(ts.URL, "OWNER", "REPO", nil)
	require.NoError(t, err)
	pullRequests, err := ListPullRequests(t.Context(), svc, []v1alpha1.PullRequestGeneratorFilter{
		{
			BranchMatch: &regexp,
		},
	})
	require.NoError(t, err)
	assert.Len(t, pullRequests, 2)
	assert.Equal(t, PullRequest{
		Number:       101,
		Title:        "feat(101)",
		Branch:       "feature-101",
		HeadSHA:      "1a8dd249c04a",
		Author:       "testName",
		TargetBranch: "master",
	}, *pullRequests[0])
	assert.Equal(t, PullRequest{
		Number:       102,
		Title:        "feat(102)",
		Branch:       "feature-102",
		HeadSHA:      "6344d9623e3b",
		Author:       "testName",
		TargetBranch: "master",
	}, *pullRequests[1])

	regexp = `.*2$`
	svc, err = NewBitbucketCloudServiceNoAuth(ts.URL, "OWNER", "REPO", nil)
	require.NoError(t, err)
	pullRequests, err = ListPullRequests(t.Context(), svc, []v1alpha1.PullRequestGeneratorFilter{
		{
			BranchMatch: &regexp,
		},
	})
	require.NoError(t, err)
	assert.Len(t, pullRequests, 1)
	assert.Equal(t, PullRequest{
		Number:       102,
		Title:        "feat(102)",
		Branch:       "feature-102",
		HeadSHA:      "6344d9623e3b",
		Author:       "testName",
		TargetBranch: "master",
	}, *pullRequests[0])

	regexp = `[\d{2}`
	svc, err = NewBitbucketCloudServiceNoAuth(ts.URL, "OWNER", "REPO", nil)
	require.NoError(t, err)
	_, err = ListPullRequests(t.Context(), svc, []v1alpha1.PullRequestGeneratorFilter{
		{
			BranchMatch: &regexp,
		},
	})
	require.Error(t, err)

	regexp = `feature-2[\d]{2}`
	targetRegexp := `branch.*`
	pullRequests, err = ListPullRequests(t.Context(), svc, []v1alpha1.PullRequestGeneratorFilter{
		{
			BranchMatch:       &regexp,
			TargetBranchMatch: &targetRegexp,
		},
	})
	require.NoError(t, err)
	assert.Len(t, pullRequests, 1)
	assert.Equal(t, PullRequest{
		Number:       200,
		Title:        "feat(200)",
		Branch:       "feature-200",
		HeadSHA:      "4cf807e67a6d",
		Author:       "testName",
		TargetBranch: "branch-200",
	}, *pullRequests[0])
}

func TestBitbucketCloudListReturnsRepositoryNotFoundError(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	path := "/repositories/nonexistent/nonexistent/pullrequests/"

	mux.HandleFunc(path, func(w http.ResponseWriter, _ *http.Request) {
		// Return 404 status to simulate repository not found
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message": "404 Project Not Found"}`))
	})

	svc, err := NewBitbucketCloudServiceNoAuth(server.URL, "nonexistent", "nonexistent", nil)
	require.NoError(t, err)

	prs, err := svc.List(t.Context())

	// Should return empty pull requests list
	assert.Empty(t, prs)

	// Should return RepositoryNotFoundError
	require.Error(t, err)
	assert.True(t, IsRepositoryNotFoundError(err), "Expected RepositoryNotFoundError but got: %v", err)
}

func TestBitbucketCloudListUsesHintAndBypassesAPI(t *testing.T) {
	// Verify that a hint seeded in PRHintStore is returned by List() without hitting
	// the Bitbucket Cloud PR list API. The API server below returns 500 — if the hint
	// is not consumed, List() would return an error.
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/repositories/myorg/myrepo/pullrequests/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	hints := &PRHintStore{}
	hints.Set("myorg", "myrepo", []*PullRequest{
		{Number: 42, Title: "feat: test PR", Branch: "feat/test", TargetBranch: "main", HeadSHA: "abc123"},
	})

	svc, err := NewBitbucketCloudServiceNoAuth(server.URL, "myorg", "myrepo", hints)
	require.NoError(t, err)

	prs, err := svc.List(t.Context())
	require.NoError(t, err)
	require.Len(t, prs, 1)
	assert.Equal(t, int64(42), prs[0].Number)
	assert.Equal(t, "feat/test", prs[0].Branch)

	// Second call: hint was consumed, falls back to API (which 500s)
	_, err = svc.List(t.Context())
	assert.Error(t, err)
}

func TestBitbucketCloudListSkipsUnreachableCommit(t *testing.T) {
	// PR whose head SHA returns 404 from the commit API should be skipped;
	// PR with a reachable SHA should be returned.
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/repositories/OWNER/REPO/pullrequests/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"values": [
			{"id": 1, "title": "reachable PR", "state": "OPEN",
			 "source": {"branch": {"name": "feat/a"}, "commit": {"hash": "aabbcc"}},
			 "destination": {"branch": {"name": "main"}},
			 "author": {"nickname": "alice"}},
			{"id": 2, "title": "deleted-branch PR", "state": "OPEN",
			 "source": {"branch": {"name": "feat/gone"}, "commit": {"hash": "deadbeef"}},
			 "destination": {"branch": {"name": "main"}},
			 "author": {"nickname": "bob"}}
		]}`))
	})
	// Reachable commit returns 200.
	mux.HandleFunc("/repositories/OWNER/REPO/commit/aabbcc", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"hash": "aabbcc"}`))
	})
	// Unreachable commit returns 404.
	mux.HandleFunc("/repositories/OWNER/REPO/commit/deadbeef", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	svc, err := NewBitbucketCloudServiceNoAuth(server.URL, "OWNER", "REPO", nil)
	require.NoError(t, err)

	prs, err := svc.List(t.Context())
	require.NoError(t, err)
	require.Len(t, prs, 1)
	assert.Equal(t, int64(1), prs[0].Number)
	assert.Equal(t, "aabbcc", prs[0].HeadSHA)
}
