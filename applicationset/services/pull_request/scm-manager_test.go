package pull_request

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func scmmMockHandler(t *testing.T) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Println(r.RequestURI)
		switch r.RequestURI {
		case "/api/v2/pull-requests/test-argocd/pr-test?status=OPEN&pageSize=10":
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
		case "/api/v2/repositories/test-argocd/pr-test/branches/test_pr/changesets?&pageSize=1":
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
	host, err := NewScmManagerService(context.Background(), "", ts.URL, "test-argocd", "pr-test", false)
	assert.Nil(t, err)
	prs, err := host.List(context.Background())
	assert.Nil(t, err)
	assert.Equal(t, len(prs), 1)
	assert.Equal(t, prs[0].Number, 1)
	assert.Equal(t, prs[0].Branch, "test_pr")
	assert.Equal(t, prs[0].HeadSHA, "b4ed814b1afe810c4902bc5590c7b09531296679")
}
