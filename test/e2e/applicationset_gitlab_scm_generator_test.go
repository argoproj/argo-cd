package e2e

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/applicationsets"
)

// gitlabSCMMockHandler serves a single GitLab group with one project, and paginates that
// project's branches endpoint over two pages. secondPageStatus controls what the second page
// returns, so a test can flip it from http.StatusOK to http.StatusNotFound partway through to
// simulate the project becoming unreadable in the middle of a listing.
func gitlabSCMMockHandler(t *testing.T, group string, projectID int, secondPageStatus *atomic.Int32) func(http.ResponseWriter, *http.Request) {
	t.Helper()
	projectsPath := fmt.Sprintf("/api/v4/groups/%s/projects", group)
	branchesPath := fmt.Sprintf("/api/v4/projects/%d/repository/branches", projectID)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case projectsPath:
			w.Header().Set("X-Page", "1")
			w.Header().Set("X-Total-Pages", "1")
			_, err := io.WriteString(w, fmt.Sprintf(`[{
				"id": %d,
				"path": "argo-cd",
				"default_branch": "master",
				"ssh_url_to_repo": "git@gitlab.com:%s/argo-cd.git",
				"http_url_to_repo": "https://gitlab.com/%s/argo-cd.git",
				"namespace": {"full_path": "%s"},
				"topics": []
			}]`, projectID, group, group, group))
			require.NoError(t, err)
		case branchesPath:
			switch r.URL.Query().Get("page") {
			case "", "1":
				w.Header().Set("X-Next-Page", "2")
				_, err := io.WriteString(w, `[{"name": "master", "commit": {"id": "8898d7999fc99dd0fd578650b58b244fc63f6b58"}}]`)
				require.NoError(t, err)
			case "2":
				status := http.StatusOK
				if secondPageStatus != nil {
					status = int(secondPageStatus.Load())
				}
				if status != http.StatusOK {
					w.WriteHeader(status)
					return
				}
				w.Header().Set("X-Next-Page", "0")
				_, err := io.WriteString(w, `[{"name": "feature", "commit": {"id": "2f1e3d4c5b6a798877665544332211009988aabb"}}]`)
				require.NoError(t, err)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

// TestGitlabSCMProviderGeneratorMidPagination404DoesNotPruneApplications reproduces the bug
// from #29323: a GitLab branch listing that 404s partway through a paginated read used to be
// reported as an empty but successful result, and the ApplicationSet reconciler trusted that
// enough to prune every Application it could no longer see. This test drives the SCM provider
// generator against a fake GitLab API that succeeds on page one and only starts 404ing on page
// two once the Applications already exist, and checks that the fix keeps them in place and
// records an error instead.
func TestGitlabSCMProviderGeneratorMidPagination404DoesNotPruneApplications(t *testing.T) {
	const group = "test-argocd-proton"
	const projectID = 27084533

	var secondPageStatus atomic.Int32
	secondPageStatus.Store(http.StatusOK)

	ts := testServerWithPort(t, 8342, http.HandlerFunc(gitlabSCMMockHandler(t, group, projectID, &secondPageStatus)))
	ts.Start()
	defer ts.Close()

	generateExpectedApp := func(branch string) v1alpha1.Application {
		return v1alpha1.Application{
			Kind:       application.ApplicationKind,
			APIVersion: "argoproj.io/v1alpha1",
			Name:       "argo-cd-" + branch,
			Namespace:  fixture.TestNamespace(),
			Finalizers: []string{v1alpha1.ResourcesFinalizerName},
			Spec: v1alpha1.ApplicationSpec{
				Project: "default",
				Source: &v1alpha1.ApplicationSource{
					RepoURL:        fmt.Sprintf("git@gitlab.com:%s/argo-cd.git", group),
					TargetRevision: branch,
					Path:           "guestbook",
				},
				Destination: v1alpha1.ApplicationDestination{
					Server:    "https://kubernetes.default.svc",
					Namespace: "guestbook",
				},
			},
		}
	}

	expectedApps := []v1alpha1.Application{
		generateExpectedApp("master"),
		generateExpectedApp("feature"),
	}

	requeueAfterSeconds := int64(2)

	Given(t).
		When().
		// Create an SCMProviderGenerator-based ApplicationSet pointed at the fake GitLab server
		Create(v1alpha1.ApplicationSet{
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{Name: "{{ repository }}-{{ branch }}"},
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "{{ url }}",
							TargetRevision: "{{ branch }}",
							Path:           "guestbook",
						},
						Destination: v1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: "guestbook",
						},
					},
				},
				Generators: []v1alpha1.ApplicationSetGenerator{
					{
						SCMProvider: &v1alpha1.SCMProviderGenerator{
							RequeueAfterSeconds: &requeueAfterSeconds,
							Gitlab: &v1alpha1.SCMProviderGeneratorGitlab{
								Group:       group,
								API:         ts.URL,
								AllBranches: true,
							},
						},
					},
				},
			},
		}).
		Then().
		// Both branches should show up as Applications while the fake server is healthy
		Expect(ApplicationsExist(expectedApps)).
		And(func() {
			// Now make the fake server fail partway through the branch listing, the same way a
			// GitLab project that lost token access mid-pagination would
			secondPageStatus.Store(http.StatusNotFound)
		}).
		// The Applications created from the earlier, successful listing must still be there.
		// If the old bug were still present, this generator error would instead look like an
		// empty branch list and the reconciler would delete both Applications.
		Expect(ApplicationsExist(expectedApps)).
		Expect(ApplicationSetHasCondition(
			v1alpha1.ApplicationSetConditionErrorOccurred,
			v1alpha1.ApplicationSetConditionStatusTrue,
			regexp.MustCompile("received 404 requesting page 2"),
			v1alpha1.ApplicationSetReasonApplicationParamsGenerationError,
		))
}
