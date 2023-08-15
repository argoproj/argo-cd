package pull_request

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func defaultHandlerCloud(t *testing.T) func(http.ResponseWriter, *http.Request) {
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
							"source": {
								"branch": {
									"name": "feature/foo-bar"
								},
								"commit": {
									"type": "commit",
									"hash": "1a8dd249c04a"
								}
							}
						}
					]
				}`)
		default:
			t.Fail()
		}
		if err != nil {
			t.Fail()
		}
	}
}

func TestParseUrlEmptyUrl(t *testing.T) {
	url, err := parseUrl("")
	bitbucketUrl, _ := url.Parse("https://api.bitbucket.org/2.0")

	assert.NoError(t, err)
	assert.Equal(t, bitbucketUrl, url)
}

func TestInvalidBaseUrlBasicAuthCloud(t *testing.T) {
	_, err := NewBitbucketCloudServiceBasicAuth("http:// example.org", "user", "password", "OWNER", "REPO")

	assert.Error(t, err)
}

func TestInvalidBaseUrlBearerTokenCloud(t *testing.T) {
	_, err := NewBitbucketCloudServiceBearerToken("http:// example.org", "TOKEN", "OWNER", "REPO")

	assert.Error(t, err)
}

func TestInvalidBaseUrlNoAuthCloud(t *testing.T) {
	_, err := NewBitbucketCloudServiceNoAuth("http:// example.org", "OWNER", "REPO")

	assert.Error(t, err)
}

func TestListPullRequestBearerTokenCloud(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer TOKEN", r.Header.Get("Authorization"))
		defaultHandlerCloud(t)(w, r)
	}))
	defer ts.Close()
	svc, err := NewBitbucketCloudServiceBearerToken(ts.URL, "TOKEN", "OWNER", "REPO")
	assert.NoError(t, err)
	pullRequests, err := ListPullRequests(context.Background(), svc, []v1alpha1.PullRequestGeneratorFilter{})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(pullRequests))
	assert.Equal(t, 101, pullRequests[0].Number)
	assert.Equal(t, "feature/foo-bar", pullRequests[0].Branch)
	assert.Equal(t, "1a8dd249c04a", pullRequests[0].HeadSHA)
}

func TestListPullRequestNoAuthCloud(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))
		defaultHandlerCloud(t)(w, r)
	}))
	defer ts.Close()
	svc, err := NewBitbucketCloudServiceNoAuth(ts.URL, "OWNER", "REPO")
	assert.NoError(t, err)
	pullRequests, err := ListPullRequests(context.Background(), svc, []v1alpha1.PullRequestGeneratorFilter{})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(pullRequests))
	assert.Equal(t, 101, pullRequests[0].Number)
	assert.Equal(t, "feature/foo-bar", pullRequests[0].Branch)
	assert.Equal(t, "1a8dd249c04a", pullRequests[0].HeadSHA)
}

func TestListPullRequestBasicAuthCloud(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Basic dXNlcjpwYXNzd29yZA==", r.Header.Get("Authorization"))
		defaultHandlerCloud(t)(w, r)
	}))
	defer ts.Close()
	svc, err := NewBitbucketCloudServiceBasicAuth(ts.URL, "user", "password", "OWNER", "REPO")
	assert.NoError(t, err)
	pullRequests, err := ListPullRequests(context.Background(), svc, []v1alpha1.PullRequestGeneratorFilter{})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(pullRequests))
	assert.Equal(t, 101, pullRequests[0].Number)
	assert.Equal(t, "feature/foo-bar", pullRequests[0].Branch)
	assert.Equal(t, "1a8dd249c04a", pullRequests[0].HeadSHA)
}

func TestListPullRequestPaginationCloud(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var err error
		switch r.RequestURI {
		case "/repositories/OWNER/REPO/pullrequests/":
			_, err = io.WriteString(w, fmt.Sprintf(`{
				"size": 2,
				"pagelen": 1,
				"page": 1,
				"next": "http://%s/repositories/OWNER/REPO/pullrequests/?pagelen=1&page=2",
				"values": [
					{
						"id": 101,
						"source": {
							"branch": {
								"name": "feature-101"
							},
							"commit": {
								"type": "commit",
								"hash": "1a8dd249c04a"
							}
						}
					},
					{
						"id": 102,
						"source": {
							"branch": {
								"name": "feature-102"
							},
							"commit": {
								"type": "commit",
								"hash": "4cf807e67a6d"
							}
						}
					}
				]
			}`, r.Host))
		case "/repositories/OWNER/REPO/pullrequests/?pagelen=1&page=2":
			_, err = io.WriteString(w, fmt.Sprintf(`{
				"size": 2,
				"pagelen": 1,
				"page": 2,
				"previous": "http://%s/repositories/OWNER/REPO/pullrequests/?pagelen=1&page=1",
				"values": [
					{
						"id": 103,
						"source": {
							"branch": {
								"name": "feature-103"
							},
							"commit": {
								"type": "commit",
								"hash": "6344d9623e3b"
							}
						}
					}
				]
			}`, r.Host))
		default:
			t.Fail()
		}
		if err != nil {
			t.Fail()
		}
	}))
	defer ts.Close()
	svc, err := NewBitbucketCloudServiceNoAuth(ts.URL, "OWNER", "REPO")
	assert.NoError(t, err)
	pullRequests, err := ListPullRequests(context.Background(), svc, []v1alpha1.PullRequestGeneratorFilter{})
	assert.NoError(t, err)
	assert.Equal(t, 3, len(pullRequests))
	assert.Equal(t, PullRequest{
		Number:  101,
		Branch:  "feature-101",
		HeadSHA: "1a8dd249c04a",
	}, *pullRequests[0])
	assert.Equal(t, PullRequest{
		Number:  102,
		Branch:  "feature-102",
		HeadSHA: "4cf807e67a6d",
	}, *pullRequests[1])
	assert.Equal(t, PullRequest{
		Number:  103,
		Branch:  "feature-103",
		HeadSHA: "6344d9623e3b",
	}, *pullRequests[2])
}

func TestListResponseErrorCloud(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer ts.Close()
	svc, _ := NewBitbucketCloudServiceNoAuth(ts.URL, "OWNER", "REPO")
	_, err := ListPullRequests(context.Background(), svc, []v1alpha1.PullRequestGeneratorFilter{})
	assert.Error(t, err)
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
	svc, _ := NewBitbucketCloudServiceNoAuth(ts.URL, "OWNER", "REPO")
	_, err := ListPullRequests(context.Background(), svc, []v1alpha1.PullRequestGeneratorFilter{})
	assert.Error(t, err)
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
	svc, _ := NewBitbucketCloudServiceNoAuth(ts.URL, "OWNER", "REPO")
	_, err := ListPullRequests(context.Background(), svc, []v1alpha1.PullRequestGeneratorFilter{})
	assert.Error(t, err)
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
	svc, err := NewBitbucketCloudServiceNoAuth(ts.URL, "OWNER", "REPO")
	assert.NoError(t, err)
	pullRequests, err := ListPullRequests(context.Background(), svc, []v1alpha1.PullRequestGeneratorFilter{})
	assert.NoError(t, err)
	assert.Empty(t, pullRequests)
}

func TestListPullRequestBranchMatchCloud(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var err error
		switch r.RequestURI {
		case "/repositories/OWNER/REPO/pullrequests/":
			_, err = io.WriteString(w, fmt.Sprintf(`{
				"size": 2,
				"pagelen": 1,
				"page": 1,
				"next": "http://%s/repositories/OWNER/REPO/pullrequests/?pagelen=1&page=2",
				"values": [
					{
						"id": 101,
						"source": {
							"branch": {
								"name": "feature-101"
							},
							"commit": {
								"type": "commit",
								"hash": "1a8dd249c04a"
							}
						}
					},
					{
						"id": 200,
						"source": {
							"branch": {
								"name": "feature-200"
							},
							"commit": {
								"type": "commit",
								"hash": "4cf807e67a6d"
							}
						}
					}
				]
			}`, r.Host))
		case "/repositories/OWNER/REPO/pullrequests/?pagelen=1&page=2":
			_, err = io.WriteString(w, fmt.Sprintf(`{
				"size": 2,
				"pagelen": 1,
				"page": 2,
				"previous": "http://%s/repositories/OWNER/REPO/pullrequests/?pagelen=1&page=1",
				"values": [
					{
						"id": 102,
						"source": {
							"branch": {
								"name": "feature-102"
							},
							"commit": {
								"type": "commit",
								"hash": "6344d9623e3b"
							}
						}
					}
				]
			}`, r.Host))
		default:
			t.Fail()
		}
		if err != nil {
			t.Fail()
		}
	}))
	defer ts.Close()
	regexp := `feature-1[\d]{2}`
	svc, err := NewBitbucketCloudServiceNoAuth(ts.URL, "OWNER", "REPO")
	assert.NoError(t, err)
	pullRequests, err := ListPullRequests(context.Background(), svc, []v1alpha1.PullRequestGeneratorFilter{
		{
			BranchMatch: &regexp,
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(pullRequests))
	assert.Equal(t, PullRequest{
		Number:  101,
		Branch:  "feature-101",
		HeadSHA: "1a8dd249c04a",
	}, *pullRequests[0])
	assert.Equal(t, PullRequest{
		Number:  102,
		Branch:  "feature-102",
		HeadSHA: "6344d9623e3b",
	}, *pullRequests[1])

	regexp = `.*2$`
	svc, err = NewBitbucketCloudServiceNoAuth(ts.URL, "OWNER", "REPO")
	assert.NoError(t, err)
	pullRequests, err = ListPullRequests(context.Background(), svc, []v1alpha1.PullRequestGeneratorFilter{
		{
			BranchMatch: &regexp,
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(pullRequests))
	assert.Equal(t, PullRequest{
		Number:  102,
		Branch:  "feature-102",
		HeadSHA: "6344d9623e3b",
	}, *pullRequests[0])

	regexp = `[\d{2}`
	svc, err = NewBitbucketCloudServiceNoAuth(ts.URL, "OWNER", "REPO")
	assert.NoError(t, err)
	_, err = ListPullRequests(context.Background(), svc, []v1alpha1.PullRequestGeneratorFilter{
		{
			BranchMatch: &regexp,
		},
	})
	assert.Error(t, err)
}
