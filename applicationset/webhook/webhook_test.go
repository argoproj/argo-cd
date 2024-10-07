package webhook

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/argoproj/argo-cd/v2/applicationset/generators"
	"github.com/argoproj/argo-cd/v2/applicationset/services/scm_provider"
	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argosettings "github.com/argoproj/argo-cd/v2/util/settings"
)

type generatorMock struct {
	mock.Mock
}

func (g *generatorMock) GetTemplate(appSetGenerator *v1alpha1.ApplicationSetGenerator) *v1alpha1.ApplicationSetTemplate {
	return &v1alpha1.ApplicationSetTemplate{}
}

func (g *generatorMock) GenerateParams(appSetGenerator *v1alpha1.ApplicationSetGenerator, _ *v1alpha1.ApplicationSet, client client.Client) ([]map[string]interface{}, error) {
	return []map[string]interface{}{}, nil
}

func (g *generatorMock) GetRequeueAfter(appSetGenerator *v1alpha1.ApplicationSetGenerator) time.Duration {
	d, _ := time.ParseDuration("10s")
	return d
}

func TestWebhookHandler(t *testing.T) {
	tt := []struct {
		desc               string
		headerKey          string
		headerValue        string
		effectedAppSets    []string
		payloadFile        string
		expectedStatusCode int
		expectedRefresh    bool
	}{
		{
			desc:               "WebHook from a GitHub repository via Commit",
			headerKey:          "X-GitHub-Event",
			headerValue:        "push",
			payloadFile:        "github-commit-event.json",
			effectedAppSets:    []string{"git-github", "matrix-git-github", "merge-git-github", "matrix-scm-git-github", "matrix-nested-git-github", "merge-nested-git-github", "plugin", "matrix-pull-request-github-plugin"},
			expectedStatusCode: http.StatusOK,
			expectedRefresh:    true,
		},
		{
			desc:               "WebHook from a GitHub repository via Commit shorthand",
			headerKey:          "X-GitHub-Event",
			headerValue:        "push",
			payloadFile:        "github-commit-event-feature-branch.json",
			effectedAppSets:    []string{"github-shorthand", "matrix-pull-request-github-plugin", "plugin"},
			expectedStatusCode: http.StatusOK,
			expectedRefresh:    true,
		},
		{
			desc:               "WebHook from a GitHub repository via Commit to branch",
			headerKey:          "X-GitHub-Event",
			headerValue:        "push",
			payloadFile:        "github-commit-branch-event.json",
			effectedAppSets:    []string{"git-github", "plugin", "matrix-pull-request-github-plugin"},
			expectedStatusCode: http.StatusOK,
			expectedRefresh:    true,
		},
		{
			desc:               "WebHook from a GitHub ping event",
			headerKey:          "X-GitHub-Event",
			headerValue:        "ping",
			payloadFile:        "github-ping-event.json",
			effectedAppSets:    []string{"git-github", "plugin"},
			expectedStatusCode: http.StatusOK,
			expectedRefresh:    false,
		},
		{
			desc:               "WebHook from a GitLab repository via Commit",
			headerKey:          "X-Gitlab-Event",
			headerValue:        "Push Hook",
			payloadFile:        "gitlab-event.json",
			effectedAppSets:    []string{"git-gitlab", "plugin", "matrix-pull-request-github-plugin"},
			expectedStatusCode: http.StatusOK,
			expectedRefresh:    true,
		},
		{
			desc:               "WebHook with an unknown event",
			headerKey:          "X-Random-Event",
			headerValue:        "Push Hook",
			payloadFile:        "gitlab-event.json",
			effectedAppSets:    []string{"git-gitlab", "plugin"},
			expectedStatusCode: http.StatusBadRequest,
			expectedRefresh:    false,
		},
		{
			desc:               "WebHook with an invalid event",
			headerKey:          "X-Random-Event",
			headerValue:        "Push Hook",
			payloadFile:        "invalid-event.json",
			effectedAppSets:    []string{"git-gitlab", "plugin"},
			expectedStatusCode: http.StatusBadRequest,
			expectedRefresh:    false,
		},
		{
			desc:               "WebHook from a GitHub repository via pull_request opened event",
			headerKey:          "X-GitHub-Event",
			headerValue:        "pull_request",
			payloadFile:        "github-pull-request-opened-event.json",
			effectedAppSets:    []string{"pull-request-github", "matrix-pull-request-github", "matrix-scm-pull-request-github", "merge-pull-request-github", "plugin", "matrix-pull-request-github-plugin"},
			expectedStatusCode: http.StatusOK,
			expectedRefresh:    true,
		},
		{
			desc:               "WebHook from a GitHub repository via pull_request assigned event",
			headerKey:          "X-GitHub-Event",
			headerValue:        "pull_request",
			payloadFile:        "github-pull-request-assigned-event.json",
			effectedAppSets:    []string{"pull-request-github", "matrix-pull-request-github", "matrix-scm-pull-request-github", "merge-pull-request-github", "plugin", "matrix-pull-request-github-plugin"},
			expectedStatusCode: http.StatusOK,
			expectedRefresh:    false,
		},
		{
			desc:               "WebHook from a GitHub repository via pull_request labeled event",
			headerKey:          "X-GitHub-Event",
			headerValue:        "pull_request",
			payloadFile:        "github-pull-request-labeled-event.json",
			effectedAppSets:    []string{"pull-request-github", "matrix-pull-request-github", "matrix-scm-pull-request-github", "merge-pull-request-github", "plugin", "matrix-pull-request-github-plugin"},
			expectedStatusCode: http.StatusOK,
			expectedRefresh:    true,
		},
		{
			desc:               "WebHook from a GitLab repository via open merge request event",
			headerKey:          "X-Gitlab-Event",
			headerValue:        "Merge Request Hook",
			payloadFile:        "gitlab-merge-request-open-event.json",
			effectedAppSets:    []string{"pull-request-gitlab", "plugin", "matrix-pull-request-github-plugin"},
			expectedStatusCode: http.StatusOK,
			expectedRefresh:    true,
		},
		{
			desc:               "WebHook from a GitLab repository via approval merge request event",
			headerKey:          "X-Gitlab-Event",
			headerValue:        "Merge Request Hook",
			payloadFile:        "gitlab-merge-request-approval-event.json",
			effectedAppSets:    []string{"pull-request-gitlab", "plugin"},
			expectedStatusCode: http.StatusOK,
			expectedRefresh:    false,
		},
		{
			desc:               "WebHook from a Azure DevOps repository via Commit",
			headerKey:          "X-Vss-Activityid",
			headerValue:        "Push Hook",
			payloadFile:        "azuredevops-push.json",
			effectedAppSets:    []string{"git-azure-devops", "plugin", "matrix-pull-request-github-plugin"},
			expectedStatusCode: http.StatusOK,
			expectedRefresh:    true,
		},
		{
			desc:               "WebHook from a Azure DevOps repository via pull request event",
			headerKey:          "X-Vss-Activityid",
			headerValue:        "Pull Request Hook",
			payloadFile:        "azuredevops-pull-request.json",
			effectedAppSets:    []string{"pull-request-azure-devops", "plugin", "matrix-pull-request-github-plugin"},
			expectedStatusCode: http.StatusOK,
			expectedRefresh:    true,
		},
	}

	namespace := "test"
	webhookParallelism := 10
	fakeClient := newFakeClient(namespace)
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)
	err = v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	for _, test := range tt {
		t.Run(test.desc, func(t *testing.T) {
			fc := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
				fakeAppWithGitGenerator("git-github", namespace, "https://github.com/org/repo"),
				fakeAppWithGitGenerator("git-gitlab", namespace, "https://gitlab/group/name"),
				fakeAppWithGitGenerator("git-azure-devops", namespace, "https://dev.azure.com/fabrikam-fiber-inc/DefaultCollection/_git/Fabrikam-Fiber-Git"),
				fakeAppWithGitGeneratorWithRevision("github-shorthand", namespace, "https://github.com/org/repo", "env/dev"),
				fakeAppWithGithubPullRequestGenerator("pull-request-github", namespace, "CodErTOcat", "Hello-World"),
				fakeAppWithGitlabPullRequestGenerator("pull-request-gitlab", namespace, "100500"),
				fakeAppWithAzureDevOpsPullRequestGenerator("pull-request-azure-devops", namespace, "DefaultCollection", "Fabrikam"),
				fakeAppWithPluginGenerator("plugin", namespace),
				fakeAppWithMatrixAndGitGenerator("matrix-git-github", namespace, "https://github.com/org/repo"),
				fakeAppWithMatrixAndPullRequestGenerator("matrix-pull-request-github", namespace, "Codertocat", "Hello-World"),
				fakeAppWithMatrixAndScmWithGitGenerator("matrix-scm-git-github", namespace, "org"),
				fakeAppWithMatrixAndScmWithPullRequestGenerator("matrix-scm-pull-request-github", namespace, "Codertocat"),
				fakeAppWithMatrixAndNestedGitGenerator("matrix-nested-git-github", namespace, "https://github.com/org/repo"),
				fakeAppWithMatrixAndPullRequestGeneratorWithPluginGenerator("matrix-pull-request-github-plugin", namespace, "coDErtoCat", "HeLLO-WorLD", "plugin-cm"),
				fakeAppWithMergeAndGitGenerator("merge-git-github", namespace, "https://github.com/org/repo"),
				fakeAppWithMergeAndPullRequestGenerator("merge-pull-request-github", namespace, "Codertocat", "Hello-World"),
				fakeAppWithMergeAndNestedGitGenerator("merge-nested-git-github", namespace, "https://github.com/org/repo"),
			).Build()
			set := argosettings.NewSettingsManager(context.TODO(), fakeClient, namespace)
			h, err := NewWebhookHandler(namespace, webhookParallelism, set, fc, mockGenerators())
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/api/webhook", nil)
			req.Header.Set(test.headerKey, test.headerValue)
			eventJSON, err := os.ReadFile(filepath.Join("testdata", test.payloadFile))
			require.NoError(t, err)
			req.Body = io.NopCloser(bytes.NewReader(eventJSON))
			w := httptest.NewRecorder()

			h.Handler(w, req)
			close(h.queue)
			h.Wait()
			assert.Equal(t, test.expectedStatusCode, w.Code)

			list := &v1alpha1.ApplicationSetList{}
			err = fc.List(context.TODO(), list)
			require.NoError(t, err)
			effectedAppSetsAsExpected := make(map[string]bool)
			for _, appSetName := range test.effectedAppSets {
				effectedAppSetsAsExpected[appSetName] = false
			}
			for i := range list.Items {
				gotAppSet := &list.Items[i]
				if _, isEffected := effectedAppSetsAsExpected[gotAppSet.Name]; isEffected {
					if expected, got := test.expectedRefresh, gotAppSet.RefreshRequired(); expected != got {
						t.Errorf("unexpected RefreshRequired() for appset '%s' expect: %v got: %v", gotAppSet.Name, expected, got)
					}
					effectedAppSetsAsExpected[gotAppSet.Name] = true
				} else {
					assert.False(t, gotAppSet.RefreshRequired())
				}
			}
			for appSetName, checked := range effectedAppSetsAsExpected {
				assert.True(t, checked, "appset %s not found", appSetName)
			}
		})
	}
}

func mockGenerators() map[string]generators.Generator {
	// generatorMockList := generatorMock{}
	generatorMockGit := &generatorMock{}
	generatorMockPR := &generatorMock{}
	generatorMockPlugin := &generatorMock{}
	mockSCMProvider := &scm_provider.MockProvider{
		Repos: []*scm_provider.Repository{
			{
				Organization: "myorg",
				Repository:   "repo1",
				URL:          "git@github.com:org/repo.git",
				Branch:       "main",
				SHA:          "0bc57212c3cbbec69d20b34c507284bd300def5b",
			},
			{
				Organization: "Codertocat",
				Repository:   "Hello-World",
				URL:          "git@github.com:Codertocat/Hello-World.git",
				Branch:       "main",
				SHA:          "59d0",
			},
		},
	}
	generatorMockSCM := generators.NewTestSCMProviderGenerator(mockSCMProvider)

	terminalMockGenerators := map[string]generators.Generator{
		"List":        generators.NewListGenerator(),
		"Git":         generatorMockGit,
		"SCMProvider": generatorMockSCM,
		"PullRequest": generatorMockPR,
		"Plugin":      generatorMockPlugin,
	}

	nestedGenerators := map[string]generators.Generator{
		"List":        terminalMockGenerators["List"],
		"Git":         terminalMockGenerators["Git"],
		"SCMProvider": terminalMockGenerators["SCMProvider"],
		"PullRequest": terminalMockGenerators["PullRequest"],
		"Plugin":      terminalMockGenerators["Plugin"],
		"Matrix":      generators.NewMatrixGenerator(terminalMockGenerators),
		"Merge":       generators.NewMergeGenerator(terminalMockGenerators),
	}

	return map[string]generators.Generator{
		"List":        terminalMockGenerators["List"],
		"Git":         terminalMockGenerators["Git"],
		"SCMProvider": terminalMockGenerators["SCMProvider"],
		"PullRequest": terminalMockGenerators["PullRequest"],
		"Plugin":      terminalMockGenerators["Plugin"],
		"Matrix":      generators.NewMatrixGenerator(nestedGenerators),
		"Merge":       generators.NewMergeGenerator(nestedGenerators),
	}
}

func TestGenRevisionHasChanged(t *testing.T) {
	type args struct {
		gen         *v1alpha1.GitGenerator
		revision    string
		touchedHead bool
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{name: "touchedHead", args: args{
			gen:         &v1alpha1.GitGenerator{},
			revision:    "main",
			touchedHead: true,
		}, want: true},
		{name: "didntTouchHead", args: args{
			gen:         &v1alpha1.GitGenerator{},
			revision:    "main",
			touchedHead: false,
		}, want: false},
		{name: "foundEqualShort", args: args{
			gen:         &v1alpha1.GitGenerator{Revision: "dev"},
			revision:    "dev",
			touchedHead: true,
		}, want: true},
		{name: "foundEqualLongGen", args: args{
			gen:         &v1alpha1.GitGenerator{Revision: "refs/heads/dev"},
			revision:    "dev",
			touchedHead: true,
		}, want: true},
		{name: "foundNotEqualLongGen", args: args{
			gen:         &v1alpha1.GitGenerator{Revision: "refs/heads/dev"},
			revision:    "main",
			touchedHead: true,
		}, want: false},
		{name: "foundNotEqualShort", args: args{
			gen:         &v1alpha1.GitGenerator{Revision: "dev"},
			revision:    "main",
			touchedHead: false,
		}, want: false},
		{name: "foundEqualTag", args: args{
			gen:         &v1alpha1.GitGenerator{Revision: "v3.14.1"},
			revision:    "v3.14.1",
			touchedHead: false,
		}, want: true},
		{name: "foundEqualTagLongGen", args: args{
			gen:         &v1alpha1.GitGenerator{Revision: "refs/tags/v3.14.1"},
			revision:    "v3.14.1",
			touchedHead: false,
		}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, genRevisionHasChanged(tt.args.gen, tt.args.revision, tt.args.touchedHead), "genRevisionHasChanged(%v, %v, %v)", tt.args.gen, tt.args.revision, tt.args.touchedHead)
		})
	}
}

func fakeAppWithGitGenerator(name, namespace, repo string) *v1alpha1.ApplicationSet {
	return &v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.ApplicationSetSpec{
			Generators: []v1alpha1.ApplicationSetGenerator{
				{
					Git: &v1alpha1.GitGenerator{
						RepoURL:  repo,
						Revision: "master",
					},
				},
			},
		},
	}
}

func fakeAppWithGitGeneratorWithRevision(name, namespace, repo, revision string) *v1alpha1.ApplicationSet {
	appSet := fakeAppWithGitGenerator(name, namespace, repo)
	appSet.Spec.Generators[0].Git.Revision = revision
	return appSet
}

func fakeAppWithGitlabPullRequestGenerator(name, namespace, projectId string) *v1alpha1.ApplicationSet {
	return &v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.ApplicationSetSpec{
			Generators: []v1alpha1.ApplicationSetGenerator{
				{
					PullRequest: &v1alpha1.PullRequestGenerator{
						GitLab: &v1alpha1.PullRequestGeneratorGitLab{
							Project: projectId,
						},
					},
				},
			},
		},
	}
}

func fakeAppWithGithubPullRequestGenerator(name, namespace, owner, repo string) *v1alpha1.ApplicationSet {
	return &v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.ApplicationSetSpec{
			Generators: []v1alpha1.ApplicationSetGenerator{
				{
					PullRequest: &v1alpha1.PullRequestGenerator{
						Github: &v1alpha1.PullRequestGeneratorGithub{
							Owner: owner,
							Repo:  repo,
						},
					},
				},
			},
		},
	}
}

func fakeAppWithAzureDevOpsPullRequestGenerator(name, namespace, project, repo string) *v1alpha1.ApplicationSet {
	return &v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.ApplicationSetSpec{
			Generators: []v1alpha1.ApplicationSetGenerator{
				{
					PullRequest: &v1alpha1.PullRequestGenerator{
						AzureDevOps: &v1alpha1.PullRequestGeneratorAzureDevOps{
							Project: project,
							Repo:    repo,
						},
					},
				},
			},
		},
	}
}

func fakeAppWithMatrixAndGitGenerator(name, namespace, repo string) *v1alpha1.ApplicationSet {
	return &v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.ApplicationSetSpec{
			Generators: []v1alpha1.ApplicationSetGenerator{
				{
					Matrix: &v1alpha1.MatrixGenerator{
						Generators: []v1alpha1.ApplicationSetNestedGenerator{
							{
								List: &v1alpha1.ListGenerator{},
							},
							{
								Git: &v1alpha1.GitGenerator{
									RepoURL: repo,
								},
							},
						},
					},
				},
			},
		},
	}
}

func fakeAppWithMatrixAndPullRequestGenerator(name, namespace, owner, repo string) *v1alpha1.ApplicationSet {
	return &v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.ApplicationSetSpec{
			Generators: []v1alpha1.ApplicationSetGenerator{
				{
					Matrix: &v1alpha1.MatrixGenerator{
						Generators: []v1alpha1.ApplicationSetNestedGenerator{
							{
								List: &v1alpha1.ListGenerator{},
							},
							{
								PullRequest: &v1alpha1.PullRequestGenerator{
									Github: &v1alpha1.PullRequestGeneratorGithub{
										Owner: owner,
										Repo:  repo,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func fakeAppWithMatrixAndScmWithGitGenerator(name, namespace, owner string) *v1alpha1.ApplicationSet {
	return &v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.ApplicationSetSpec{
			Generators: []v1alpha1.ApplicationSetGenerator{
				{
					Matrix: &v1alpha1.MatrixGenerator{
						Generators: []v1alpha1.ApplicationSetNestedGenerator{
							{
								SCMProvider: &v1alpha1.SCMProviderGenerator{
									CloneProtocol: "ssh",
									Github: &v1alpha1.SCMProviderGeneratorGithub{
										Organization: owner,
									},
								},
							},
							{
								Git: &v1alpha1.GitGenerator{
									RepoURL: "{{ url }}",
								},
							},
						},
					},
				},
			},
		},
	}
}

func fakeAppWithMatrixAndScmWithPullRequestGenerator(name, namespace, owner string) *v1alpha1.ApplicationSet {
	return &v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.ApplicationSetSpec{
			Generators: []v1alpha1.ApplicationSetGenerator{
				{
					Matrix: &v1alpha1.MatrixGenerator{
						Generators: []v1alpha1.ApplicationSetNestedGenerator{
							{
								SCMProvider: &v1alpha1.SCMProviderGenerator{
									CloneProtocol: "https",
									Github: &v1alpha1.SCMProviderGeneratorGithub{
										Organization: owner,
									},
								},
							},
							{
								PullRequest: &v1alpha1.PullRequestGenerator{
									Github: &v1alpha1.PullRequestGeneratorGithub{
										Owner: "{{ organization }}",
										Repo:  "{{ repository }}",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func fakeAppWithMatrixAndNestedGitGenerator(name, namespace, repo string) *v1alpha1.ApplicationSet {
	return &v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.ApplicationSetSpec{
			Generators: []v1alpha1.ApplicationSetGenerator{
				{
					Matrix: &v1alpha1.MatrixGenerator{
						Generators: []v1alpha1.ApplicationSetNestedGenerator{
							{
								List: &v1alpha1.ListGenerator{},
							},
							{
								Matrix: &apiextensionsv1.JSON{
									Raw: []byte(fmt.Sprintf(`{
										"Generators": [
											{
												"List": {
													"Elements": [
														{
															"repository": "%s"
														}
													]
												}
											},
											{
												"Git": {
													"RepoURL": "{{ repository }}"
												}
											}
										]
									}`, repo)),
								},
							},
						},
					},
				},
			},
		},
	}
}

func fakeAppWithMergeAndGitGenerator(name, namespace, repo string) *v1alpha1.ApplicationSet {
	return &v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.ApplicationSetSpec{
			Generators: []v1alpha1.ApplicationSetGenerator{
				{
					Merge: &v1alpha1.MergeGenerator{
						Generators: []v1alpha1.ApplicationSetNestedGenerator{
							{
								Git: &v1alpha1.GitGenerator{
									RepoURL: repo,
								},
							},
						},
					},
				},
			},
		},
	}
}

func fakeAppWithMergeAndPullRequestGenerator(name, namespace, owner, repo string) *v1alpha1.ApplicationSet {
	return &v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.ApplicationSetSpec{
			Generators: []v1alpha1.ApplicationSetGenerator{
				{
					Merge: &v1alpha1.MergeGenerator{
						Generators: []v1alpha1.ApplicationSetNestedGenerator{
							{
								PullRequest: &v1alpha1.PullRequestGenerator{
									Github: &v1alpha1.PullRequestGeneratorGithub{
										Owner: owner,
										Repo:  repo,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func fakeAppWithMergeAndNestedGitGenerator(name, namespace, repo string) *v1alpha1.ApplicationSet {
	return &v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.ApplicationSetSpec{
			Generators: []v1alpha1.ApplicationSetGenerator{
				{
					Merge: &v1alpha1.MergeGenerator{
						MergeKeys: []string{
							"server",
						},
						Generators: []v1alpha1.ApplicationSetNestedGenerator{
							{
								List: &v1alpha1.ListGenerator{},
							},
							{
								Merge: &apiextensionsv1.JSON{
									Raw: []byte(fmt.Sprintf(`{
										"MergeKeys": ["server"],
										"Generators": [
											{
												"List": {}
											},
											{
												"Git": {
													"RepoURL": "%s"
												}
											}
										]
									}`, repo)),
								},
							},
						},
					},
				},
			},
		},
	}
}

func fakeAppWithPluginGenerator(name, namespace string) *v1alpha1.ApplicationSet {
	return &v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.ApplicationSetSpec{
			Generators: []v1alpha1.ApplicationSetGenerator{
				{
					Plugin: &v1alpha1.PluginGenerator{
						ConfigMapRef: v1alpha1.PluginConfigMapRef{
							Name: "test",
						},
					},
				},
			},
		},
	}
}

func fakeAppWithMatrixAndPullRequestGeneratorWithPluginGenerator(name, namespace, owner, repo, configmapName string) *v1alpha1.ApplicationSet {
	return &v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.ApplicationSetSpec{
			Generators: []v1alpha1.ApplicationSetGenerator{
				{
					Matrix: &v1alpha1.MatrixGenerator{
						Generators: []v1alpha1.ApplicationSetNestedGenerator{
							{
								PullRequest: &v1alpha1.PullRequestGenerator{
									Github: &v1alpha1.PullRequestGeneratorGithub{
										Owner: owner,
										Repo:  repo,
									},
								},
							},
							{
								Plugin: &v1alpha1.PluginGenerator{
									ConfigMapRef: v1alpha1.PluginConfigMapRef{
										Name: configmapName,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func newFakeClient(ns string) *kubefake.Clientset {
	s := runtime.NewScheme()
	s.AddKnownTypes(v1alpha1.SchemeGroupVersion, &v1alpha1.ApplicationSet{})
	return kubefake.NewClientset(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "argocd-cm", Namespace: ns, Labels: map[string]string{
		"app.kubernetes.io/part-of": "argocd",
	}}}, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDSecretName,
			Namespace: ns,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: map[string][]byte{
			"server.secretkey": nil,
		},
	})
}
