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
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/argoproj/argo-cd/v2/applicationset/generators"
	"github.com/argoproj/argo-cd/v2/applicationset/services/scm_provider"
	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argosettings "github.com/argoproj/argo-cd/v2/util/settings"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

type generatorMock struct {
	mock.Mock
}

func (g *generatorMock) GetTemplate(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) *argoprojiov1alpha1.ApplicationSetTemplate {
	return &argoprojiov1alpha1.ApplicationSetTemplate{}
}

func (g *generatorMock) GenerateParams(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, _ *argoprojiov1alpha1.ApplicationSet) ([]map[string]interface{}, error) {
	return []map[string]interface{}{}, nil
}

func (g *generatorMock) GetRequeueAfter(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) time.Duration {
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
			effectedAppSets:    []string{"git-github", "matrix-git-github", "merge-git-github", "matrix-scm-git-github", "matrix-nested-git-github", "merge-nested-git-github"},
			expectedStatusCode: http.StatusOK,
			expectedRefresh:    true,
		},
		{
			desc:               "WebHook from a GitHub repository via Commit to branch",
			headerKey:          "X-GitHub-Event",
			headerValue:        "push",
			payloadFile:        "github-commit-branch-event.json",
			effectedAppSets:    []string{"git-github"},
			expectedStatusCode: http.StatusOK,
			expectedRefresh:    true,
		},
		{
			desc:               "WebHook from a GitHub ping event",
			headerKey:          "X-GitHub-Event",
			headerValue:        "ping",
			payloadFile:        "github-ping-event.json",
			effectedAppSets:    []string{"git-github"},
			expectedStatusCode: http.StatusOK,
			expectedRefresh:    false,
		},
		{
			desc:               "WebHook from a GitLab repository via Commit",
			headerKey:          "X-Gitlab-Event",
			headerValue:        "Push Hook",
			payloadFile:        "gitlab-event.json",
			effectedAppSets:    []string{"git-gitlab"},
			expectedStatusCode: http.StatusOK,
			expectedRefresh:    true,
		},
		{
			desc:               "WebHook with an unknown event",
			headerKey:          "X-Random-Event",
			headerValue:        "Push Hook",
			payloadFile:        "gitlab-event.json",
			effectedAppSets:    []string{"git-gitlab"},
			expectedStatusCode: http.StatusBadRequest,
			expectedRefresh:    false,
		},
		{
			desc:               "WebHook with an invalid event",
			headerKey:          "X-Random-Event",
			headerValue:        "Push Hook",
			payloadFile:        "invalid-event.json",
			effectedAppSets:    []string{"git-gitlab"},
			expectedStatusCode: http.StatusBadRequest,
			expectedRefresh:    false,
		},
		{
			desc:               "WebHook from a GitHub repository via pull_reqeuest opened event",
			headerKey:          "X-GitHub-Event",
			headerValue:        "pull_request",
			payloadFile:        "github-pull-request-opened-event.json",
			effectedAppSets:    []string{"pull-request-github", "matrix-pull-request-github", "matrix-scm-pull-request-github", "merge-pull-request-github"},
			expectedStatusCode: http.StatusOK,
			expectedRefresh:    true,
		},
		{
			desc:               "WebHook from a GitHub repository via pull_reqeuest assigned event",
			headerKey:          "X-GitHub-Event",
			headerValue:        "pull_request",
			payloadFile:        "github-pull-request-assigned-event.json",
			effectedAppSets:    []string{"pull-request-github", "matrix-pull-request-github", "matrix-scm-pull-request-github", "merge-pull-request-github"},
			expectedStatusCode: http.StatusOK,
			expectedRefresh:    false,
		},
		{
			desc:               "WebHook from a GitLab repository via open merge request event",
			headerKey:          "X-Gitlab-Event",
			headerValue:        "Merge Request Hook",
			payloadFile:        "gitlab-merge-request-open-event.json",
			effectedAppSets:    []string{"pull-request-gitlab"},
			expectedStatusCode: http.StatusOK,
			expectedRefresh:    true,
		},
		{
			desc:               "WebHook from a GitLab repository via approval merge request event",
			headerKey:          "X-Gitlab-Event",
			headerValue:        "Merge Request Hook",
			payloadFile:        "gitlab-merge-request-approval-event.json",
			effectedAppSets:    []string{"pull-request-gitlab"},
			expectedStatusCode: http.StatusOK,
			expectedRefresh:    false,
		},
	}

	namespace := "test"
	fakeClient := newFakeClient(namespace)
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)
	err = v1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	for _, test := range tt {
		t.Run(test.desc, func(t *testing.T) {
			fc := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
				fakeAppWithGitGenerator("git-github", namespace, "https://github.com/org/repo"),
				fakeAppWithGitGenerator("git-gitlab", namespace, "https://gitlab/group/name"),
				fakeAppWithGithubPullRequestGenerator("pull-request-github", namespace, "Codertocat", "Hello-World"),
				fakeAppWithGitlabPullRequestGenerator("pull-request-gitlab", namespace, "100500"),
				fakeAppWithMatrixAndGitGenerator("matrix-git-github", namespace, "https://github.com/org/repo"),
				fakeAppWithMatrixAndPullRequestGenerator("matrix-pull-request-github", namespace, "Codertocat", "Hello-World"),
				fakeAppWithMatrixAndScmWithGitGenerator("matrix-scm-git-github", namespace, "org"),
				fakeAppWithMatrixAndScmWithPullRequestGenerator("matrix-scm-pull-request-github", namespace, "Codertocat"),
				fakeAppWithMatrixAndNestedGitGenerator("matrix-nested-git-github", namespace, "https://github.com/org/repo"),
				fakeAppWithMergeAndGitGenerator("merge-git-github", namespace, "https://github.com/org/repo"),
				fakeAppWithMergeAndPullRequestGenerator("merge-pull-request-github", namespace, "Codertocat", "Hello-World"),
				fakeAppWithMergeAndNestedGitGenerator("merge-nested-git-github", namespace, "https://github.com/org/repo"),
			).Build()
			set := argosettings.NewSettingsManager(context.TODO(), fakeClient, namespace)
			h, err := NewWebhookHandler(namespace, set, fc, mockGenerators())
			assert.Nil(t, err)

			req := httptest.NewRequest("POST", "/api/webhook", nil)
			req.Header.Set(test.headerKey, test.headerValue)
			eventJSON, err := os.ReadFile(filepath.Join("testdata", test.payloadFile))
			assert.NoError(t, err)
			req.Body = io.NopCloser(bytes.NewReader(eventJSON))
			w := httptest.NewRecorder()

			h.Handler(w, req)
			assert.Equal(t, w.Code, test.expectedStatusCode)

			list := &v1alpha1.ApplicationSetList{}
			err = fc.List(context.TODO(), list)
			assert.Nil(t, err)
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
	}

	nestedGenerators := map[string]generators.Generator{
		"List":        terminalMockGenerators["List"],
		"Git":         terminalMockGenerators["Git"],
		"SCMProvider": terminalMockGenerators["SCMProvider"],
		"PullRequest": terminalMockGenerators["PullRequest"],
		"Matrix":      generators.NewMatrixGenerator(terminalMockGenerators),
		"Merge":       generators.NewMergeGenerator(terminalMockGenerators),
	}

	return map[string]generators.Generator{
		"List":        terminalMockGenerators["List"],
		"Git":         terminalMockGenerators["Git"],
		"SCMProvider": terminalMockGenerators["SCMProvider"],
		"PullRequest": terminalMockGenerators["PullRequest"],
		"Matrix":      generators.NewMatrixGenerator(nestedGenerators),
		"Merge":       generators.NewMergeGenerator(nestedGenerators),
	}
}

func TestGenRevisionHasChanged(t *testing.T) {
	assert.True(t, genRevisionHasChanged(&argoprojiov1alpha1.GitGenerator{}, "master", true))
	assert.False(t, genRevisionHasChanged(&argoprojiov1alpha1.GitGenerator{}, "master", false))

	assert.True(t, genRevisionHasChanged(&argoprojiov1alpha1.GitGenerator{Revision: "dev"}, "dev", true))
	assert.False(t, genRevisionHasChanged(&argoprojiov1alpha1.GitGenerator{Revision: "dev"}, "master", false))

	assert.True(t, genRevisionHasChanged(&argoprojiov1alpha1.GitGenerator{Revision: "refs/heads/dev"}, "dev", true))
	assert.False(t, genRevisionHasChanged(&argoprojiov1alpha1.GitGenerator{Revision: "refs/heads/dev"}, "master", false))
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

func fakeAppWithGitlabPullRequestGenerator(name, namespace, projectId string) *argoprojiov1alpha1.ApplicationSet {
	return &argoprojiov1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: argoprojiov1alpha1.ApplicationSetSpec{
			Generators: []argoprojiov1alpha1.ApplicationSetGenerator{
				{
					PullRequest: &argoprojiov1alpha1.PullRequestGenerator{
						GitLab: &argoprojiov1alpha1.PullRequestGeneratorGitLab{
							Project: projectId,
						},
					},
				},
			},
		},
	}
}

func fakeAppWithGithubPullRequestGenerator(name, namespace, owner, repo string) *argoprojiov1alpha1.ApplicationSet {
	return &argoprojiov1alpha1.ApplicationSet{
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
								List: &argoprojiov1alpha1.ListGenerator{},
							},
							{
								Git: &argoprojiov1alpha1.GitGenerator{
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
								List: &argoprojiov1alpha1.ListGenerator{},
							},
							{
								PullRequest: &argoprojiov1alpha1.PullRequestGenerator{
									Github: &argoprojiov1alpha1.PullRequestGeneratorGithub{
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

func fakeAppWithMatrixAndScmWithGitGenerator(name, namespace, owner string) *argoprojiov1alpha1.ApplicationSet {
	return &argoprojiov1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: argoprojiov1alpha1.ApplicationSetSpec{
			Generators: []argoprojiov1alpha1.ApplicationSetGenerator{
				{
					Matrix: &argoprojiov1alpha1.MatrixGenerator{
						Generators: []argoprojiov1alpha1.ApplicationSetNestedGenerator{
							{
								SCMProvider: &argoprojiov1alpha1.SCMProviderGenerator{
									CloneProtocol: "ssh",
									Github: &argoprojiov1alpha1.SCMProviderGeneratorGithub{
										Organization: owner,
									},
								},
							},
							{
								Git: &argoprojiov1alpha1.GitGenerator{
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

func fakeAppWithMatrixAndScmWithPullRequestGenerator(name, namespace, owner string) *argoprojiov1alpha1.ApplicationSet {
	return &argoprojiov1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: argoprojiov1alpha1.ApplicationSetSpec{
			Generators: []argoprojiov1alpha1.ApplicationSetGenerator{
				{
					Matrix: &argoprojiov1alpha1.MatrixGenerator{
						Generators: []argoprojiov1alpha1.ApplicationSetNestedGenerator{
							{
								SCMProvider: &argoprojiov1alpha1.SCMProviderGenerator{
									CloneProtocol: "https",
									Github: &argoprojiov1alpha1.SCMProviderGeneratorGithub{
										Organization: owner,
									},
								},
							},
							{
								PullRequest: &argoprojiov1alpha1.PullRequestGenerator{
									Github: &argoprojiov1alpha1.PullRequestGeneratorGithub{
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

func fakeAppWithMatrixAndNestedGitGenerator(name, namespace, repo string) *argoprojiov1alpha1.ApplicationSet {
	return &argoprojiov1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: argoprojiov1alpha1.ApplicationSetSpec{
			Generators: []argoprojiov1alpha1.ApplicationSetGenerator{
				{
					Matrix: &argoprojiov1alpha1.MatrixGenerator{
						Generators: []argoprojiov1alpha1.ApplicationSetNestedGenerator{
							{
								List: &argoprojiov1alpha1.ListGenerator{},
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

func fakeAppWithMergeAndGitGenerator(name, namespace, repo string) *argoprojiov1alpha1.ApplicationSet {
	return &argoprojiov1alpha1.ApplicationSet{
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

func fakeAppWithMergeAndNestedGitGenerator(name, namespace, repo string) *argoprojiov1alpha1.ApplicationSet {
	return &argoprojiov1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: argoprojiov1alpha1.ApplicationSetSpec{
			Generators: []argoprojiov1alpha1.ApplicationSetGenerator{
				{
					Merge: &argoprojiov1alpha1.MergeGenerator{
						MergeKeys: []string{
							"server",
						},
						Generators: []argoprojiov1alpha1.ApplicationSetNestedGenerator{
							{
								List: &argoprojiov1alpha1.ListGenerator{},
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

func newFakeClient(ns string) *kubefake.Clientset {
	s := runtime.NewScheme()
	s.AddKnownTypes(v1alpha1.SchemeGroupVersion, &v1alpha1.ApplicationSet{})
	return kubefake.NewSimpleClientset(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "argocd-cm", Namespace: ns, Labels: map[string]string{
		"app.kubernetes.io/part-of": "argocd",
	}}}, &v1.Secret{
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
