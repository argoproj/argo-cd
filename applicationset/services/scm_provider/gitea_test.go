package scm_provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v2/applicationset/services/scm_provider/testdata"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
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
		case "/api/v1/repos/gitea/go-sdk/contents/README.md?ref=master":
			_, err := io.WriteString(w, `{
  "name": "README.md",
  "path": "README.md",
  "sha": "3605625ef3f80dc092167b54e3f55eb0663d729f",
  "last_commit_sha": "6b6fdd91ce769bb4641084e15f76554fb841bf27",
  "type": "file",
  "size": 1673,
  "encoding": "base64",
  "content": "IyBHaXRlYSBTREsgZm9yIEdvCgpbIVtMaWNlbnNlOiBNSVRdKGh0dHBzOi8vaW1nLnNoaWVsZHMuaW8vYmFkZ2UvTGljZW5zZS1NSVQtYmx1ZS5zdmcpXShodHRwczovL29wZW5zb3VyY2Uub3JnL2xpY2Vuc2VzL01JVCkgWyFbUmVsZWFzZV0oaHR0cHM6Ly9yYXN0ZXIuc2hpZWxkcy5pby9iYWRnZS9keW5hbWljL2pzb24uc3ZnP2xhYmVsPXJlbGVhc2UmdXJsPWh0dHBzOi8vZ2l0ZWEuY29tL2FwaS92MS9yZXBvcy9naXRlYS9nby1zZGsvcmVsZWFzZXMmcXVlcnk9JFswXS50YWdfbmFtZSldKGh0dHBzOi8vZ2l0ZWEuY29tL2dpdGVhL2dvLXNkay9yZWxlYXNlcykgWyFbQnVpbGQgU3RhdHVzXShodHRwczovL2Ryb25lLmdpdGVhLmNvbS9hcGkvYmFkZ2VzL2dpdGVhL2dvLXNkay9zdGF0dXMuc3ZnKV0oaHR0cHM6Ly9kcm9uZS5naXRlYS5jb20vZ2l0ZWEvZ28tc2RrKSBbIVtKb2luIHRoZSBjaGF0IGF0IGh0dHBzOi8vaW1nLnNoaWVsZHMuaW8vZGlzY29yZC8zMjI1Mzg5NTQxMTkxODQzODQuc3ZnXShodHRwczovL2ltZy5zaGllbGRzLmlvL2Rpc2NvcmQvMzIyNTM4OTU0MTE5MTg0Mzg0LnN2ZyldKGh0dHBzOi8vZGlzY29yZC5nZy9HaXRlYSkgWyFbXShodHRwczovL2ltYWdlcy5taWNyb2JhZGdlci5jb20vYmFkZ2VzL2ltYWdlL2dpdGVhL2dpdGVhLnN2ZyldKGh0dHA6Ly9taWNyb2JhZGdlci5jb20vaW1hZ2VzL2dpdGVhL2dpdGVhICJHZXQgeW91ciBvd24gaW1hZ2UgYmFkZ2Ugb24gbWljcm9iYWRnZXIuY29tIikgWyFbR28gUmVwb3J0IENhcmRdKGh0dHBzOi8vZ29yZXBvcnRjYXJkLmNvbS9iYWRnZS9jb2RlLmdpdGVhLmlvL3NkayldKGh0dHBzOi8vZ29yZXBvcnRjYXJkLmNvbS9yZXBvcnQvY29kZS5naXRlYS5pby9zZGspIFshW0dvRG9jXShodHRwczovL2dvZG9jLm9yZy9jb2RlLmdpdGVhLmlvL3Nkay9naXRlYT9zdGF0dXMuc3ZnKV0oaHR0cHM6Ly9nb2RvYy5vcmcvY29kZS5naXRlYS5pby9zZGsvZ2l0ZWEpCgpUaGlzIHByb2plY3QgYWN0cyBhcyBhIGNsaWVudCBTREsgaW1wbGVtZW50YXRpb24gd3JpdHRlbiBpbiBHbyB0byBpbnRlcmFjdCB3aXRoIHRoZSBHaXRlYSBBUEkgaW1wbGVtZW50YXRpb24uIEZvciBmdXJ0aGVyIGluZm9ybWF0aW9ucyB0YWtlIGEgbG9vayBhdCB0aGUgY3VycmVudCBbZG9jdW1lbnRhdGlvbl0oaHR0cHM6Ly9nb2RvYy5vcmcvY29kZS5naXRlYS5pby9zZGsvZ2l0ZWEpLgoKTm90ZTogZnVuY3Rpb24gYXJndW1lbnRzIGFyZSBlc2NhcGVkIGJ5IHRoZSBTREsuCgojIyBVc2UgaXQKCmBgYGdvCmltcG9ydCAiY29kZS5naXRlYS5pby9zZGsvZ2l0ZWEiCmBgYAoKIyMgVmVyc2lvbiBSZXF1aXJlbWVudHMKICogZ28gPj0gMS4xMwogKiBnaXRlYSA+PSAxLjExCgojIyBDb250cmlidXRpbmcKCkZvcmsgLT4gUGF0Y2ggLT4gUHVzaCAtPiBQdWxsIFJlcXVlc3QKCiMjIEF1dGhvcnMKCiogW01haW50YWluZXJzXShodHRwczovL2dpdGh1Yi5jb20vb3Jncy9nby1naXRlYS9wZW9wbGUpCiogW0NvbnRyaWJ1dG9yc10oaHR0cHM6Ly9naXRodWIuY29tL2dvLWdpdGVhL2dvLXNkay9ncmFwaHMvY29udHJpYnV0b3JzKQoKIyMgTGljZW5zZQoKVGhpcyBwcm9qZWN0IGlzIHVuZGVyIHRoZSBNSVQgTGljZW5zZS4gU2VlIHRoZSBbTElDRU5TRV0oTElDRU5TRSkgZmlsZSBmb3IgdGhlIGZ1bGwgbGljZW5zZSB0ZXh0Lgo=",
  "target": null,
  "url": "https://gitea.com/api/v1/repos/gitea/go-sdk/contents/README.md?ref=master",
  "html_url": "https://gitea.com/gitea/go-sdk/src/branch/master/README.md",
  "git_url": "https://gitea.com/api/v1/repos/gitea/go-sdk/git/blobs/3605625ef3f80dc092167b54e3f55eb0663d729f",
  "download_url": "https://gitea.com/gitea/go-sdk/raw/branch/master/README.md",
  "submodule_git_url": null,
  "_links": {
    "self": "https://gitea.com/api/v1/repos/gitea/go-sdk/contents/README.md?ref=master",
    "git": "https://gitea.com/api/v1/repos/gitea/go-sdk/git/blobs/3605625ef3f80dc092167b54e3f55eb0663d729f",
    "html": "https://gitea.com/gitea/go-sdk/src/branch/master/README.md"
  }
}
`)
			require.NoError(t, err)
		case "/api/v1/repos/gitea/go-sdk/contents/gitea?ref=master":
			_, err := io.WriteString(w, testdata.ReposGiteaGoSdkContentsGiteaResponse)
			require.NoError(t, err)
		case "/api/v1/repos/gitea/go-sdk/contents/notathing?ref=master":
			w.WriteHeader(http.StatusNotFound)
			_, err := io.WriteString(w, `{"errors":["object does not exist [id: , rel_path: notathing]"],"message":"GetContentsOrList","url":"https://gitea.com/api/swagger"}`)
			require.NoError(t, err)
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
				require.Error(t, err)
			} else {
				require.NoError(t, err)
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
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		giteaMockHandler(t)(w, r)
	}))
	defer ts.Close()
	host, _ := NewGiteaProvider(context.Background(), "gitea", "", ts.URL, false, false)
	repo := &Repository{
		Organization: "gitea",
		Repository:   "go-sdk",
		Branch:       "master",
	}

	t.Run("file exists", func(t *testing.T) {
		ok, err := host.RepoHasPath(context.Background(), repo, "README.md")
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("directory exists", func(t *testing.T) {
		ok, err := host.RepoHasPath(context.Background(), repo, "gitea")
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("does not exists", func(t *testing.T) {
		ok, err := host.RepoHasPath(context.Background(), repo, "notathing")
		require.NoError(t, err)
		assert.False(t, ok)
	})
}
