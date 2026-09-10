package pull_request

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"code.gitea.io/sdk/gitea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/applicationset/services"
)

func giteaMockHandler(t *testing.T) func(http.ResponseWriter, *http.Request) {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Println(r.RequestURI)
		switch r.RequestURI {
		case "/api/v1/version":
			_, err := io.WriteString(w, `{"version":"1.17.0+dev-452-g1f0541780"}`)
			if err != nil {
				t.Fail()
			}
		case "/api/v1/repos/test-argocd/pr-test/pulls?limit=50&page=1&state=open":
			_, err := io.WriteString(w, `[{
				"id": 50721,
				"url": "https://gitea.com/test-argocd/pr-test/pulls/1",
				"number": 1,
				"user": {
					"id": 4476,
					"login": "graytshirt",
					"full_name": "Dan",
					"email": "graytshirt@noreply.gitea.io",
					"avatar_url": "https://secure.gravatar.com/avatar/2446c67bcd59d71f6ae3cf376ec2ae37?d=identicon",
					"language": "",
					"is_admin": false,
					"last_login": "0001-01-01T00:00:00Z",
					"created": "2020-04-07T01:14:36+08:00",
					"restricted": false,
					"active": false,
					"prohibit_login": false,
					"location": "",
					"website": "",
					"description": "",
					"visibility": "public",
					"followers_count": 0,
					"following_count": 4,
					"starred_repos_count": 1,
					"username": "graytshirt"
				},
				"title": "add an empty file",
				"body": "",
				"labels": [{"id": 1, "name": "label1", "color": "00aabb", "description": "foo", "url": ""}],
				"milestone": null,
				"assignee": null,
				"assignees": null,
				"state": "open",
				"is_locked": false,
				"comments": 0,
				"html_url": "https://gitea.com/test-argocd/pr-test/pulls/1",
				"diff_url": "https://gitea.com/test-argocd/pr-test/pulls/1.diff",
				"patch_url": "https://gitea.com/test-argocd/pr-test/pulls/1.patch",
				"mergeable": true,
				"merged": false,
				"merged_at": null,
				"merge_commit_sha": null,
				"merged_by": null,
				"base": {
					"label": "main",
					"ref": "main",
					"sha": "72687815ccba81ef014a96201cc2e846a68789d8",
					"repo_id": 21618,
					"repo": {
						"id": 21618,
						"owner": {
							"id": 31480,
							"login": "test-argocd",
							"full_name": "",
							"email": "",
							"avatar_url": "https://gitea.com/avatars/22d1b1d3f61abf95951c4a958731d848",
							"language": "",
							"is_admin": false,
							"last_login": "0001-01-01T00:00:00Z",
							"created": "2022-04-06T02:28:06+08:00",
							"restricted": false,
							"active": false,
							"prohibit_login": false,
							"location": "",
							"website": "",
							"description": "",
							"visibility": "public",
							"followers_count": 0,
							"following_count": 0,
							"starred_repos_count": 0,
							"username": "test-argocd"
						},
						"name": "pr-test",
						"full_name": "test-argocd/pr-test",
						"description": "",
						"empty": false,
						"private": false,
						"fork": false,
						"template": false,
						"parent": null,
						"mirror": false,
						"size": 28,
						"language": "",
						"languages_url": "https://gitea.com/api/v1/repos/test-argocd/pr-test/languages",
						"html_url": "https://gitea.com/test-argocd/pr-test",
						"ssh_url": "git@gitea.com:test-argocd/pr-test.git",
						"clone_url": "https://gitea.com/test-argocd/pr-test.git",
						"original_url": "",
						"website": "",
						"stars_count": 0,
						"forks_count": 0,
						"watchers_count": 1,
						"open_issues_count": 0,
						"open_pr_counter": 1,
						"release_counter": 0,
						"default_branch": "main",
						"archived": false,
						"created_at": "2022-04-06T02:32:09+08:00",
						"updated_at": "2022-04-06T02:33:12+08:00",
						"permissions": {
							"admin": false,
							"push": false,
							"pull": true
						},
						"has_issues": true,
						"internal_tracker": {
							"enable_time_tracker": true,
							"allow_only_contributors_to_track_time": true,
							"enable_issue_dependencies": true
						},
						"has_wiki": true,
						"has_pull_requests": true,
						"has_projects": true,
						"ignore_whitespace_conflicts": false,
						"allow_merge_commits": true,
						"allow_rebase": true,
						"allow_rebase_explicit": true,
						"allow_squash_merge": true,
						"default_merge_style": "merge",
						"avatar_url": "",
						"internal": false,
						"mirror_interval": "",
						"mirror_updated": "0001-01-01T00:00:00Z",
						"repo_transfer": null
					}
				},
				"head": {
					"label": "test",
					"ref": "test",
					"sha": "7bbaf62d92ddfafd9cc8b340c619abaec32bc09f",
					"repo_id": 21618,
					"repo": {
						"id": 21618,
						"owner": {
							"id": 31480,
							"login": "test-argocd",
							"full_name": "",
							"email": "",
							"avatar_url": "https://gitea.com/avatars/22d1b1d3f61abf95951c4a958731d848",
							"language": "",
							"is_admin": false,
							"last_login": "0001-01-01T00:00:00Z",
							"created": "2022-04-06T02:28:06+08:00",
							"restricted": false,
							"active": false,
							"prohibit_login": false,
							"location": "",
							"website": "",
							"description": "",
							"visibility": "public",
							"followers_count": 0,
							"following_count": 0,
							"starred_repos_count": 0,
							"username": "test-argocd"
						},
						"name": "pr-test",
						"full_name": "test-argocd/pr-test",
						"description": "",
						"empty": false,
						"private": false,
						"fork": false,
						"template": false,
						"parent": null,
						"mirror": false,
						"size": 28,
						"language": "",
						"languages_url": "https://gitea.com/api/v1/repos/test-argocd/pr-test/languages",
						"html_url": "https://gitea.com/test-argocd/pr-test",
						"ssh_url": "git@gitea.com:test-argocd/pr-test.git",
						"clone_url": "https://gitea.com/test-argocd/pr-test.git",
						"original_url": "",
						"website": "",
						"stars_count": 0,
						"forks_count": 0,
						"watchers_count": 1,
						"open_issues_count": 0,
						"open_pr_counter": 1,
						"release_counter": 0,
						"default_branch": "main",
						"archived": false,
						"created_at": "2022-04-06T02:32:09+08:00",
						"updated_at": "2022-04-06T02:33:12+08:00",
						"permissions": {
							"admin": false,
							"push": false,
							"pull": true
						},
						"has_issues": true,
						"internal_tracker": {
							"enable_time_tracker": true,
							"allow_only_contributors_to_track_time": true,
							"enable_issue_dependencies": true
						},
						"has_wiki": true,
						"has_pull_requests": true,
						"has_projects": true,
						"ignore_whitespace_conflicts": false,
						"allow_merge_commits": true,
						"allow_rebase": true,
						"allow_rebase_explicit": true,
						"allow_squash_merge": true,
						"default_merge_style": "merge",
						"avatar_url": "",
						"internal": false,
						"mirror_interval": "",
						"mirror_updated": "0001-01-01T00:00:00Z",
						"repo_transfer": null
					}
				},
				"merge_base": "72687815ccba81ef014a96201cc2e846a68789d8",
				"due_date": null,
				"created_at": "2022-04-06T02:34:24+08:00",
				"updated_at": "2022-04-06T02:34:24+08:00",
				"closed_at": null
			}]`)
			if err != nil {
				t.Fail()
			}
		default:
			// Any page past the first one is empty. Anything else is a request
			// the test did not expect.
			if r.URL.Query().Get("page") == "1" || r.URL.Query().Get("page") == "" {
				t.Errorf("unexpected request %q", r.RequestURI)
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, err := io.WriteString(w, `[]`)
			if err != nil {
				t.Fail()
			}
		}
	}
}

func TestGiteaContainLabels(t *testing.T) {
	t.Parallel()
	cases := []struct {
		Name       string
		Labels     []string
		PullLabels []*gitea.Label
		Expect     bool
	}{
		{
			Name:   "Match labels",
			Labels: []string{"label1", "label2"},
			PullLabels: []*gitea.Label{
				{Name: "label1"},
				{Name: "label2"},
				{Name: "label3"},
			},
			Expect: true,
		},
		{
			Name:   "Not match labels",
			Labels: []string{"label1", "label4"},
			PullLabels: []*gitea.Label{
				{Name: "label1"},
				{Name: "label2"},
				{Name: "label3"},
			},
			Expect: false,
		},
		{
			Name:   "No specify",
			Labels: []string{},
			PullLabels: []*gitea.Label{
				{Name: "label1"},
				{Name: "label2"},
				{Name: "label3"},
			},
			Expect: true,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			if got := giteaContainLabels(c.Labels, c.PullLabels); got != c.Expect {
				t.Errorf("expect: %v, got: %v", c.Expect, got)
			}
		})
	}
}

func TestGiteaList(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		giteaMockHandler(t)(w, r)
	}))
	host, err := NewGiteaService("", ts.URL, "test-argocd", "pr-test", []string{"label1"}, false, "", "")
	require.NoError(t, err)
	prs, err := host.List(t.Context())
	require.NoError(t, err)
	assert.Len(t, prs, 1)
	assert.Equal(t, int64(1), prs[0].Number)
	assert.Equal(t, "add an empty file", prs[0].Title)
	assert.Equal(t, "test", prs[0].Branch)
	assert.Equal(t, "main", prs[0].TargetBranch)
	assert.Equal(t, "7bbaf62d92ddfafd9cc8b340c619abaec32bc09f", prs[0].HeadSHA)
	assert.Equal(t, "graytshirt", prs[0].Author)
}

func TestGetGiteaPRLabelNames(t *testing.T) {
	t.Parallel()
	Tests := []struct {
		Name           string
		PullLabels     []*gitea.Label
		ExpectedResult []string
	}{
		{
			Name: "PR has labels",
			PullLabels: []*gitea.Label{
				{Name: "label1"},
				{Name: "label2"},
				{Name: "label3"},
			},
			ExpectedResult: []string{"label1", "label2", "label3"},
		},
		{
			Name:           "PR does not have labels",
			PullLabels:     []*gitea.Label{},
			ExpectedResult: nil,
		},
	}
	for _, test := range Tests {
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()
			labels := getGiteaPRLabelNames(test.PullLabels)
			assert.Equal(t, test.ExpectedResult, labels)
		})
	}
}

func TestGiteaListReturnsRepositoryNotFoundError(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	// Handle version endpoint that Gitea client calls first
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"1.17.0+dev-452-g1f0541780"}`))
	})

	path := "/api/v1/repos/nonexistent/nonexistent/pulls?limit=50&page=1&state=open"

	mux.HandleFunc(path, func(w http.ResponseWriter, _ *http.Request) {
		// Return 404 status to simulate repository not found
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message": "404 Project Not Found"}`))
	})

	svc, err := NewGiteaService("", server.URL, "nonexistent", "nonexistent", []string{}, false, "", "")
	require.NoError(t, err)

	prs, err := svc.List(t.Context())

	// Should return empty pull requests list
	assert.Empty(t, prs)

	// Should return RepositoryNotFoundError
	require.Error(t, err)
	assert.True(t, IsRepositoryNotFoundError(err), "Expected RepositoryNotFoundError but got: %v", err)
}

// giteaPaginatedPullsHandler serves open pull requests from a fake Gitea API. It
// mimics a server whose MAX_RESPONSE_ITEMS is smaller than the requested page
// size, so a short page is not necessarily the last page.
func giteaPaginatedPullsHandler(t *testing.T, total, serverMaxPageSize int, sendTotalCount bool, requests *atomic.Int32) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"version":"1.17.0+dev-452-g1f0541780"}`)
	})
	mux.HandleFunc("/api/v1/repos/test-argocd/pr-test/pulls", func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		page, limit, ok := giteaPageBounds(t, w, r, serverMaxPageSize)
		if !ok {
			return
		}

		start := min((page-1)*limit, total)
		end := min(start+limit, total)

		prs := make([]string, 0, end-start)
		for i := start; i < end; i++ {
			number := i + 1
			prs = append(prs, fmt.Sprintf(`{
				"number": %d,
				"title": "pr-%d",
				"user": {"username": "graytshirt"},
				"labels": [{"id": 1, "name": "label1"}],
				"state": "open",
				"base": {"ref": "main", "sha": "72687815ccba81ef014a96201cc2e846a68789d8"},
				"head": {"ref": "branch-%d", "sha": "7bbaf62d92ddfafd9cc8b340c619abaec32bc09f"}
			}`, number, number, number))
		}

		w.Header().Set("Content-Type", "application/json")
		if sendTotalCount {
			w.Header().Set("X-Total-Count", strconv.Itoa(total))
		}
		_, _ = io.WriteString(w, "["+strings.Join(prs, ",")+"]")
	})
	return mux
}

// giteaPageBounds reads the page and limit query parameters, clamping the limit
// the way a Gitea server clamps it to MAX_RESPONSE_ITEMS. It reports false when
// the request is malformed, in which case the response has already been written.
func giteaPageBounds(t *testing.T, w http.ResponseWriter, r *http.Request, serverMaxPageSize int) (int, int, bool) {
	t.Helper()
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil {
		t.Errorf("bad page parameter in %q: %v", r.RequestURI, err)
		http.Error(w, "bad page", http.StatusBadRequest)
		return 0, 0, false
	}
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil {
		t.Errorf("bad limit parameter in %q: %v", r.RequestURI, err)
		http.Error(w, "bad limit", http.StatusBadRequest)
		return 0, 0, false
	}
	if limit <= 0 || limit > serverMaxPageSize {
		limit = serverMaxPageSize
	}
	return page, limit, true
}

func TestGiteaListPaginates(t *testing.T) {
	t.Parallel()
	// The stub caps every response at 20 items while the client asks for
	// services.GiteaPageSize, so a short page is not the last page.
	const (
		total             = 45
		serverMaxPageSize = 20
	)

	for _, tc := range []struct {
		name             string
		sendTotalCount   bool
		expectedRequests int32
	}{
		// 3 pages of 20, 20, 5, and the total count says to stop there.
		{name: "server sends X-Total-Count", sendTotalCount: true, expectedRequests: 3},
		// Without the header the client needs one more request to see an empty page.
		{name: "server omits X-Total-Count", sendTotalCount: false, expectedRequests: 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var requests atomic.Int32
			ts := httptest.NewServer(giteaPaginatedPullsHandler(t, total, serverMaxPageSize, tc.sendTotalCount, &requests))
			defer ts.Close()

			host, err := NewGiteaService("", ts.URL, "test-argocd", "pr-test", nil, false, "", "")
			require.NoError(t, err)

			prs, err := host.List(t.Context())
			require.NoError(t, err)
			require.Len(t, prs, total)
			assert.Equal(t, int64(1), prs[0].Number)
			assert.Equal(t, "branch-45", prs[total-1].Branch)
			assert.Equal(t, tc.expectedRequests, requests.Load())
		})
	}
}

// giteaEndlessPullsHandler serves a full page of pull requests for every
// request and never sends X-Total-Count, so the client has no signal that the
// list ended. When repeatPage is true it serves the same page every time, the
// way a server that ignores the page parameter would.
func giteaEndlessPullsHandler(t *testing.T, repeatPage bool) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"version":"1.17.0+dev-452-g1f0541780"}`)
	})
	mux.HandleFunc("/api/v1/repos/test-argocd/pr-test/pulls", func(w http.ResponseWriter, r *http.Request) {
		page, limit, ok := giteaPageBounds(t, w, r, services.GiteaPageSize)
		if !ok {
			return
		}
		if repeatPage {
			page = 1
		}

		prs := make([]string, 0, limit)
		for i := range limit {
			number := (page-1)*limit + i + 1
			prs = append(prs, fmt.Sprintf(`{
				"number": %d,
				"title": "pr-%d",
				"user": {"username": "graytshirt"},
				"state": "open",
				"base": {"ref": "main", "sha": "72687815ccba81ef014a96201cc2e846a68789d8"},
				"head": {"ref": "branch-%d", "sha": "7bbaf62d92ddfafd9cc8b340c619abaec32bc09f"}
			}`, number, number, number))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "["+strings.Join(prs, ",")+"]")
	})
	return mux
}

func TestGiteaListRejectsRepeatedPage(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(giteaEndlessPullsHandler(t, true))
	defer ts.Close()

	host, err := NewGiteaService("", ts.URL, "test-argocd", "pr-test", nil, false, "", "")
	require.NoError(t, err)

	_, err = host.List(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not honouring the page parameter")
}

func TestGiteaListStopsAtMaxPages(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(giteaEndlessPullsHandler(t, false))
	defer ts.Close()

	host, err := NewGiteaService("", ts.URL, "test-argocd", "pr-test", nil, false, "", "")
	require.NoError(t, err)

	_, err = host.List(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("more than %d pages", services.GiteaMaxPages))
}
