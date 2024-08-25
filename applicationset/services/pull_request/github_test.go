package pull_request

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/google/go-github/v63/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func githubMockHandler(t *testing.T) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.RequestURI {
		case "/api/v3/repos/argoproj/argo-cd/pulls?per_page=100":
			_, err := io.WriteString(w, `[
				{
				  "url": "https://api.github.com/repos/repos/argoproj/argo-cd/pulls/1347",
				  "id": 1,
				  "node_id": "MDExOlB1bGxSZXF1ZXN0MQ==",
				  "html_url": "https://github.com/repos/argoproj/argo-cd/pull/1347",
				  "diff_url": "https://github.com/repos/argoproj/argo-cd/pull/1347.diff",
				  "patch_url": "https://github.com/repos/argoproj/argo-cd/pull/1347.patch",
				  "issue_url": "https://api.github.com/repos/repos/argoproj/argo-cd/issues/1347",
				  "commits_url": "https://api.github.com/repos/repos/argoproj/argo-cd/pulls/1347/commits",
				  "review_comments_url": "https://api.github.com/repos/repos/argoproj/argo-cd/pulls/1347/comments",
				  "review_comment_url": "https://api.github.com/repos/repos/argoproj/argo-cd/pulls/comments{/number}",
				  "comments_url": "https://api.github.com/repos/repos/argoproj/argo-cd/issues/1347/comments",
				  "statuses_url": "https://api.github.com/repos/repos/argoproj/argo-cd/statuses/6dcb09b5b57875f334f61aebed695e2e4193db5e",
				  "number": 1347,
				  "state": "open",
				  "locked": true,
				  "title": "Amazing new feature",
				  "user": {
					"login": "argo-cd",
					"id": 1,
					"node_id": "MDQ6VXNlcjE=",
					"avatar_url": "https://github.com/images/error/argo-cd_happy.gif",
					"gravatar_id": "",
					"url": "https://api.github.com/users/argo-cd",
					"html_url": "https://github.com/argo-cd",
					"followers_url": "https://api.github.com/users/argo-cd/followers",
					"following_url": "https://api.github.com/users/argo-cd/following{/other_user}",
					"gists_url": "https://api.github.com/users/argo-cd/gists{/gist_id}",
					"starred_url": "https://api.github.com/users/argo-cd/starred{/owner}{/repo}",
					"subscriptions_url": "https://api.github.com/users/argo-cd/subscriptions",
					"organizations_url": "https://api.github.com/users/argo-cd/orgs",
					"repos_url": "https://api.github.com/users/argo-cd/repos",
					"events_url": "https://api.github.com/users/argo-cd/events{/privacy}",
					"received_events_url": "https://api.github.com/users/argo-cd/received_events",
					"type": "User",
					"site_admin": false
				  },
				  "body": "Please pull these awesome changes in!",
				  "labels": [
					{
					  "id": 208045946,
					  "node_id": "MDU6TGFiZWwyMDgwNDU5NDY=",
					  "url": "https://api.github.com/repos/repos/argoproj/argo-cd/labels/bug",
					  "name": "bug",
					  "description": "Something isn't working",
					  "color": "f29513",
					  "default": true
					}
				  ],
				  "milestone": {
					"url": "https://api.github.com/repos/repos/argoproj/argo-cd/milestones/1",
					"html_url": "https://github.com/repos/argoproj/argo-cd/milestones/v1.0",
					"labels_url": "https://api.github.com/repos/repos/argoproj/argo-cd/milestones/1/labels",
					"id": 1002604,
					"node_id": "MDk6TWlsZXN0b25lMTAwMjYwNA==",
					"number": 1,
					"state": "open",
					"title": "v1.0",
					"description": "Tracking milestone for version 1.0",
					"creator": {
					  "login": "argo-cd",
					  "id": 1,
					  "node_id": "MDQ6VXNlcjE=",
					  "avatar_url": "https://github.com/images/error/argo-cd_happy.gif",
					  "gravatar_id": "",
					  "url": "https://api.github.com/users/argo-cd",
					  "html_url": "https://github.com/argo-cd",
					  "followers_url": "https://api.github.com/users/argo-cd/followers",
					  "following_url": "https://api.github.com/users/argo-cd/following{/other_user}",
					  "gists_url": "https://api.github.com/users/argo-cd/gists{/gist_id}",
					  "starred_url": "https://api.github.com/users/argo-cd/starred{/owner}{/repo}",
					  "subscriptions_url": "https://api.github.com/users/argo-cd/subscriptions",
					  "organizations_url": "https://api.github.com/users/argo-cd/orgs",
					  "repos_url": "https://api.github.com/users/argo-cd/repos",
					  "events_url": "https://api.github.com/users/argo-cd/events{/privacy}",
					  "received_events_url": "https://api.github.com/users/argo-cd/received_events",
					  "type": "User",
					  "site_admin": false
					},
					"open_issues": 4,
					"closed_issues": 8,
					"created_at": "2011-04-10T20:09:31Z",
					"updated_at": "2014-03-03T18:58:10Z",
					"closed_at": "2013-02-12T13:22:01Z",
					"due_on": "2012-10-09T23:39:01Z"
				  },
				  "active_lock_reason": "too heated",
				  "created_at": "2011-01-26T19:01:12Z",
				  "updated_at": "2011-01-26T19:01:12Z",
				  "closed_at": "2011-01-26T19:01:12Z",
				  "merged_at": "2011-01-26T19:01:12Z",
				  "merge_commit_sha": "e5bd3914e2e596debea16f433f57875b5b90bcd6",
				  "assignee": {
					"login": "argo-cd",
					"id": 1,
					"node_id": "MDQ6VXNlcjE=",
					"avatar_url": "https://github.com/images/error/argo-cd_happy.gif",
					"gravatar_id": "",
					"url": "https://api.github.com/users/argo-cd",
					"html_url": "https://github.com/argo-cd",
					"followers_url": "https://api.github.com/users/argo-cd/followers",
					"following_url": "https://api.github.com/users/argo-cd/following{/other_user}",
					"gists_url": "https://api.github.com/users/argo-cd/gists{/gist_id}",
					"starred_url": "https://api.github.com/users/argo-cd/starred{/owner}{/repo}",
					"subscriptions_url": "https://api.github.com/users/argo-cd/subscriptions",
					"organizations_url": "https://api.github.com/users/argo-cd/orgs",
					"repos_url": "https://api.github.com/users/argo-cd/repos",
					"events_url": "https://api.github.com/users/argo-cd/events{/privacy}",
					"received_events_url": "https://api.github.com/users/argo-cd/received_events",
					"type": "User",
					"site_admin": false
				  },
				  "assignees": [
					{
					  "login": "argo-cd",
					  "id": 1,
					  "node_id": "MDQ6VXNlcjE=",
					  "avatar_url": "https://github.com/images/error/argo-cd_happy.gif",
					  "gravatar_id": "",
					  "url": "https://api.github.com/users/argo-cd",
					  "html_url": "https://github.com/argo-cd",
					  "followers_url": "https://api.github.com/users/argo-cd/followers",
					  "following_url": "https://api.github.com/users/argo-cd/following{/other_user}",
					  "gists_url": "https://api.github.com/users/argo-cd/gists{/gist_id}",
					  "starred_url": "https://api.github.com/users/argo-cd/starred{/owner}{/repo}",
					  "subscriptions_url": "https://api.github.com/users/argo-cd/subscriptions",
					  "organizations_url": "https://api.github.com/users/argo-cd/orgs",
					  "repos_url": "https://api.github.com/users/argo-cd/repos",
					  "events_url": "https://api.github.com/users/argo-cd/events{/privacy}",
					  "received_events_url": "https://api.github.com/users/argo-cd/received_events",
					  "type": "User",
					  "site_admin": false
					},
					{
					  "login": "hubot",
					  "id": 1,
					  "node_id": "MDQ6VXNlcjE=",
					  "avatar_url": "https://github.com/images/error/hubot_happy.gif",
					  "gravatar_id": "",
					  "url": "https://api.github.com/users/hubot",
					  "html_url": "https://github.com/hubot",
					  "followers_url": "https://api.github.com/users/hubot/followers",
					  "following_url": "https://api.github.com/users/hubot/following{/other_user}",
					  "gists_url": "https://api.github.com/users/hubot/gists{/gist_id}",
					  "starred_url": "https://api.github.com/users/hubot/starred{/owner}{/repo}",
					  "subscriptions_url": "https://api.github.com/users/hubot/subscriptions",
					  "organizations_url": "https://api.github.com/users/hubot/orgs",
					  "repos_url": "https://api.github.com/users/hubot/repos",
					  "events_url": "https://api.github.com/users/hubot/events{/privacy}",
					  "received_events_url": "https://api.github.com/users/hubot/received_events",
					  "type": "User",
					  "site_admin": true
					}
				  ],
				  "requested_reviewers": [
					{
					  "login": "other_user",
					  "id": 1,
					  "node_id": "MDQ6VXNlcjE=",
					  "avatar_url": "https://github.com/images/error/other_user_happy.gif",
					  "gravatar_id": "",
					  "url": "https://api.github.com/users/other_user",
					  "html_url": "https://github.com/other_user",
					  "followers_url": "https://api.github.com/users/other_user/followers",
					  "following_url": "https://api.github.com/users/other_user/following{/other_user}",
					  "gists_url": "https://api.github.com/users/other_user/gists{/gist_id}",
					  "starred_url": "https://api.github.com/users/other_user/starred{/owner}{/repo}",
					  "subscriptions_url": "https://api.github.com/users/other_user/subscriptions",
					  "organizations_url": "https://api.github.com/users/other_user/orgs",
					  "repos_url": "https://api.github.com/users/other_user/repos",
					  "events_url": "https://api.github.com/users/other_user/events{/privacy}",
					  "received_events_url": "https://api.github.com/users/other_user/received_events",
					  "type": "User",
					  "site_admin": false
					}
				  ],
				  "requested_teams": [
					{
					  "id": 1,
					  "node_id": "MDQ6VGVhbTE=",
					  "url": "https://api.github.com/teams/1",
					  "html_url": "https://github.com/orgs/github/teams/justice-league",
					  "name": "Justice League",
					  "slug": "justice-league",
					  "description": "A great team.",
					  "privacy": "closed",
					  "permission": "admin",
					  "notification_setting": "notifications_enabled",
					  "members_url": "https://api.github.com/teams/1/members{/member}",
					  "repositories_url": "https://api.github.com/teams/1/repos",
					  "parent": null
					}
				  ],
				  "head": {
					"label": "argo-cd:new-topic",
					"ref": "new-topic",
					"sha": "6dcb09b5b57875f334f61aebed695e2e4193db5e",
					"user": {
					  "login": "argo-cd",
					  "id": 1,
					  "node_id": "MDQ6VXNlcjE=",
					  "avatar_url": "https://github.com/images/error/argo-cd_happy.gif",
					  "gravatar_id": "",
					  "url": "https://api.github.com/users/argo-cd",
					  "html_url": "https://github.com/argo-cd",
					  "followers_url": "https://api.github.com/users/argo-cd/followers",
					  "following_url": "https://api.github.com/users/argo-cd/following{/other_user}",
					  "gists_url": "https://api.github.com/users/argo-cd/gists{/gist_id}",
					  "starred_url": "https://api.github.com/users/argo-cd/starred{/owner}{/repo}",
					  "subscriptions_url": "https://api.github.com/users/argo-cd/subscriptions",
					  "organizations_url": "https://api.github.com/users/argo-cd/orgs",
					  "repos_url": "https://api.github.com/users/argo-cd/repos",
					  "events_url": "https://api.github.com/users/argo-cd/events{/privacy}",
					  "received_events_url": "https://api.github.com/users/argo-cd/received_events",
					  "type": "User",
					  "site_admin": false
					},
					"repo": {
					  "id": 1296269,
					  "node_id": "MDEwOlJlcG9zaXRvcnkxMjk2MjY5",
					  "name": "Hello-World",
					  "full_name": "repos/argoproj/argo-cd",
					  "owner": {
						"login": "argo-cd",
						"id": 1,
						"node_id": "MDQ6VXNlcjE=",
						"avatar_url": "https://github.com/images/error/argo-cd_happy.gif",
						"gravatar_id": "",
						"url": "https://api.github.com/users/argo-cd",
						"html_url": "https://github.com/argo-cd",
						"followers_url": "https://api.github.com/users/argo-cd/followers",
						"following_url": "https://api.github.com/users/argo-cd/following{/other_user}",
						"gists_url": "https://api.github.com/users/argo-cd/gists{/gist_id}",
						"starred_url": "https://api.github.com/users/argo-cd/starred{/owner}{/repo}",
						"subscriptions_url": "https://api.github.com/users/argo-cd/subscriptions",
						"organizations_url": "https://api.github.com/users/argo-cd/orgs",
						"repos_url": "https://api.github.com/users/argo-cd/repos",
						"events_url": "https://api.github.com/users/argo-cd/events{/privacy}",
						"received_events_url": "https://api.github.com/users/argo-cd/received_events",
						"type": "User",
						"site_admin": false
					  },
					  "private": false,
					  "html_url": "https://github.com/repos/argoproj/argo-cd",
					  "description": "This your first repo!",
					  "fork": false,
					  "url": "https://api.github.com/repos/repos/argoproj/argo-cd",
					  "archive_url": "https://api.github.com/repos/repos/argoproj/argo-cd/{archive_format}{/ref}",
					  "assignees_url": "https://api.github.com/repos/repos/argoproj/argo-cd/assignees{/user}",
					  "blobs_url": "https://api.github.com/repos/repos/argoproj/argo-cd/git/blobs{/sha}",
					  "branches_url": "https://api.github.com/repos/repos/argoproj/argo-cd/branches{/branch}",
					  "collaborators_url": "https://api.github.com/repos/repos/argoproj/argo-cd/collaborators{/collaborator}",
					  "comments_url": "https://api.github.com/repos/repos/argoproj/argo-cd/comments{/number}",
					  "commits_url": "https://api.github.com/repos/repos/argoproj/argo-cd/commits{/sha}",
					  "compare_url": "https://api.github.com/repos/repos/argoproj/argo-cd/compare/{base}...{head}",
					  "contents_url": "https://api.github.com/repos/repos/argoproj/argo-cd/contents/{+path}",
					  "contributors_url": "https://api.github.com/repos/repos/argoproj/argo-cd/contributors",
					  "deployments_url": "https://api.github.com/repos/repos/argoproj/argo-cd/deployments",
					  "downloads_url": "https://api.github.com/repos/repos/argoproj/argo-cd/downloads",
					  "events_url": "https://api.github.com/repos/repos/argoproj/argo-cd/events",
					  "forks_url": "https://api.github.com/repos/repos/argoproj/argo-cd/forks",
					  "git_commits_url": "https://api.github.com/repos/repos/argoproj/argo-cd/git/commits{/sha}",
					  "git_refs_url": "https://api.github.com/repos/repos/argoproj/argo-cd/git/refs{/sha}",
					  "git_tags_url": "https://api.github.com/repos/repos/argoproj/argo-cd/git/tags{/sha}",
					  "git_url": "git:github.com/repos/argoproj/argo-cd.git",
					  "issue_comment_url": "https://api.github.com/repos/repos/argoproj/argo-cd/issues/comments{/number}",
					  "issue_events_url": "https://api.github.com/repos/repos/argoproj/argo-cd/issues/events{/number}",
					  "issues_url": "https://api.github.com/repos/repos/argoproj/argo-cd/issues{/number}",
					  "keys_url": "https://api.github.com/repos/repos/argoproj/argo-cd/keys{/key_id}",
					  "labels_url": "https://api.github.com/repos/repos/argoproj/argo-cd/labels{/name}",
					  "languages_url": "https://api.github.com/repos/repos/argoproj/argo-cd/languages",
					  "merges_url": "https://api.github.com/repos/repos/argoproj/argo-cd/merges",
					  "milestones_url": "https://api.github.com/repos/repos/argoproj/argo-cd/milestones{/number}",
					  "notifications_url": "https://api.github.com/repos/repos/argoproj/argo-cd/notifications{?since,all,participating}",
					  "pulls_url": "https://api.github.com/repos/repos/argoproj/argo-cd/pulls{/number}",
					  "releases_url": "https://api.github.com/repos/repos/argoproj/argo-cd/releases{/id}",
					  "ssh_url": "git@github.com:repos/argoproj/argo-cd.git",
					  "stargazers_url": "https://api.github.com/repos/repos/argoproj/argo-cd/stargazers",
					  "statuses_url": "https://api.github.com/repos/repos/argoproj/argo-cd/statuses/{sha}",
					  "subscribers_url": "https://api.github.com/repos/repos/argoproj/argo-cd/subscribers",
					  "subscription_url": "https://api.github.com/repos/repos/argoproj/argo-cd/subscription",
					  "tags_url": "https://api.github.com/repos/repos/argoproj/argo-cd/tags",
					  "teams_url": "https://api.github.com/repos/repos/argoproj/argo-cd/teams",
					  "trees_url": "https://api.github.com/repos/repos/argoproj/argo-cd/git/trees{/sha}",
					  "clone_url": "https://github.com/repos/argoproj/argo-cd.git",
					  "mirror_url": "git:git.example.com/repos/argoproj/argo-cd",
					  "hooks_url": "https://api.github.com/repos/repos/argoproj/argo-cd/hooks",
					  "svn_url": "https://svn.github.com/repos/argoproj/argo-cd",
					  "homepage": "https://github.com",
					  "language": null,
					  "forks_count": 9,
					  "stargazers_count": 80,
					  "watchers_count": 80,
					  "size": 108,
					  "default_branch": "master",
					  "open_issues_count": 0,
					  "is_template": true,
					  "topics": [
						"argo-cd",
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
					  "allow_rebase_merge": true,
					  "template_repository": null,
					  "temp_clone_token": "ABTLWHOULUVAXGTRYU7OC2876QJ2O",
					  "allow_squash_merge": true,
					  "allow_auto_merge": false,
					  "delete_branch_on_merge": true,
					  "allow_merge_commit": true,
					  "subscribers_count": 42,
					  "network_count": 0,
					  "license": {
						"key": "mit",
						"name": "MIT License",
						"url": "https://api.github.com/licenses/mit",
						"spdx_id": "MIT",
						"node_id": "MDc6TGljZW5zZW1pdA==",
						"html_url": "https://github.com/licenses/mit"
					  },
					  "forks": 1,
					  "open_issues": 1,
					  "watchers": 1
					}
				  },
				  "base": {
					"label": "argo-cd:master",
					"ref": "master",
					"sha": "6dcb09b5b57875f334f61aebed695e2e4193db5e",
					"user": {
					  "login": "argo-cd",
					  "id": 1,
					  "node_id": "MDQ6VXNlcjE=",
					  "avatar_url": "https://github.com/images/error/argo-cd_happy.gif",
					  "gravatar_id": "",
					  "url": "https://api.github.com/users/argo-cd",
					  "html_url": "https://github.com/argo-cd",
					  "followers_url": "https://api.github.com/users/argo-cd/followers",
					  "following_url": "https://api.github.com/users/argo-cd/following{/other_user}",
					  "gists_url": "https://api.github.com/users/argo-cd/gists{/gist_id}",
					  "starred_url": "https://api.github.com/users/argo-cd/starred{/owner}{/repo}",
					  "subscriptions_url": "https://api.github.com/users/argo-cd/subscriptions",
					  "organizations_url": "https://api.github.com/users/argo-cd/orgs",
					  "repos_url": "https://api.github.com/users/argo-cd/repos",
					  "events_url": "https://api.github.com/users/argo-cd/events{/privacy}",
					  "received_events_url": "https://api.github.com/users/argo-cd/received_events",
					  "type": "User",
					  "site_admin": false
					},
					"repo": {
					  "id": 1296269,
					  "node_id": "MDEwOlJlcG9zaXRvcnkxMjk2MjY5",
					  "name": "Hello-World",
					  "full_name": "repos/argoproj/argo-cd",
					  "owner": {
						"login": "argo-cd",
						"id": 1,
						"node_id": "MDQ6VXNlcjE=",
						"avatar_url": "https://github.com/images/error/argo-cd_happy.gif",
						"gravatar_id": "",
						"url": "https://api.github.com/users/argo-cd",
						"html_url": "https://github.com/argo-cd",
						"followers_url": "https://api.github.com/users/argo-cd/followers",
						"following_url": "https://api.github.com/users/argo-cd/following{/other_user}",
						"gists_url": "https://api.github.com/users/argo-cd/gists{/gist_id}",
						"starred_url": "https://api.github.com/users/argo-cd/starred{/owner}{/repo}",
						"subscriptions_url": "https://api.github.com/users/argo-cd/subscriptions",
						"organizations_url": "https://api.github.com/users/argo-cd/orgs",
						"repos_url": "https://api.github.com/users/argo-cd/repos",
						"events_url": "https://api.github.com/users/argo-cd/events{/privacy}",
						"received_events_url": "https://api.github.com/users/argo-cd/received_events",
						"type": "User",
						"site_admin": false
					  },
					  "private": false,
					  "html_url": "https://github.com/repos/argoproj/argo-cd",
					  "description": "This your first repo!",
					  "fork": false,
					  "url": "https://api.github.com/repos/repos/argoproj/argo-cd",
					  "archive_url": "https://api.github.com/repos/repos/argoproj/argo-cd/{archive_format}{/ref}",
					  "assignees_url": "https://api.github.com/repos/repos/argoproj/argo-cd/assignees{/user}",
					  "blobs_url": "https://api.github.com/repos/repos/argoproj/argo-cd/git/blobs{/sha}",
					  "branches_url": "https://api.github.com/repos/repos/argoproj/argo-cd/branches{/branch}",
					  "collaborators_url": "https://api.github.com/repos/repos/argoproj/argo-cd/collaborators{/collaborator}",
					  "comments_url": "https://api.github.com/repos/repos/argoproj/argo-cd/comments{/number}",
					  "commits_url": "https://api.github.com/repos/repos/argoproj/argo-cd/commits{/sha}",
					  "compare_url": "https://api.github.com/repos/repos/argoproj/argo-cd/compare/{base}...{head}",
					  "contents_url": "https://api.github.com/repos/repos/argoproj/argo-cd/contents/{+path}",
					  "contributors_url": "https://api.github.com/repos/repos/argoproj/argo-cd/contributors",
					  "deployments_url": "https://api.github.com/repos/repos/argoproj/argo-cd/deployments",
					  "downloads_url": "https://api.github.com/repos/repos/argoproj/argo-cd/downloads",
					  "events_url": "https://api.github.com/repos/repos/argoproj/argo-cd/events",
					  "forks_url": "https://api.github.com/repos/repos/argoproj/argo-cd/forks",
					  "git_commits_url": "https://api.github.com/repos/repos/argoproj/argo-cd/git/commits{/sha}",
					  "git_refs_url": "https://api.github.com/repos/repos/argoproj/argo-cd/git/refs{/sha}",
					  "git_tags_url": "https://api.github.com/repos/repos/argoproj/argo-cd/git/tags{/sha}",
					  "git_url": "git:github.com/repos/argoproj/argo-cd.git",
					  "issue_comment_url": "https://api.github.com/repos/repos/argoproj/argo-cd/issues/comments{/number}",
					  "issue_events_url": "https://api.github.com/repos/repos/argoproj/argo-cd/issues/events{/number}",
					  "issues_url": "https://api.github.com/repos/repos/argoproj/argo-cd/issues{/number}",
					  "keys_url": "https://api.github.com/repos/repos/argoproj/argo-cd/keys{/key_id}",
					  "labels_url": "https://api.github.com/repos/repos/argoproj/argo-cd/labels{/name}",
					  "languages_url": "https://api.github.com/repos/repos/argoproj/argo-cd/languages",
					  "merges_url": "https://api.github.com/repos/repos/argoproj/argo-cd/merges",
					  "milestones_url": "https://api.github.com/repos/repos/argoproj/argo-cd/milestones{/number}",
					  "notifications_url": "https://api.github.com/repos/repos/argoproj/argo-cd/notifications{?since,all,participating}",
					  "pulls_url": "https://api.github.com/repos/repos/argoproj/argo-cd/pulls{/number}",
					  "releases_url": "https://api.github.com/repos/repos/argoproj/argo-cd/releases{/id}",
					  "ssh_url": "git@github.com:repos/argoproj/argo-cd.git",
					  "stargazers_url": "https://api.github.com/repos/repos/argoproj/argo-cd/stargazers",
					  "statuses_url": "https://api.github.com/repos/repos/argoproj/argo-cd/statuses/{sha}",
					  "subscribers_url": "https://api.github.com/repos/repos/argoproj/argo-cd/subscribers",
					  "subscription_url": "https://api.github.com/repos/repos/argoproj/argo-cd/subscription",
					  "tags_url": "https://api.github.com/repos/repos/argoproj/argo-cd/tags",
					  "teams_url": "https://api.github.com/repos/repos/argoproj/argo-cd/teams",
					  "trees_url": "https://api.github.com/repos/repos/argoproj/argo-cd/git/trees{/sha}",
					  "clone_url": "https://github.com/repos/argoproj/argo-cd.git",
					  "mirror_url": "git:git.example.com/repos/argoproj/argo-cd",
					  "hooks_url": "https://api.github.com/repos/repos/argoproj/argo-cd/hooks",
					  "svn_url": "https://svn.github.com/repos/argoproj/argo-cd",
					  "homepage": "https://github.com",
					  "language": null,
					  "forks_count": 9,
					  "stargazers_count": 80,
					  "watchers_count": 80,
					  "size": 108,
					  "default_branch": "master",
					  "open_issues_count": 0,
					  "is_template": true,
					  "topics": [
						"argo-cd",
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
					  "allow_rebase_merge": true,
					  "template_repository": null,
					  "temp_clone_token": "ABTLWHOULUVAXGTRYU7OC2876QJ2O",
					  "allow_squash_merge": true,
					  "allow_auto_merge": false,
					  "delete_branch_on_merge": true,
					  "allow_merge_commit": true,
					  "subscribers_count": 42,
					  "network_count": 0,
					  "license": {
						"key": "mit",
						"name": "MIT License",
						"url": "https://api.github.com/licenses/mit",
						"spdx_id": "MIT",
						"node_id": "MDc6TGljZW5zZW1pdA==",
						"html_url": "https://github.com/licenses/mit"
					  },
					  "forks": 1,
					  "open_issues": 1,
					  "watchers": 1
					}
				  },
				  "_links": {
					"self": {
					  "href": "https://api.github.com/repos/repos/argoproj/argo-cd/pulls/1347"
					},
					"html": {
					  "href": "https://github.com/repos/argoproj/argo-cd/pull/1347"
					},
					"issue": {
					  "href": "https://api.github.com/repos/repos/argoproj/argo-cd/issues/1347"
					},
					"comments": {
					  "href": "https://api.github.com/repos/repos/argoproj/argo-cd/issues/1347/comments"
					},
					"review_comments": {
					  "href": "https://api.github.com/repos/repos/argoproj/argo-cd/pulls/1347/comments"
					},
					"review_comment": {
					  "href": "https://api.github.com/repos/repos/argoproj/argo-cd/pulls/comments{/number}"
					},
					"commits": {
					  "href": "https://api.github.com/repos/repos/argoproj/argo-cd/pulls/1347/commits"
					},
					"statuses": {
					  "href": "https://api.github.com/repos/repos/argoproj/argo-cd/statuses/6dcb09b5b57875f334f61aebed695e2e4193db5e"
					}
				  },
				  "author_association": "OWNER",
				  "auto_merge": null,
				  "draft": false
				}
			  ]`)
			if err != nil {
				t.Fail()
			}
		case "/api/v3/repos/argoproj/argo-cd/pulls/1347/files?per_page=100":
			_, err := io.WriteString(w, `[
			{
			  "sha": "bbcd538c8e72b8c175046e27cc8f907076331401",
			  "filename": "file1.txt",
			  "status": "added",
			  "additions": 103,
			  "deletions": 21,
			  "changes": 124,
			  "blob_url": "https://github.com/argoproj/argo-cd/blob/6dcb09b5b57875f334f61aebed695e2e4193db5e/file1.txt",
			  "raw_url": "https://github.com/argoproj/argo-cd/raw/6dcb09b5b57875f334f61aebed695e2e4193db5e/file1.txt",
			  "contents_url": "https://api.github.com/repos/argoproj/argo-cd/contents/file1.txt?ref=6dcb09b5b57875f334f61aebed695e2e4193db5e",
			  "patch": "@@ -132,7 +132,7 @@ module Test @@ -1000,7 +1000,7 @@ module Test"
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

func TestGithubListPulls(t *testing.T) {
	cases := []struct {
		name, proto, url     string
		hasError             bool
		filters              []v1alpha1.PullRequestGeneratorFilter
		pullRequestsExpected []*PullRequest
	}{
		{
			name:    "List PRs",
			url:     "git@github.com:argoproj/argo-cd.git",
			filters: []v1alpha1.PullRequestGeneratorFilter{},
			pullRequestsExpected: []*PullRequest{
				{
					Number:       1347,
					Title:        "Amazing new feature",
					Branch:       "new-topic",
					TargetBranch: "master",
					HeadSHA:      "6dcb09b5b57875f334f61aebed695e2e4193db5e",
					Labels:       []string{"bug"},
					Author:       "argo-cd",
					ChangedFiles: []string{"file1.txt"},
				},
			},
		},
		{
			name: "Only prs with files matching images/*",
			url:  "git@github.com:argoproj/argo-cd.git",
			filters: []v1alpha1.PullRequestGeneratorFilter{
				{
					FileMatch: strp("images/*"),
				},
			},
			pullRequestsExpected: []*PullRequest{},
		},
		{
			name: "PRs with files ending in txt",
			url:  "git@github.com:argoproj/argo-cd.git",
			filters: []v1alpha1.PullRequestGeneratorFilter{
				{
					FileMatch: strp(".*.txt"),
				},
			},
			pullRequestsExpected: []*PullRequest{
				{
					Number:       1347,
					Title:        "Amazing new feature",
					Branch:       "new-topic",
					TargetBranch: "master",
					HeadSHA:      "6dcb09b5b57875f334f61aebed695e2e4193db5e",
					Labels:       []string{"bug"},
					Author:       "argo-cd",
					ChangedFiles: []string{"file1.txt"},
				},
			},
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		githubMockHandler(t)(w, r)
	}))
	defer ts.Close()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, _ := NewGithubService(context.Background(), "", ts.URL, "argoproj", "argo-cd", nil)
			prs, err := ListPullRequests(context.Background(), svc, c.filters)
			assert.ElementsMatch(t, c.pullRequestsExpected, prs)
			assert.Len(t, prs, len(c.pullRequestsExpected))

			if c.hasError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

			}
		})
	}
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
