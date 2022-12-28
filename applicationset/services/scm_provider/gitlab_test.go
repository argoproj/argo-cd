package scm_provider

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

func gitlabMockHandler(t *testing.T) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.RequestURI {
		case "/api/v4":
			fmt.Println("here1")
		case "/api/v4/groups/test-argocd-proton/projects?include_subgroups=false&per_page=100":
			fmt.Println("here")
			_, err := io.WriteString(w, `[{
				"id": 27084533,
				"description": "",
				"name": "argocd",
				"name_with_namespace": "test argocd proton / argocd",
				"path": "argocd",
				"path_with_namespace": "test-argocd-proton/argocd",
				"created_at": "2021-06-01T17:30:44.724Z",
				"default_branch": "master",
				"tag_list": [],
				"topics": [],
				"ssh_url_to_repo": "git@gitlab.com:test-argocd-proton/argocd.git",
				"http_url_to_repo": "https://gitlab.com/test-argocd-proton/argocd.git",
				"web_url": "https://gitlab.com/test-argocd-proton/argocd",
				"readme_url": null,
				"avatar_url": null,
				"forks_count": 0,
				"star_count": 0,
				"last_activity_at": "2021-06-04T08:19:51.656Z",
				"namespace": {
					"id": 12258515,
					"name": "test argocd proton",
					"path": "test-argocd-proton",
					"kind": "gro* Connection #0 to host gitlab.com left intact up ",
					"full_path ": "test - argocd - proton ",
					"parent_id ": null,
					"avatar_url ": null,
					"web_url ": "https: //gitlab.com/groups/test-argocd-proton"
				},
				"container_registry_image_prefix": "registry.gitlab.com/test-argocd-proton/argocd",
				"_links": {
					"self": "https://gitlab.com/api/v4/projects/27084533",
					"issues": "https://gitlab.com/api/v4/projects/27084533/issues",
					"merge_requests": "https://gitlab.com/api/v4/projects/27084533/merge_requests",
					"repo_branches": "https://gitlab.com/api/v4/projects/27084533/repository/branches",
					"labels": "https://gitlab.com/api/v4/projects/27084533/labels",
					"events": "https://gitlab.com/api/v4/projects/27084533/events",
					"members": "https://gitlab.com/api/v4/projects/27084533/members",
					"cluster_agents": "https://gitlab.com/api/v4/projects/27084533/cluster_agents"
				},
				"packages_enabled": true,
				"empty_repo": false,
				"archived": false,
				"visibility": "public",
				"resolve_outdated_diff_discussions": false,
				"container_expiration_policy": {
					"cadence": "1d",
					"enabled": false,
					"keep_n": 10,
					"older_than": "90d",
					"name_regex": ".*",
					"name_regex_keep": null,
					"next_run_at": "2021-06-02T17:30:44.740Z"
				},
				"issues_enabled": true,
				"merge_requests_enabled": true,
				"wiki_enabled": true,
				"jobs_enabled": true,
				"snippets_enabled": true,
				"container_registry_enabled": true,
				"service_desk_enabled": true,
				"can_create_merge_request_in": false,
				"issues_access_level": "enabled",
				"repository_access_level": "enabled",
				"merge_requests_access_level": "enabled",
				"forking_access_level": "enabled",
				"wiki_access_level": "enabled",
				"builds_access_level": "enabled",
				"snippets_access_level": "enabled",
				"pages_access_level": "enabled",
				"operations_access_level": "enabled",
				"analytics_access_level": "enabled",
				"container_registry_access_level": "enabled",
				"security_and_compliance_access_level": "private",
				"emails_disabled": null,
				"shared_runners_enabled": true,
				"lfs_enabled": true,
				"creator_id": 2378866,
				"import_status": "none",
				"open_issues_count": 0,
				"ci_default_git_depth": 50,
				"ci_forward_deployment_enabled": true,
				"ci_job_token_scope_enabled": false,
				"public_jobs": true,
				"build_timeout": 3600,
				"auto_cancel_pending_pipelines": "enabled",
				"ci_config_path": "",
				"shared_with_groups": [],
				"only_allow_merge_if_pipeline_succeeds": false,
				"allow_merge_on_skipped_pipeline": null,
				"restrict_user_defined_variables": false,
				"request_access_enabled": true,
				"only_allow_merge_if_all_discussions_are_resolved": false,
				"remove_source_branch_after_merge": true,
				"printing_merge_request_link_enabled": true,
				"merge_method": "merge",
				"squash_option": "default_off",
				"suggestion_commit_message": null,
				"merge_commit_template": null,
				"squash_commit_template": null,
				"auto_devops_enabled": false,
				"auto_devops_deploy_strategy": "continuous",
				"autoclose_referenced_issues": true,
				"keep_latest_artifact": true,
				"runner_token_expiration_interval": null,
				"approvals_before_merge": 0,
				"mirror": false,
				"external_authorization_classification_label": "",
				"marked_for_deletion_at": null,
				"marked_for_deletion_on": null,
				"requirements_enabled": true,
				"requirements_access_level": "enabled",
				"security_and_compliance_enabled": false,
				"compliance_frameworks": [],
				"issues_template": null,
				"merge_requests_template": null,
				"merge_pipelines_enabled": false,
				"merge_trains_enabled": false
			}]`)
			if err != nil {
				t.Fail()
			}
		case "/api/v4/projects/27084533/repository/branches/master":
			fmt.Println("returning")
			_, err := io.WriteString(w, `{
				"name": "master",
				"commit": {
					"id": "8898d7999fc99dd0fd578650b58b244fc63f6b53",
					"short_id": "8898d799",
					"created_at": "2021-06-04T08:24:44.000+00:00",
					"parent_ids": ["3c9d50be1ef949ad28674e238c7e12a17b1e9706", "56482e001731640b4123cf177e51c696f08a3005"],
					"title": "Merge branch 'pipeline-1317911429' into 'master'",
					"message": "Merge branch 'pipeline-1317911429' into 'master'\n\n[testapp-ci] manifests/demo/test-app.yaml: release v1.1.0\n\nSee merge request test-argocd-proton/argocd!3",
					"author_name": "Martin Vozník",
					"author_email": "martin@voznik.cz",
					"authored_date": "2021-06-04T08:24:44.000+00:00",
					"committer_name": "Martin Vozník",
					"committer_email": "martin@voznik.cz",
					"committed_date": "2021-06-04T08:24:44.000+00:00",
					"trailers": {},
					"web_url": "https://gitlab.com/test-argocd-proton/argocd/-/commit/8898d7999fc99dd0fd578650b58b244fc63f6b53"
				},
				"merged": false,
				"protected": true,
				"developers_can_push": false,
				"developers_can_merge": false,
				"can_push": false,
				"default": true,
				"web_url": "https://gitlab.com/test-argocd-proton/argocd/-/tree/master"
			}`)
			if err != nil {
				t.Fail()
			}
		case "/api/v4/projects/27084533/repository/branches?per_page=100":
			_, err := io.WriteString(w, `[{
				"name": "master",
				"commit": {
					"id": "8898d7999fc99dd0fd578650b58b244fc63f6b53",
					"short_id": "8898d799",
					"created_at": "2021-06-04T08:24:44.000+00:00",
					"parent_ids": null,
					"title": "Merge branch 'pipeline-1317911429' into 'master'",
					"message": "Merge branch 'pipeline-1317911429' into 'master'",
					"author_name": "Martin Vozník",
					"author_email": "martin@voznik.cz",
					"authored_date": "2021-06-04T08:24:44.000+00:00",
					"committer_name": "Martin Vozník",
					"committer_email": "martin@voznik.cz",
					"committed_date": "2021-06-04T08:24:44.000+00:00",
					"trailers": null,
					"web_url": "https://gitlab.com/test-argocd-proton/argocd/-/commit/8898d7999fc99dd0fd578650b58b244fc63f6b53"
				},
				"merged": false,
				"protected": true,
				"developers_can_push": false,
				"developers_can_merge": false,
				"can_push": false,
				"default": true,
				"web_url": "https://gitlab.com/test-argocd-proton/argocd/-/tree/master"
			}, {
				"name": "pipeline-1310077506",
				"commit": {
					"id": "0f92540e5f396ba960adea4ed0aa905baf3f73d1",
					"short_id": "0f92540e",
					"created_at": "2021-06-01T18:39:59.000+00:00",
					"parent_ids": null,
					"title": "[testapp-ci] manifests/demo/test-app.yaml: release v1.0.1",
					"message": "[testapp-ci] manifests/demo/test-app.yaml: release v1.0.1",
					"author_name": "ci-test-app",
					"author_email": "mvoznik+cicd@protonmail.com",
					"authored_date": "2021-06-01T18:39:59.000+00:00",
					"committer_name": "ci-test-app",
					"committer_email": "mvoznik+cicd@protonmail.com",
					"committed_date": "2021-06-01T18:39:59.000+00:00",
					"trailers": null,
					"web_url": "https://gitlab.com/test-argocd-proton/argocd/-/commit/0f92540e5f396ba960adea4ed0aa905baf3f73d1"
				},
				"merged": false,
				"protected": false,
				"developers_can_push": false,
				"developers_can_merge": false,
				"can_push": false,
				"default": false,
				"web_url": "https://gitlab.com/test-argocd-proton/argocd/-/tree/pipeline-1310077506"
			}]`)
			if err != nil {
				t.Fail()
			}
		case "/api/v4/projects/test-argocd-proton%2Fargocd":
			fmt.Println("auct")
			_, err := io.WriteString(w, `{
				"id": 27084533,
				"description": "",
				"name": "argocd",
				"name_with_namespace": "test argocd proton / argocd",
				"path": "argocd",
				"path_with_namespace": "test-argocd-proton/argocd",
				"created_at": "2021-06-01T17:30:44.724Z",
				"default_branch": "master",
				"tag_list": [],
				"topics": [],
				"ssh_url_to_repo": "git@gitlab.com:test-argocd-proton/argocd.git",
				"http_url_to_repo": "https://gitlab.com/test-argocd-proton/argocd.git",
				"web_url": "https://gitlab.com/test-argocd-proton/argocd",
				"readme_url": null,
				"avatar_url": null,
				"forks_count": 0,
				"star_count": 0,
				"last_activity_at": "2021-06-04T08:19:51.656Z",
				"namespace": {
					"id": 12258515,
					"name": "test argocd proton",
					"path": "test-argocd-proton",
					"kind": "group",
					"full_path": "test-argocd-proton",
					"parent_id": null,
					"avatar_url": null,
					"web_url": "https://gitlab.com/groups/test-argocd-proton"
				}
			}`)
			if err != nil {
				t.Fail()
			}
		case "/api/v4/projects/27084533/repository/tree?path=argocd&ref=master":
			_, err := io.WriteString(w, `[{"id":"ca14f2a3718159c74572a5325fb4bfb0662a2d3e","name":"ingress.yaml","type":"blob","path":"argocd/ingress.yaml","mode":"100644"},{"id":"de2a53a73b1550b3e0f4d37ea0a6d878bf9c5096","name":"install.yaml","type":"blob","path":"argocd/install.yaml","mode":"100644"}]`)
			if err != nil {
				t.Fail()
			}
		case "/api/v4/projects/27084533/repository/tree?path=.&ref=master":
			_, err := io.WriteString(w, `[{"id":"f2bf99fa8f7a27df9c43d2dffc8c8cd747f3181a","name":"argocd","type":"tree","path":"argocd","mode":"040000"},{"id":"68a3125232e01c1583a6a6299534ce10c5e7dd83","name":"manifests","type":"tree","path":"manifests","mode":"040000"}]`)
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
func TestGitlabListRepos(t *testing.T) {
	cases := []struct {
		name, proto, url                        string
		hasError, allBranches, includeSubgroups bool
		branches                                []string
		filters                                 []v1alpha1.SCMProviderGeneratorFilter
	}{
		{
			name:     "blank protocol",
			url:      "git@gitlab.com:test-argocd-proton/argocd.git",
			branches: []string{"master"},
		},
		{
			name:  "ssh protocol",
			proto: "ssh",
			url:   "git@gitlab.com:test-argocd-proton/argocd.git",
		},
		{
			name:  "https protocol",
			proto: "https",
			url:   "https://gitlab.com/test-argocd-proton/argocd.git",
		},
		{
			name:     "other protocol",
			proto:    "other",
			hasError: true,
		},
		{
			name:        "all branches",
			allBranches: true,
			url:         "git@gitlab.com:test-argocd-proton/argocd.git",
			branches:    []string{"master"},
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gitlabMockHandler(t)(w, r)
	}))
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			provider, _ := NewGitlabProvider(context.Background(), "test-argocd-proton", "", ts.URL, c.allBranches, c.includeSubgroups)
			rawRepos, err := ListRepos(context.Background(), provider, c.filters, c.proto)
			if c.hasError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				// Just check that this one project shows up. Not a great test but better thing nothing?
				repos := []*Repository{}
				branches := []string{}
				for _, r := range rawRepos {
					if r.Repository == "argocd" {
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

func TestGitlabHasPath(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gitlabMockHandler(t)(w, r)
	}))
	host, _ := NewGitlabProvider(context.Background(), "test-argocd-proton", "", ts.URL, false, true)
	repo := &Repository{
		Organization: "test-argocd-proton",
		Repository:   "argocd",
		Branch:       "master",
	}

	cases := []struct {
		name, path string
		exists     bool
	}{
		{
			name:   "directory exists",
			path:   "argocd",
			exists: true,
		},
		{
			name:   "file exists",
			path:   "argocd/install.yaml",
			exists: true,
		},
		{
			name:   "directory does not exist",
			path:   "notathing",
			exists: false,
		},
		{
			name:   "file does not exist",
			path:   "argocd/notathing.yaml",
			exists: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ok, err := host.RepoHasPath(context.Background(), repo, c.path)
			assert.Nil(t, err)
			assert.Equal(t, c.exists, ok)
		})
	}
}
