package scm_provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/v2/pkg/apis/applicationset/v1alpha1"
)

func giteaMockHandler(t *testing.T) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.RequestURI {
		case "/api/v1/version":
			_, err := io.WriteString(w, `{"version":"1.17.0+dev-452-g1f0541780"}`)
			if err != nil {
				t.Fail()
			}
		case "/api/v1/orgs/test-argocd/repos?limit=0&page=1":
			_, err := io.WriteString(w, `[{
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
				}]`)
			if err != nil {
				t.Fail()
			}
		case "/api/v1/repos/test-argocd/pr-test/branches/main":
			_, err := io.WriteString(w, `{
				"name": "main",
				"commit": {
					"id": "72687815ccba81ef014a96201cc2e846a68789d8",
					"message": "initial commit\n",
					"url": "https://gitea.com/test-argocd/pr-test/commit/72687815ccba81ef014a96201cc2e846a68789d8",
					"author": {
						"name": "Dan Molik",
						"email": "dan@danmolik.com",
						"username": "graytshirt"
					},
					"committer": {
						"name": "Dan Molik",
						"email": "dan@danmolik.com",
						"username": "graytshirt"
					},
					"verification": {
						"verified": false,
						"reason": "gpg.error.no_gpg_keys_found",
						"signature": "-----BEGIN PGP SIGNATURE-----\n\niQEzBAABCAAdFiEEXYAkwEBRpXzXgHFWlgCr7m50zBMFAmJMiqUACgkQlgCr7m50\nzBPSmQgAiVVEIxC42tuks4iGFNURrtYvypZAEIc+hJgt2kBpmdCrAphYPeAj+Wtr\n9KT7dDscCZIba2wx39HEXO2S7wNCXESvAzrA8rdfbXjR4L2miZ1urfBkEoqK5i/F\noblWGuAyjurX4KPa2ARROd0H4AXxt6gNAXaFPgZO+xXCyNKZfad/lkEP1AiPRknD\nvTTMbEkIzFHK9iVwZ9DORGpfF1wnLzxWmMfhYatZnBgFNnoeJNtFhCJo05rHBgqc\nqVZWXt1iF7nysBoXSzyx1ZAsmBr/Qerkuj0nonh0aPVa6NKJsdmeJyPX4zXXoi6E\ne/jpxX2UQJkpFezg3IjUpvE5FvIiYg==\n=3Af2\n-----END PGP SIGNATURE-----\n",
						"signer": null,
						"payload": "tree 64d47c7fc6e31dcf00654223ec4ab749dd0a464e\nauthor Dan Molik \u003cdan@danmolik.com\u003e 1649183391 -0400\ncommitter Dan Molik \u003cdan@danmolik.com\u003e 1649183391 -0400\n\ninitial commit\n"
					},
					"timestamp": "2022-04-05T14:29:51-04:00",
					"added": null,
					"removed": null,
					"modified": null
				},
				"protected": false,
				"required_approvals": 0,
				"enable_status_check": false,
				"status_check_contexts": [],
				"user_can_push": false,
				"user_can_merge": false,
				"effective_branch_protection_name": ""
			}`)
			if err != nil {
				t.Fail()
			}
		case "/api/v1/repos/test-argocd/pr-test/branches?limit=0&page=1":
			_, err := io.WriteString(w, `[{
				"name": "main",
				"commit": {
					"id": "72687815ccba81ef014a96201cc2e846a68789d8",
					"message": "initial commit\n",
					"url": "https://gitea.com/test-argocd/pr-test/commit/72687815ccba81ef014a96201cc2e846a68789d8",
					"author": {
						"name": "Dan Molik",
						"email": "dan@danmolik.com",
						"username": "graytshirt"
					},
					"committer": {
						"name": "Dan Molik",
						"email": "dan@danmolik.com",
						"username": "graytshirt"
					},
					"verification": {
						"verified": false,
						"reason": "gpg.error.no_gpg_keys_found",
						"signature": "-----BEGIN PGP SIGNATURE-----\n\niQEzBAABCAAdFiEEXYAkwEBRpXzXgHFWlgCr7m50zBMFAmJMiqUACgkQlgCr7m50\nzBPSmQgAiVVEIxC42tuks4iGFNURrtYvypZAEIc+hJgt2kBpmdCrAphYPeAj+Wtr\n9KT7dDscCZIba2wx39HEXO2S7wNCXESvAzrA8rdfbXjR4L2miZ1urfBkEoqK5i/F\noblWGuAyjurX4KPa2ARROd0H4AXxt6gNAXaFPgZO+xXCyNKZfad/lkEP1AiPRknD\nvTTMbEkIzFHK9iVwZ9DORGpfF1wnLzxWmMfhYatZnBgFNnoeJNtFhCJo05rHBgqc\nqVZWXt1iF7nysBoXSzyx1ZAsmBr/Qerkuj0nonh0aPVa6NKJsdmeJyPX4zXXoi6E\ne/jpxX2UQJkpFezg3IjUpvE5FvIiYg==\n=3Af2\n-----END PGP SIGNATURE-----\n",
						"signer": null,
						"payload": "tree 64d47c7fc6e31dcf00654223ec4ab749dd0a464e\nauthor Dan Molik \u003cdan@danmolik.com\u003e 1649183391 -0400\ncommitter Dan Molik \u003cdan@danmolik.com\u003e 1649183391 -0400\n\ninitial commit\n"
					},
					"timestamp": "2022-04-05T14:29:51-04:00",
					"added": null,
					"removed": null,
					"modified": null
				},
				"protected": false,
				"required_approvals": 0,
				"enable_status_check": false,
				"status_check_contexts": [],
				"user_can_push": false,
				"user_can_merge": false,
				"effective_branch_protection_name": ""
			}, {
				"name": "test",
				"commit": {
					"id": "7bbaf62d92ddfafd9cc8b340c619abaec32bc09f",
					"message": "add an empty file\n",
					"url": "https://gitea.com/test-argocd/pr-test/commit/7bbaf62d92ddfafd9cc8b340c619abaec32bc09f",
					"author": {
						"name": "Dan Molik",
						"email": "dan@danmolik.com",
						"username": "graytshirt"
					},
					"committer": {
						"name": "Dan Molik",
						"email": "dan@danmolik.com",
						"username": "graytshirt"
					},
					"verification": {
						"verified": false,
						"reason": "gpg.error.no_gpg_keys_found",
						"signature": "-----BEGIN PGP SIGNATURE-----\n\niQEzBAABCAAdFiEEXYAkwEBRpXzXgHFWlgCr7m50zBMFAmJMiugACgkQlgCr7m50\nzBN+7wgAkCHD3KfX3Ffkqv2qPwqgHNYM1bA6Hmffzhv0YeD9jWCI3tp0JulP4iFZ\ncQ7jqx9xP9tCQMSFCaijLRHaE6Js1xrVtf0OKRkbpdlvkyrIM3sQhqyQgAsISrDG\nLzSqeoQQjglzeWESYh2Tjn1CgqQNKjI6LLepSwvF1pIeV4pJpJobaEbIfTgStdzM\nWEk8o0I+EZaYqK0C0vU9N0LK/LR/jnlaHsb4OUjvk+S7lRjZwBkrsg7P/QsqtCVd\nw5nkxDiCx1J58zKMnQ7ZinJEK9A5WYdnMYc6aBn7ARgZrblXPPBkkKUhEv3ZSPeW\nKv9i4GQy838xkVSTFkHNj1+a5o6zEA==\n=JiFw\n-----END PGP SIGNATURE-----\n",
						"signer": null,
						"payload": "tree cdddf3e1d6a8a7e6899a044d0e1bc73bf798e2f5\nparent 72687815ccba81ef014a96201cc2e846a68789d8\nauthor Dan Molik \u003cdan@danmolik.com\u003e 1649183458 -0400\ncommitter Dan Molik \u003cdan@danmolik.com\u003e 1649183458 -0400\n\nadd an empty file\n"
					},
					"timestamp": "2022-04-05T14:30:58-04:00",
					"added": null,
					"removed": null,
					"modified": null
				},
				"protected": false,
				"required_approvals": 0,
				"enable_status_check": false,
				"status_check_contexts": [],
				"user_can_push": false,
				"user_can_merge": false,
				"effective_branch_protection_name": ""
			}]`)
			if err != nil {
				t.Fail()
			}
		default:
			_, err := io.WriteString(w, `[]`)
			if err != nil {
				t.Fail()
			}
		}
	}
}
func TestGiteaListRepos(t *testing.T) {
	cases := []struct {
		name, proto, url                        string
		hasError, allBranches, includeSubgroups bool
		branches                                []string
		filters                                 []v1alpha1.SCMProviderGeneratorFilter
	}{
		{
			name:        "blank protocol",
			allBranches: false,
			url:         "git@gitea.com:test-argocd/pr-test.git",
			branches:    []string{"main"},
		},
		{
			name:        "ssh protocol",
			allBranches: false,
			proto:       "ssh",
			url:         "git@gitea.com:test-argocd/pr-test.git",
		},
		{
			name:        "https protocol",
			allBranches: false,
			proto:       "https",
			url:         "https://gitea.com/test-argocd/pr-test",
		},
		{
			name:        "other protocol",
			allBranches: false,
			proto:       "other",
			hasError:    true,
		},
		{
			name:        "all branches",
			allBranches: true,
			url:         "git@gitea.com:test-argocd/pr-test.git",
			branches:    []string{"main"},
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		giteaMockHandler(t)(w, r)
	}))
	defer ts.Close()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			provider, _ := NewGiteaProvider(context.Background(), "test-argocd", "", ts.URL, c.allBranches, false)
			rawRepos, err := ListRepos(context.Background(), provider, c.filters, c.proto)
			if c.hasError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				// Just check that this one project shows up. Not a great test but better thing nothing?
				repos := []*Repository{}
				branches := []string{}
				for _, r := range rawRepos {
					if r.Repository == "pr-test" {
						repos = append(repos, r)
						branches = append(branches, r.Branch)
					}
				}
				assert.NotEmpty(t, repos)
				assert.Equal(t, c.url, repos[0].URL)
				for _, b := range c.branches {
					assert.Contains(t, branches, b)
				}
			}
		})
	}
}

func TestGiteaHasPath(t *testing.T) {
	host, _ := NewGiteaProvider(context.Background(), "gitea", "", "https://gitea.com/", false, false)
	repo := &Repository{
		Organization: "gitea",
		Repository:   "go-sdk",
		Branch:       "master",
	}
	ok, err := host.RepoHasPath(context.Background(), repo, "README.md")
	assert.Nil(t, err)
	assert.True(t, ok)

	// directory
	ok, err = host.RepoHasPath(context.Background(), repo, "gitea")
	assert.Nil(t, err)
	assert.True(t, ok)

	ok, err = host.RepoHasPath(context.Background(), repo, "notathing")
	assert.Nil(t, err)
	assert.False(t, ok)
}
