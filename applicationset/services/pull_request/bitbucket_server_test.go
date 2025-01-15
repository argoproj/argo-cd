package pull_request

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func defaultHandler(t *testing.T) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var err error
		switch r.RequestURI {
		case "/rest/api/1.0/projects/PROJECT/repos/REPO/pull-requests?limit=100":
			_, err = io.WriteString(w, `{
					"size": 1,
					"limit": 100,
					"isLastPage": true,
					"values": [
						{
							"id": 101,
							"title": "feat(ABC) : 123",
							"toRef": {
								"latestCommit": "5b766e3564a3453808f3cd3dd3f2e5fad8ef0e7a",
								"displayId": "master",
								"id": "refs/heads/master"
							},
							"fromRef": {
								"id": "refs/heads/feature-ABC-123",
								"displayId": "feature-ABC-123",
								"latestCommit": "cb3cf2e4d1517c83e720d2585b9402dbef71f992"
							},
							"author": {
								"user": {
									"name": "testName"
								}
							}
						}
					],
					"start": 0
				}`)
		default:
			t.Fail()
		}
		if err != nil {
			t.Fail()
		}
	}
}

func TestListPullRequestNoAuth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))
		defaultHandler(t)(w, r)
	}))
	defer ts.Close()
	svc, err := NewBitbucketServiceNoAuth(context.Background(), ts.URL, "PROJECT", "REPO", "", false, nil)
	require.NoError(t, err)
	pullRequests, err := ListPullRequests(context.Background(), svc, []v1alpha1.PullRequestGeneratorFilter{})
	require.NoError(t, err)
	assert.Len(t, pullRequests, 1)
	assert.Equal(t, 101, pullRequests[0].Number)
	assert.Equal(t, "feat(ABC) : 123", pullRequests[0].Title)
	assert.Equal(t, "feature-ABC-123", pullRequests[0].Branch)
	assert.Equal(t, "master", pullRequests[0].TargetBranch)
	assert.Equal(t, "cb3cf2e4d1517c83e720d2585b9402dbef71f992", pullRequests[0].HeadSHA)
	assert.Equal(t, "testName", pullRequests[0].Author)
}

func TestListPullRequestPagination(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var err error
		switch r.RequestURI {
		case "/rest/api/1.0/projects/PROJECT/repos/REPO/pull-requests?limit=100":
			_, err = io.WriteString(w, `{
					"size": 2,
					"limit": 2,
					"isLastPage": false,
					"values": [
						{
							"id": 101,
							"title": "feat(101)",
							"toRef": {
								"latestCommit": "5b766e3564a3453808f3cd3dd3f2e5fad8ef0e7a",
								"displayId": "master",
								"id": "refs/heads/master"
							},
							"fromRef": {
								"id": "refs/heads/feature-101",
								"displayId": "feature-101",
								"latestCommit": "ab3cf2e4d1517c83e720d2585b9402dbef71f992"
							},
							"author": {
								"user": {
									"name": "testName"
								}
							}
						},
						{
							"id": 102,
							"title": "feat(102)",
							"toRef": {
								"latestCommit": "5b766e3564a3453808f3cd3dd3f2e5fad8ef0e7a",
								"displayId": "branch",
								"id": "refs/heads/branch"
							},
							"fromRef": {
								"id": "refs/heads/feature-102",
								"displayId": "feature-102",
								"latestCommit": "bb3cf2e4d1517c83e720d2585b9402dbef71f992"
							},
							"author": {
								"user": {
									"name": "testName"
								}
							}
						}
					],
					"nextPageStart": 200
				}`)
		case "/rest/api/1.0/projects/PROJECT/repos/REPO/pull-requests?limit=100&start=200":
			_, err = io.WriteString(w, `{
				"size": 1,
				"limit": 2,
				"isLastPage": true,
				"values": [
					{
						"id": 200,
						"title": "feat(200)",
						"toRef": {
							"latestCommit": "5b766e3564a3453808f3cd3dd3f2e5fad8ef0e7a",
							"displayId": "master",
							"id": "refs/heads/master"
						},
						"fromRef": {
							"id": "refs/heads/feature-200",
							"displayId": "feature-200",
							"latestCommit": "cb3cf2e4d1517c83e720d2585b9402dbef71f992"
						},
						"author": {
							"user": {
								"name": "testName"
							}
						}
					}
				],
				"start": 200
			}`)
		default:
			t.Fail()
		}
		if err != nil {
			t.Fail()
		}
	}))
	defer ts.Close()
	svc, err := NewBitbucketServiceNoAuth(context.Background(), ts.URL, "PROJECT", "REPO", "", false, nil)
	require.NoError(t, err)
	pullRequests, err := ListPullRequests(context.Background(), svc, []v1alpha1.PullRequestGeneratorFilter{})
	require.NoError(t, err)
	assert.Len(t, pullRequests, 3)
	assert.Equal(t, PullRequest{
		Number:       101,
		Title:        "feat(101)",
		Branch:       "feature-101",
		TargetBranch: "master",
		HeadSHA:      "ab3cf2e4d1517c83e720d2585b9402dbef71f992",
		Labels:       []string{},
		Author:       "testName",
	}, *pullRequests[0])
	assert.Equal(t, PullRequest{
		Number:       102,
		Title:        "feat(102)",
		Branch:       "feature-102",
		TargetBranch: "branch",
		HeadSHA:      "bb3cf2e4d1517c83e720d2585b9402dbef71f992",
		Labels:       []string{},
		Author:       "testName",
	}, *pullRequests[1])
	assert.Equal(t, PullRequest{
		Number:       200,
		Title:        "feat(200)",
		Branch:       "feature-200",
		TargetBranch: "master",
		HeadSHA:      "cb3cf2e4d1517c83e720d2585b9402dbef71f992",
		Labels:       []string{},
		Author:       "testName",
	}, *pullRequests[2])
}

func TestListPullRequestBasicAuth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// base64(user:password)
		assert.Equal(t, "Basic dXNlcjpwYXNzd29yZA==", r.Header.Get("Authorization"))
		assert.Equal(t, "no-check", r.Header.Get("X-Atlassian-Token"))
		defaultHandler(t)(w, r)
	}))
	defer ts.Close()
	svc, err := NewBitbucketServiceBasicAuth(context.Background(), "user", "password", ts.URL, "PROJECT", "REPO", "", false, nil)
	require.NoError(t, err)
	pullRequests, err := ListPullRequests(context.Background(), svc, []v1alpha1.PullRequestGeneratorFilter{})
	require.NoError(t, err)
	assert.Len(t, pullRequests, 1)
	assert.Equal(t, 101, pullRequests[0].Number)
	assert.Equal(t, "feature-ABC-123", pullRequests[0].Branch)
	assert.Equal(t, "cb3cf2e4d1517c83e720d2585b9402dbef71f992", pullRequests[0].HeadSHA)
}

func TestListPullRequestBearerAuth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer tolkien", r.Header.Get("Authorization"))
		assert.Equal(t, "no-check", r.Header.Get("X-Atlassian-Token"))
		defaultHandler(t)(w, r)
	}))
	defer ts.Close()
	svc, err := NewBitbucketServiceBearerToken(context.Background(), "tolkien", ts.URL, "PROJECT", "REPO", "", false, nil)
	require.NoError(t, err)
	pullRequests, err := ListPullRequests(context.Background(), svc, []v1alpha1.PullRequestGeneratorFilter{})
	require.NoError(t, err)
	assert.Len(t, pullRequests, 1)
	assert.Equal(t, 101, pullRequests[0].Number)
	assert.Equal(t, "feat(ABC) : 123", pullRequests[0].Title)
	assert.Equal(t, "feature-ABC-123", pullRequests[0].Branch)
	assert.Equal(t, "cb3cf2e4d1517c83e720d2585b9402dbef71f992", pullRequests[0].HeadSHA)
}

func TestListPullRequestTLS(t *testing.T) {
	tests := []struct {
		name        string
		tlsInsecure bool
		passCerts   bool
		requireErr  bool
	}{
		{
			name:        "TLS Insecure: true, No Certs",
			tlsInsecure: true,
			passCerts:   false,
			requireErr:  false,
		},
		{
			name:        "TLS Insecure: true, With Certs",
			tlsInsecure: true,
			passCerts:   true,
			requireErr:  false,
		},
		{
			name:        "TLS Insecure: false, With Certs",
			tlsInsecure: false,
			passCerts:   true,
			requireErr:  false,
		},
		{
			name:        "TLS Insecure: false, No Certs",
			tlsInsecure: false,
			passCerts:   false,
			requireErr:  true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defaultHandler(t)(w, r)
			}))
			defer ts.Close()

			var certs []byte
			if test.passCerts == true {
				for _, cert := range ts.TLS.Certificates {
					for _, c := range cert.Certificate {
						parsedCert, err := x509.ParseCertificate(c)
						require.NoError(t, err, "Failed to parse certificate")
						certs = append(certs, pem.EncodeToMemory(&pem.Block{
							Type:  "CERTIFICATE",
							Bytes: parsedCert.Raw,
						})...)
					}
				}
			}

			svc, err := NewBitbucketServiceBasicAuth(context.Background(), "user", "password", ts.URL, "PROJECT", "REPO", "", test.tlsInsecure, certs)
			require.NoError(t, err)
			_, err = ListPullRequests(context.Background(), svc, []v1alpha1.PullRequestGeneratorFilter{})
			if test.requireErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestListResponseError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()
	svc, _ := NewBitbucketServiceNoAuth(context.Background(), ts.URL, "PROJECT", "REPO", "", false, nil)
	_, err := ListPullRequests(context.Background(), svc, []v1alpha1.PullRequestGeneratorFilter{})
	require.Error(t, err)
}

func TestListResponseMalformed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.RequestURI {
		case "/rest/api/1.0/projects/PROJECT/repos/REPO/pull-requests?limit=100":
			_, err := io.WriteString(w, `{
					"size": 1,
					"limit": 100,
					"isLastPage": true,
					"values": { "id": 101 },
					"start": 0
				}`)
			if err != nil {
				t.Fail()
			}
		default:
			t.Fail()
		}
	}))
	defer ts.Close()
	svc, _ := NewBitbucketServiceNoAuth(context.Background(), ts.URL, "PROJECT", "REPO", "", false, nil)
	_, err := ListPullRequests(context.Background(), svc, []v1alpha1.PullRequestGeneratorFilter{})
	require.Error(t, err)
}

func TestListResponseEmpty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.RequestURI {
		case "/rest/api/1.0/projects/PROJECT/repos/REPO/pull-requests?limit=100":
			_, err := io.WriteString(w, `{
					"size": 0,
					"limit": 100,
					"isLastPage": true,
					"values": [],
					"start": 0
				}`)
			if err != nil {
				t.Fail()
			}
		default:
			t.Fail()
		}
	}))
	defer ts.Close()
	svc, err := NewBitbucketServiceNoAuth(context.Background(), ts.URL, "PROJECT", "REPO", "", false, nil)
	require.NoError(t, err)
	pullRequests, err := ListPullRequests(context.Background(), svc, []v1alpha1.PullRequestGeneratorFilter{})
	require.NoError(t, err)
	assert.Empty(t, pullRequests)
}

func TestListPullRequestBranchMatch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var err error
		switch r.RequestURI {
		case "/rest/api/1.0/projects/PROJECT/repos/REPO/pull-requests?limit=100":
			_, err = io.WriteString(w, `{
					"size": 2,
					"limit": 2,
					"isLastPage": false,
					"values": [
						{
							"id": 101,
							"title": "feat(101)",
							"toRef": {
								"latestCommit": "5b766e3564a3453808f3cd3dd3f2e5fad8ef0e7a",
								"displayId": "master",
								"id": "refs/heads/master"
							},
							"fromRef": {
								"id": "refs/heads/feature-101",
								"displayId": "feature-101",
								"latestCommit": "ab3cf2e4d1517c83e720d2585b9402dbef71f992"
							},
							"author": {
								"user": {
									"name": "testName"
								}
							}
						},
						{
							"id": 102,
							"title": "feat(102)",
							"toRef": {
								"latestCommit": "5b766e3564a3453808f3cd3dd3f2e5fad8ef0e7a",
								"displayId": "branch",
								"id": "refs/heads/branch"
							},
							"fromRef": {
								"id": "refs/heads/feature-102",
								"displayId": "feature-102",
								"latestCommit": "bb3cf2e4d1517c83e720d2585b9402dbef71f992"
							},
							"author": {
								"user": {
									"name": "testName"
								}
							}
						}
					],
					"nextPageStart": 200
				}`)
		case "/rest/api/1.0/projects/PROJECT/repos/REPO/pull-requests?limit=100&start=200":
			_, err = io.WriteString(w, `{
				"size": 1,
				"limit": 2,
				"isLastPage": true,
				"values": [
					{
						"id": 200,
						"title": "feat(200)",
						"toRef": {
							"latestCommit": "5b766e3564a3453808f3cd3dd3f2e5fad8ef0e7a",
							"displayId": "master",
							"id": "refs/heads/master"
						},
						"fromRef": {
							"id": "refs/heads/feature-200",
							"displayId": "feature-200",
							"latestCommit": "cb3cf2e4d1517c83e720d2585b9402dbef71f992"
						},
						"author": {
							"user": {
								"name": "testName"
							}
						}
					}
				],
				"start": 200
			}`)
		default:
			t.Fail()
		}
		if err != nil {
			t.Fail()
		}
	}))
	defer ts.Close()
	regexp := `feature-1[\d]{2}`
	svc, err := NewBitbucketServiceNoAuth(context.Background(), ts.URL, "PROJECT", "REPO", "", false, nil)
	require.NoError(t, err)
	pullRequests, err := ListPullRequests(context.Background(), svc, []v1alpha1.PullRequestGeneratorFilter{
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
		TargetBranch: "master",
		HeadSHA:      "ab3cf2e4d1517c83e720d2585b9402dbef71f992",
		Labels:       []string{},
		Author:       "testName",
	}, *pullRequests[0])
	assert.Equal(t, PullRequest{
		Number:       102,
		Title:        "feat(102)",
		Branch:       "feature-102",
		TargetBranch: "branch",
		HeadSHA:      "bb3cf2e4d1517c83e720d2585b9402dbef71f992",
		Labels:       []string{},
		Author:       "testName",
	}, *pullRequests[1])

	regexp = `.*2$`
	svc, err = NewBitbucketServiceNoAuth(context.Background(), ts.URL, "PROJECT", "REPO", "", false, nil)
	require.NoError(t, err)
	pullRequests, err = ListPullRequests(context.Background(), svc, []v1alpha1.PullRequestGeneratorFilter{
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
		TargetBranch: "branch",
		HeadSHA:      "bb3cf2e4d1517c83e720d2585b9402dbef71f992",
		Labels:       []string{},
		Author:       "testName",
	}, *pullRequests[0])

	regexp = `[\d{2}`
	svc, err = NewBitbucketServiceNoAuth(context.Background(), ts.URL, "PROJECT", "REPO", "", false, nil)
	require.NoError(t, err)
	_, err = ListPullRequests(context.Background(), svc, []v1alpha1.PullRequestGeneratorFilter{
		{
			BranchMatch: &regexp,
		},
	})
	require.Error(t, err)
}
