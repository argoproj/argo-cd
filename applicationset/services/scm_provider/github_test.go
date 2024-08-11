package scm_provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
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
				},
				{
				  "id": 1296270,
				  "node_id": "MDEwOlJlcGsddRvcnkxMjk2MjY5",
				  "name": "another-repo",
				  "full_name": "argoproj/another-repo",
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
				  "html_url": "https://github.com/argoproj/another-repo",
				  "description": "This your first repo!",
				  "fork": false,
				  "url": "https://api.github.com/repos/argoproj/another-repo",
				  "archive_url": "https://api.github.com/repos/argoproj/another-repo/{archive_format}{/ref}",
				  "assignees_url": "https://api.github.com/repos/argoproj/another-repo/assignees{/user}",
				  "blobs_url": "https://api.github.com/repos/argoproj/another-repo/git/blobs{/sha}",
				  "branches_url": "https://api.github.com/repos/argoproj/another-repo/branches{/branch}",
				  "collaborators_url": "https://api.github.com/repos/argoproj/another-repo/collaborators{/collaborator}",
				  "comments_url": "https://api.github.com/repos/argoproj/another-repo/comments{/number}",
				  "commits_url": "https://api.github.com/repos/argoproj/another-repo/commits{/sha}",
				  "compare_url": "https://api.github.com/repos/argoproj/another-repo/compare/{base}...{head}",
				  "contents_url": "https://api.github.com/repos/argoproj/another-repo/contents/{path}",
				  "contributors_url": "https://api.github.com/repos/argoproj/another-repo/contributors",
				  "deployments_url": "https://api.github.com/repos/argoproj/another-repo/deployments",
				  "downloads_url": "https://api.github.com/repos/argoproj/another-repo/downloads",
				  "events_url": "https://api.github.com/repos/argoproj/another-repo/events",
				  "forks_url": "https://api.github.com/repos/argoproj/another-repo/forks",
				  "git_commits_url": "https://api.github.com/repos/argoproj/another-repo/git/commits{/sha}",
				  "git_refs_url": "https://api.github.com/repos/argoproj/another-repo/git/refs{/sha}",
				  "git_tags_url": "https://api.github.com/repos/argoproj/another-repo/git/tags{/sha}",
				  "git_url": "git:github.com/argoproj/another-repo.git",
				  "issue_comment_url": "https://api.github.com/repos/argoproj/another-repo/issues/comments{/number}",
				  "issue_events_url": "https://api.github.com/repos/argoproj/another-repo/issues/events{/number}",
				  "issues_url": "https://api.github.com/repos/argoproj/another-repo/issues{/number}",
				  "keys_url": "https://api.github.com/repos/argoproj/another-repo/keys{/key_id}",
				  "labels_url": "https://api.github.com/repos/argoproj/another-repo/labels{/name}",
				  "languages_url": "https://api.github.com/repos/argoproj/another-repo/languages",
				  "merges_url": "https://api.github.com/repos/argoproj/another-repo/merges",
				  "milestones_url": "https://api.github.com/repos/argoproj/another-repo/milestones{/number}",
				  "notifications_url": "https://api.github.com/repos/argoproj/another-repo/notifications{?since,all,participating}",
				  "pulls_url": "https://api.github.com/repos/argoproj/another-repo/pulls{/number}",
				  "releases_url": "https://api.github.com/repos/argoproj/another-repo/releases{/id}",
				  "ssh_url": "git@github.com:argoproj/another-repo.git",
				  "stargazers_url": "https://api.github.com/repos/argoproj/another-repo/stargazers",
				  "statuses_url": "https://api.github.com/repos/argoproj/another-repo/statuses/{sha}",
				  "subscribers_url": "https://api.github.com/repos/argoproj/another-repo/subscribers",
				  "subscription_url": "https://api.github.com/repos/argoproj/another-repo/subscription",
				  "tags_url": "https://api.github.com/repos/argoproj/another-repo/tags",
				  "teams_url": "https://api.github.com/repos/argoproj/another-repo/teams",
				  "trees_url": "https://api.github.com/repos/argoproj/another-repo/git/trees{/sha}",
				  "clone_url": "https://github.com/argoproj/another-repo.git",
				  "mirror_url": "git:git.example.com/argoproj/another-repo",
				  "hooks_url": "https://api.github.com/repos/argoproj/another-repo/hooks",
				  "svn_url": "https://svn.github.com/argoproj/another-repo",
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
				  "archived": true,
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
		case "/api/v3/repos/argoproj/argo-cd/branches?per_page=100":
			_, err := io.WriteString(w, `[
				{
				  "name": "master",
				  "commit": {
					"sha": "c5b97d5ae6c19d5c5df71a34c7fbeeda2479ccbc",
					"url": "https://api.github.com/repos/argoproj/argo-cd/commits/c5b97d5ae6c19d5c5df71a34c7fbeeda2479ccbc"
				  },
				  "protected": true,
				  "protection": {
					"required_status_checks": {
					  "enforcement_level": "non_admins",
					  "contexts": [
						"ci-test",
						"linter"
					  ]
					}
				  },
				  "protection_url": "https://api.github.com/repos/argoproj/hello-world/branches/master/protection"
				},
				{
				  "name": "test",
				  "commit": {
					"sha": "80a6e93f16e8093e24091b03c614362df3fb9b92",
					"url": "https://api.github.com/repos/argoproj/argo-cd/commits/80a6e93f16e8093e24091b03c614362df3fb9b92"
				  },
				  "protected": true,
				  "protection": {
					"required_status_checks": {
					  "enforcement_level": "non_admins",
					  "contexts": [
						"ci-test",
						"linter"
					  ]
					}
				  },
				  "protection_url": "https://api.github.com/repos/argoproj/hello-world/branches/master/protection"
				}
			  ]
			`)
			if err != nil {
				t.Fail()
			}
		case "/api/v3/repos/argoproj/another-repo/branches?per_page=100":
			_, err := io.WriteString(w, `[
				{
				  "name": "main",
				  "commit": {
					"sha": "19b016818bc0e0a44ddeaab345838a2a6c97fa67",
					"url": "https://api.github.com/repos/argoproj/another-repo/commits/19b016818bc0e0a44ddeaab345838a2a6c97fa67"
				  },
				  "protected": true,
				  "protection": {
					"required_status_checks": {
					  "enforcement_level": "non_admins",
					  "contexts": [
						"ci-test",
						"linter"
					  ]
					}
				  },
				  "protection_url": "https://api.github.com/repos/argoproj/hello-world/branches/master/protection"
				}
			  ]
			`)
			if err != nil {
				t.Fail()
			}
		case "/api/v3/repos/argoproj/argo-cd/contents/pkg?ref=master":
			_, err := io.WriteString(w, `{
				"type": "file",
				"encoding": "base64",
				"size": 5362,
				"name": "pkg/",
				"path": "pkg/",
				"content": "encoded content ...",
				"sha": "3d21ec53a331a6f037a91c368710b99387d012c1",
				"url": "https://api.github.com/repos/octokit/octokit.rb/contents/README.md",
				"git_url": "https://api.github.com/repos/octokit/octokit.rb/git/blobs/3d21ec53a331a6f037a91c368710b99387d012c1",
				"html_url": "https://github.com/octokit/octokit.rb/blob/master/README.md",
				"download_url": "https://raw.githubusercontent.com/octokit/octokit.rb/master/README.md",
				"_links": {
				  "git": "https://api.github.com/repos/octokit/octokit.rb/git/blobs/3d21ec53a331a6f037a91c368710b99387d012c1",
				  "self": "https://api.github.com/repos/octokit/octokit.rb/contents/README.md",
				  "html": "https://github.com/octokit/octokit.rb/blob/master/README.md"
				}
			  }`)
			if err != nil {
				t.Fail()
			}
		case "/api/v3/repos/argoproj/argo-cd/branches/master":
			_, err := io.WriteString(w, `{
				"name": "master",
				"commit": {
				  "sha": "c5b97d5ae6c19d5c5df71a34c7fbeeda2479ccbc",
				  "url": "https://api.github.com/repos/octocat/Hello-World/commits/c5b97d5ae6c19d5c5df71a34c7fbeeda2479ccbc"
				},
				"protected": true,
				"protection": {
				  "required_status_checks": {
					"enforcement_level": "non_admins",
					"contexts": [
					  "ci-test",
					  "linter"
					]
				  }
				},
				"protection_url": "https://api.github.com/repos/octocat/hello-world/branches/master/protection"
			  }`)
			if err != nil {
				t.Fail()
			}
		case "/api/v3/repos/argoproj/argo-cd/branches/test":
			_, err := io.WriteString(w, `{
				"name": "test",
				"commit": {
				  "sha": "80a6e93f16e8093e24091b03c614362df3fb9b92",
				  "url": "https://api.github.com/repos/octocat/Hello-World/commits/80a6e93f16e8093e24091b03c614362df3fb9b92"
				},
				"protected": true,
				"protection": {
				  "required_status_checks": {
					"enforcement_level": "non_admins",
					"contexts": [
					  "ci-test",
					  "linter"
					]
				  }
				},
				"protection_url": "https://api.github.com/repos/octocat/hello-world/branches/test/protection"
			  }`)
			if err != nil {
				t.Fail()
			}
		case "/api/v3/repos/argoproj/another-repo/branches/main":
			_, err := io.WriteString(w, `{
					"name": "main",
					"commit": {
						"sha": "19b016818bc0e0a44ddeaab345838a2a6c97fa67",
						"url": "https://api.github.com/repos/octocat/Hello-World/commits/c5b97d5ae6c19d5c5df71a34c7fbeeda2479ccbc"
					},
					"protected": true,
					"protection": {
						"required_status_checks": {
						"enforcement_level": "non_admins",
						"contexts": [
							"ci-test",
							"linter"
						]
						}
					},
					"protection_url": "https://api.github.com/repos/octocat/hello-world/branches/master/protection"
					}`)
			if err != nil {
				t.Fail()
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func TestGithubListRepos(t *testing.T) {
	idptr := func(i int64) *int64 {
		return &i
	}
	// Test cases for ListRepos
	cases := []struct {
		name, proto           string
		hasError, allBranches bool
		expectedRepos         []*Repository
		filters               []v1alpha1.SCMProviderGeneratorFilter
	}{
		{
			name:        "blank protocol",
			allBranches: true,
			expectedRepos: []*Repository{
				{
					Organization: "argoproj",
					Repository:   "argo-cd",
					Branch:       "master",
					URL:          "git@github.com:argoproj/argo-cd.git",
					SHA:          "c5b97d5ae6c19d5c5df71a34c7fbeeda2479ccbc",
					Labels: []string{
						"argoproj",
						"atom",
						"electron",
						"api",
					},
					RepositoryId: idptr(1296269),
					Archived:     false,
				},
				{
					Organization: "argoproj",
					Repository:   "argo-cd",
					Branch:       "test",
					URL:          "git@github.com:argoproj/argo-cd.git",
					SHA:          "80a6e93f16e8093e24091b03c614362df3fb9b92",
					Labels: []string{
						"argoproj",
						"atom",
						"electron",
						"api",
					},
					RepositoryId: idptr(1296269),
					Archived:     false,
				},
				{
					Organization: "argoproj",
					Repository:   "another-repo",
					Branch:       "main",
					URL:          "git@github.com:argoproj/another-repo.git",
					SHA:          "19b016818bc0e0a44ddeaab345838a2a6c97fa67",
					Labels: []string{
						"argoproj",
						"atom",
						"electron",
						"api",
					},
					RepositoryId: idptr(1296270),
					Archived:     true,
				},
			},
			filters: []v1alpha1.SCMProviderGeneratorFilter{
				{
					IncludeArchivedRepos: true,
				},
			},
		},
		{
			name:        "ssh protocol",
			proto:       "ssh",
			allBranches: true,
			expectedRepos: []*Repository{
				{
					Organization: "argoproj",
					Repository:   "argo-cd",
					Branch:       "master",
					URL:          "git@github.com:argoproj/argo-cd.git",
					SHA:          "c5b97d5ae6c19d5c5df71a34c7fbeeda2479ccbc",
					Labels: []string{
						"argoproj",
						"atom",
						"electron",
						"api",
					},
					RepositoryId: idptr(1296269),
					Archived:     false,
				},
				{
					Organization: "argoproj",
					Repository:   "argo-cd",
					Branch:       "test",
					URL:          "git@github.com:argoproj/argo-cd.git",
					SHA:          "80a6e93f16e8093e24091b03c614362df3fb9b92",
					Labels: []string{
						"argoproj",
						"atom",
						"electron",
						"api",
					},
					RepositoryId: idptr(1296269),
					Archived:     false,
				},
				{
					Organization: "argoproj",
					Repository:   "another-repo",
					Branch:       "main",
					URL:          "git@github.com:argoproj/another-repo.git",
					SHA:          "19b016818bc0e0a44ddeaab345838a2a6c97fa67",
					Labels: []string{
						"argoproj",
						"atom",
						"electron",
						"api",
					},
					RepositoryId: idptr(1296270),
					Archived:     true,
				},
			},
			filters: []v1alpha1.SCMProviderGeneratorFilter{
				{
					IncludeArchivedRepos: true,
				},
			},
		},
		{
			name:        "https protocol",
			proto:       "https",
			allBranches: true,
			expectedRepos: []*Repository{
				{
					Organization: "argoproj",
					Repository:   "argo-cd",
					Branch:       "master",
					URL:          "https://github.com/argoproj/argo-cd.git",
					SHA:          "c5b97d5ae6c19d5c5df71a34c7fbeeda2479ccbc",
					Labels: []string{
						"argoproj",
						"atom",
						"electron",
						"api",
					},
					RepositoryId: idptr(1296269),
					Archived:     false,
				},
				{
					Organization: "argoproj",
					Repository:   "argo-cd",
					Branch:       "test",
					URL:          "https://github.com/argoproj/argo-cd.git",
					SHA:          "80a6e93f16e8093e24091b03c614362df3fb9b92",
					Labels: []string{
						"argoproj",
						"atom",
						"electron",
						"api",
					},
					RepositoryId: idptr(1296269),
					Archived:     false,
				},
				{
					Organization: "argoproj",
					Repository:   "another-repo",
					Branch:       "main",
					URL:          "https://github.com/argoproj/another-repo.git",
					SHA:          "19b016818bc0e0a44ddeaab345838a2a6c97fa67",
					Labels: []string{
						"argoproj",
						"atom",
						"electron",
						"api",
					},
					RepositoryId: idptr(1296270),
					Archived:     true,
				},
			},
			filters: []v1alpha1.SCMProviderGeneratorFilter{
				{
					IncludeArchivedRepos: true,
				},
			},
		},
		{
			name:          "other protocol",
			proto:         "other",
			hasError:      true,
			expectedRepos: []*Repository{},
			filters: []v1alpha1.SCMProviderGeneratorFilter{
				{
					IncludeArchivedRepos: true,
				},
			},
		},
		{
			name:        "all branches with archived repos",
			allBranches: true,
			proto:       "ssh",
			expectedRepos: []*Repository{
				{
					Organization: "argoproj",
					Repository:   "argo-cd",
					Branch:       "master",
					URL:          "git@github.com:argoproj/argo-cd.git",
					SHA:          "c5b97d5ae6c19d5c5df71a34c7fbeeda2479ccbc",
					Labels: []string{
						"argoproj",
						"atom",
						"electron",
						"api",
					},
					RepositoryId: idptr(1296269),
					Archived:     false,
				},
				{
					Organization: "argoproj",
					Repository:   "argo-cd",
					Branch:       "test",
					URL:          "git@github.com:argoproj/argo-cd.git",
					SHA:          "80a6e93f16e8093e24091b03c614362df3fb9b92",
					Labels: []string{
						"argoproj",
						"atom",
						"electron",
						"api",
					},
					RepositoryId: idptr(1296269),
					Archived:     false,
				},
				{
					Organization: "argoproj",
					Repository:   "another-repo",
					Branch:       "main",
					URL:          "git@github.com:argoproj/another-repo.git",
					SHA:          "19b016818bc0e0a44ddeaab345838a2a6c97fa67",
					Labels: []string{
						"argoproj",
						"atom",
						"electron",
						"api",
					},
					RepositoryId: idptr(1296270),
					Archived:     true,
				},
			},
			filters: []v1alpha1.SCMProviderGeneratorFilter{
				{
					IncludeArchivedRepos: true,
				},
			},
		},
		{
			name:        "test repo all branches",
			allBranches: true,
			proto:       "https",
			expectedRepos: []*Repository{
				{
					Organization: "argoproj",
					Repository:   "argo-cd",
					Branch:       "master",
					URL:          "https://github.com/argoproj/argo-cd.git",
					SHA:          "c5b97d5ae6c19d5c5df71a34c7fbeeda2479ccbc",
					Labels: []string{
						"argoproj",
						"atom",
						"electron",
						"api",
					},
					RepositoryId: idptr(1296269),
					Archived:     false,
				},
				{
					Organization: "argoproj",
					Repository:   "argo-cd",
					Branch:       "test",
					URL:          "https://github.com/argoproj/argo-cd.git",
					SHA:          "80a6e93f16e8093e24091b03c614362df3fb9b92",
					Labels: []string{
						"argoproj",
						"atom",
						"electron",
						"api",
					},
					RepositoryId: idptr(1296269),
					Archived:     false,
				},
			},
			filters: []v1alpha1.SCMProviderGeneratorFilter{
				{
					IncludeArchivedRepos: false,
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
			provider, _ := NewGithubProvider(context.Background(), "argoproj", "", ts.URL, c.allBranches)
			rawRepos, err := ListRepos(context.Background(), provider, c.filters, c.proto)
			if c.hasError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				repos := []*Repository{}

				repos = append(rawRepos, repos...)
				assert.NotEmpty(t, repos)
				assert.Len(t, repos, len(c.expectedRepos))
				assert.ElementsMatch(t, c.expectedRepos, repos)
			}
		})
	}
}

func TestGithubHasPath(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		githubMockHandler(t)(w, r)
	}))
	defer ts.Close()
	host, _ := NewGithubProvider(context.Background(), "argoproj", "", ts.URL, false)
	repo := &Repository{
		Organization: "argoproj",
		Repository:   "argo-cd",
		Branch:       "master",
	}
	ok, err := host.RepoHasPath(context.Background(), repo, "pkg/")
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = host.RepoHasPath(context.Background(), repo, "notathing/")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestGithubGetBranches(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		githubMockHandler(t)(w, r)
	}))
	defer ts.Close()
	// all branches set to false
	host, _ := NewGithubProvider(context.Background(), "argoproj", "", ts.URL, false)
	repo := &Repository{
		Organization: "argoproj",
		Repository:   "argo-cd",
		Branch:       "master",
	}
	repos, err := host.GetBranches(context.Background(), repo)
	if err != nil {
		require.NoError(t, err)
	} else {
		assert.Equal(t, "master", repos[0].Branch)
	}
	// Branch Doesn't exists instead of error will return no error
	repo2 := &Repository{
		Organization: "argoproj",
		Repository:   "applicationset",
		Branch:       "main",
	}

	_, err = host.GetBranches(context.Background(), repo2)
	require.NoError(t, err)

	// Get all branches
	host.allBranches = true
	repos, err = host.GetBranches(context.Background(), repo)
	if err != nil {
		require.NoError(t, err)
	} else {
		// considering master  branch to  exist.
		assert.Len(t, repos, 2)
	}
}
