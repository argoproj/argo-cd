package pull_request

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func scmmMockHandler(t *testing.T) func(http.ResponseWriter, *http.Request) {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.RequestURI {
		case "/api/v2/pull-requests/test-argocd/pr-test?pageSize=10&status=OPEN":
			_, err := io.WriteString(w, `{
    "page": 0,
    "pageTotal": 1,
    "_embedded": {
        "pullRequests": [
            {
                "id": "1",
                "author": {
                    "id": "eheimbuch",
                    "displayName": "Eduard Heimbuch",
                    "mail": "eduard.heimbuch@cloudogu.com"
                },
                "reviser": {
                    "id": null,
                    "displayName": null
                },
                "closeDate": null,
                "source": "test_pr",
                "target": "main",
                "title": "New feature xyz",
                "description": "Awesome!",
                "creationDate": "2023-01-23T12:58:56.770Z",
                "lastModified": null,
                "status": "OPEN",
                "reviewer": [],
                "tasks": {
                    "todo": 0,
                    "done": 0
                },
                "sourceRevision": null,
                "targetRevision": null,
                "markedAsReviewed": [],
                "emergencyMerged": false,
                "ignoredMergeObstacles": null
            }
        ]
    }
}`)
			if err != nil {
				t.Fail()
			}
		case "/api/v2/repositories/test-argocd/pr-test/branches/test_pr/changesets?pageSize=1":
			_, err := io.WriteString(w, `{
  "page": 0,
  "pageTotal": 1,
  "_embedded": {
    "changesets": [
      {
        "id": "b4ed814b1afe810c4902bc5590c7b09531296679",
        "author": {
          "mail": "eduard.heimbuch@cloudogu.com",
          "name": "Eduard Heimbuch"
        },
        "date": "2023-07-03T08:53:15Z",
        "description": "test url",
        "contributors": [
          {
            "type": "Pushed-by",
            "person": {
              "mail": "eduard.heimbuch@cloudogu.com",
              "name": "Eduard Heimbuch"
            }
          }
        ]
      }
    ]
  }
}`)
			if err != nil {
				t.Fail()
			}
		}
	}
}

func TestScmManagerPrList(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scmmMockHandler(t)(w, r)
	}))
	defer ts.Close()
	host, err := NewScmManagerService(context.Background(), "", ts.URL, "test-argocd", "pr-test", false, "", nil)
	require.NoError(t, err)
	prs, err := host.List(context.Background())
	require.NoError(t, err)
	require.Len(t, prs, 1)
	assert.Equal(t, int64(1), prs[0].Number)
	assert.Equal(t, "test_pr", prs[0].Branch)
	assert.Equal(t, "main", prs[0].TargetBranch)
	assert.Equal(t, "New feature xyz", prs[0].Title)
	assert.Equal(t, "eheimbuch", prs[0].Author)
	assert.Equal(t, "b4ed814b1afe810c4902bc5590c7b09531296679", prs[0].HeadSHA)
}

func TestScmManagerPullRequestPages(t *testing.T) {
	for _, failNextPage := range []bool{false, true} {
		name := "all pages"
		if failNextPage {
			name = "later page fails"
		}
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v2/pull-requests/team/work":
					switch r.URL.Query().Get("page") {
					case "":
						_, _ = io.WriteString(w, `{"page":0,"pageTotal":2,"_embedded":{"pullRequests":[{"id":"1","source":"first"}]}}`)
					case "1":
						if failNextPage {
							w.WriteHeader(http.StatusServiceUnavailable)
							return
						}
						_, _ = io.WriteString(w, `{"page":1,"pageTotal":2,"_embedded":{"pullRequests":[{"id":"2","source":"second"}]}}`)
					default:
						t.Errorf("unexpected page in %s", r.RequestURI)
						w.WriteHeader(http.StatusBadRequest)
					}
				case "/api/v2/repositories/team/work/branches/first/changesets",
					"/api/v2/repositories/team/work/branches/second/changesets":
					_, _ = io.WriteString(w, `{"_embedded":{"changesets":[{"id":"abc"}]}}`)
				default:
					t.Errorf("unexpected request %s", r.RequestURI)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			service, err := NewScmManagerService(t.Context(), "", server.URL, "team", "work", false, "", nil)
			require.NoError(t, err)
			prs, err := service.List(t.Context())
			if failNextPage {
				require.ErrorContains(t, err, "HTTP 503")
				require.Nil(t, prs)
				return
			}
			require.NoError(t, err)
			require.Len(t, prs, 2)
			assert.Equal(t, int64(2), prs[1].Number)
		})
	}
}
