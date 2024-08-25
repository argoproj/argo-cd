package pull_request

import (
	"io"
	"net/http"
	"testing"

	"github.com/google/go-github/v63/github"
	"github.com/stretchr/testify/assert"
)

func githubMockHandler(t *testing.T) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.RequestURI {
		case "/api/v3/orgs/argoproj/repos?per_page=100":
			_, err := io.WriteString(w, `[
				{
				  "id": 1296269,
				  "node_id": "MDEwOlJlcG9zaXRvcnkxMjk2MjY5",
				  "name": "argo-cd",
				  "full_name": "argoproj/argo-cd",
				  "owner": {
					"login": "argoproj",
					"id": 1,
					"node_id": "MDQ6VXNlcjE=",
					"avatar_url": "https://github.com/images/error/argoproj_happy.gif",
					"gravatar_id": "",
					"url": "https://api.github.com/users/argoproj",
					"html_url": "https://github.com/argoproj",
					"followers_url": "https://api.github.com/users/argoproj/followers",
					"following_url": "https://api.github.com/users/argoproj/following{/other_user}",
					"gists_url": "https://api.github.com/users/argoproj/gists{/gist_id}",
					"starred_url": "https://api.github.com/users/argoproj/starred{/owner}{/repo}",
					"subscriptions_url": "https://api.github.com/users/argoproj/subscriptions",
					"organizations_url": "https://api.github.com/users/argoproj/orgs",
					"repos_url": "https://api.github.com/users/argoproj/repos",
					"events_url": "https://api.github.com/users/argoproj/events{/privacy}",
					"received_events_url": "https://api.github.com/users/argoproj/received_events",
					"type": "User",
					"site_admin": false
				  },
				  "private": false,
				  "html_url": "https://github.com/argoproj/argo-cd",
				  "description": "This your first repo!",
				  "fork": false,
				  "url": "https://api.github.com/repos/argoproj/argo-cd",
				  "archive_url": "https://api.github.com/repos/argoproj/argo-cd/{archive_format}{/ref}",
				  "assignees_url": "https://api.github.com/repos/argoproj/argo-cd/assignees{/user}",
				  "blobs_url": "https://api.github.com/repos/argoproj/argo-cd/git/blobs{/sha}",
				  "branches_url": "https://api.github.com/repos/argoproj/argo-cd/branches{/branch}",
				  "collaborators_url": "https://api.github.com/repos/argoproj/argo-cd/collaborators{/collaborator}",
				  "comments_url": "https://api.github.com/repos/argoproj/argo-cd/comments{/number}",
				  "commits_url": "https://api.github.com/repos/argoproj/argo-cd/commits{/sha}",
				  "compare_url": "https://api.github.com/repos/argoproj/argo-cd/compare/{base}...{head}",
				  "contents_url": "https://api.github.com/repos/argoproj/argo-cd/contents/{path}",
				  "contributors_url": "https://api.github.com/repos/argoproj/argo-cd/contributors",
				  "deployments_url": "https://api.github.com/repos/argoproj/argo-cd/deployments",
				  "downloads_url": "https://api.github.com/repos/argoproj/argo-cd/downloads",
				  "events_url": "https://api.github.com/repos/argoproj/argo-cd/events",
				  "forks_url": "https://api.github.com/repos/argoproj/argo-cd/forks",
				  "git_commits_url": "https://api.github.com/repos/argoproj/argo-cd/git/commits{/sha}",
				  "git_refs_url": "https://api.github.com/repos/argoproj/argo-cd/git/refs{/sha}",
				  "git_tags_url": "https://api.github.com/repos/argoproj/argo-cd/git/tags{/sha}",
				  "git_url": "git:github.com/argoproj/argo-cd.git",
				  "issue_comment_url": "https://api.github.com/repos/argoproj/argo-cd/issues/comments{/number}",
				  "issue_events_url": "https://api.github.com/repos/argoproj/argo-cd/issues/events{/number}",
				  "issues_url": "https://api.github.com/repos/argoproj/argo-cd/issues{/number}",
				  "keys_url": "https://api.github.com/repos/argoproj/argo-cd/keys{/key_id}",
				  "labels_url": "https://api.github.com/repos/argoproj/argo-cd/labels{/name}",
				  "languages_url": "https://api.github.com/repos/argoproj/argo-cd/languages",
				  "merges_url": "https://api.github.com/repos/argoproj/argo-cd/merges",
				  "milestones_url": "https://api.github.com/repos/argoproj/argo-cd/milestones{/number}",
				  "notifications_url": "https://api.github.com/repos/argoproj/argo-cd/notifications{?since,all,participating}",
				  "pulls_url": "https://api.github.com/repos/argoproj/argo-cd/pulls{/number}",
				  "releases_url": "https://api.github.com/repos/argoproj/argo-cd/releases{/id}",
				  "ssh_url": "git@github.com:argoproj/argo-cd.git",
				  "stargazers_url": "https://api.github.com/repos/argoproj/argo-cd/stargazers",
				  "statuses_url": "https://api.github.com/repos/argoproj/argo-cd/statuses/{sha}",
				  "subscribers_url": "https://api.github.com/repos/argoproj/argo-cd/subscribers",
				  "subscription_url": "https://api.github.com/repos/argoproj/argo-cd/subscription",
				  "tags_url": "https://api.github.com/repos/argoproj/argo-cd/tags",
				  "teams_url": "https://api.github.com/repos/argoproj/argo-cd/teams",
				  "trees_url": "https://api.github.com/repos/argoproj/argo-cd/git/trees{/sha}",
				  "clone_url": "https://github.com/argoproj/argo-cd.git",
				  "mirror_url": "git:git.example.com/argoproj/argo-cd",
				  "hooks_url": "https://api.github.com/repos/argoproj/argo-cd/hooks",
				  "svn_url": "https://svn.github.com/argoproj/argo-cd",
				  "homepage": "https://github.com",
				  "language": null,
				  "forks_count": 9,
				  "stargazers_count": 80,
				  "watchers_count": 80,
				  "size": 108,
				  "default_branch": "master",
				  "open_issues_count": 0,
				  "is_template": false,
				  "topics": [
					"argoproj",
					"atom",
					"electron",
					"api"
				  ],
				  "has_issues": true,
				  "has_projects": true,
				  "has_wiki": true,
				  "has_pages": false,
				  "has_downloads": true,
				  "archived": false,
				  "disabled": false,
				  "visibility": "public",
				  "pushed_at": "2011-01-26T19:06:43Z",
				  "created_at": "2011-01-26T19:01:12Z",
				  "updated_at": "2011-01-26T19:14:43Z",
				  "permissions": {
					"admin": false,
					"push": false,
					"pull": true
				  },
				  "template_repository": null
				}
			  ]`)
			if err != nil {
				t.Fail()
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func toPtr(s string) *string {
	return &s
}

func TestContainLabels(t *testing.T) {
	cases := []struct {
		Name       string
		Labels     []string
		PullLabels []*github.Label
		Expect     bool
	}{
		{
			Name:   "Match labels",
			Labels: []string{"label1", "label2"},
			PullLabels: []*github.Label{
				{Name: toPtr("label1")},
				{Name: toPtr("label2")},
				{Name: toPtr("label3")},
			},
			Expect: true,
		},
		{
			Name:   "Not match labels",
			Labels: []string{"label1", "label4"},
			PullLabels: []*github.Label{
				{Name: toPtr("label1")},
				{Name: toPtr("label2")},
				{Name: toPtr("label3")},
			},
			Expect: false,
		},
		{
			Name:   "No specify",
			Labels: []string{},
			PullLabels: []*github.Label{
				{Name: toPtr("label1")},
				{Name: toPtr("label2")},
				{Name: toPtr("label3")},
			},
			Expect: true,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			if got := containLabels(c.Labels, c.PullLabels); got != c.Expect {
				t.Errorf("expect: %v, got: %v", c.Expect, got)
			}
		})
	}
}

func TestGetGitHubPRLabelNames(t *testing.T) {
	Tests := []struct {
		Name           string
		PullLabels     []*github.Label
		ExpectedResult []string
	}{
		{
			Name: "PR has labels",
			PullLabels: []*github.Label{
				{Name: toPtr("label1")},
				{Name: toPtr("label2")},
				{Name: toPtr("label3")},
			},
			ExpectedResult: []string{"label1", "label2", "label3"},
		},
		{
			Name:           "PR does not have labels",
			PullLabels:     []*github.Label{},
			ExpectedResult: nil,
		},
	}
	for _, test := range Tests {
		t.Run(test.Name, func(t *testing.T) {
			labels := getGithubPRLabelNames(test.PullLabels)
			assert.Equal(t, test.ExpectedResult, labels)
		})
	}
}
