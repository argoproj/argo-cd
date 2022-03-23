package utils

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/argoproj/argo-cd/v2/common"
	argov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/pkg/apis/applicationset/v1alpha1"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/applicationset/v1alpha1"
	argosettings "github.com/argoproj/argo-cd/v2/util/settings"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

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
			effectedAppSets:    []string{"git-github"},
			expectedStatusCode: http.StatusOK,
			expectedRefresh:    true,
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
			effectedAppSets:    []string{"pull-request-github"},
			expectedStatusCode: http.StatusOK,
			expectedRefresh:    true,
		},
		{
			desc:               "WebHook from a GitHub repository via pull_reqeuest assigned event",
			headerKey:          "X-GitHub-Event",
			headerValue:        "pull_request",
			payloadFile:        "github-pull-request-assigned-event.json",
			effectedAppSets:    []string{"pull-request-github"},
			expectedStatusCode: http.StatusOK,
			expectedRefresh:    false,
		},
	}

	namespace := "test"
	fakeClient := newFakeClient(namespace)
	scheme := runtime.NewScheme()
	err := argoprojiov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)
	err = argov1alpha1.AddToScheme(scheme)
	assert.Nil(t, err)

	for _, test := range tt {
		t.Run(test.desc, func(t *testing.T) {
			fc := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
				fakeAppWithGitGenerator("git-github", namespace, "https://github.com/org/repo"),
				fakeAppWithGitGenerator("git-gitlab", namespace, "https://gitlab/group/name"),
				fakeAppWithPullRequestGenerator("pull-request-github", namespace, "Codertocat", "Hello-World"),
			).Build()
			set := argosettings.NewSettingsManager(context.TODO(), fakeClient, namespace)
			h, err := NewWebhookHandler(namespace, set, fc)
			assert.Nil(t, err)

			req := httptest.NewRequest("POST", "/api/webhook", nil)
			req.Header.Set(test.headerKey, test.headerValue)
			eventJSON, err := ioutil.ReadFile(filepath.Join("testdata", test.payloadFile))
			assert.NoError(t, err)
			req.Body = ioutil.NopCloser(bytes.NewReader(eventJSON))
			w := httptest.NewRecorder()

			h.Handler(w, req)
			assert.Equal(t, w.Code, test.expectedStatusCode)

			list := &argoprojiov1alpha1.ApplicationSetList{}
			err = fc.List(context.TODO(), list)
			assert.Nil(t, err)
			for i := range list.Items {
				gotAppSet := &list.Items[i]
				for _, appSetName := range test.effectedAppSets {
					if appSetName == gotAppSet.Name {
						if expected, got := test.expectedRefresh, gotAppSet.RefreshRequired(); expected != got {
							t.Errorf("unexpected RefreshRequired() expect: %v got: %v", expected, got)
						}
					} else {
						assert.False(t, gotAppSet.RefreshRequired())
					}
				}
			}
		})
	}
}

func TestGenRevisionHasChanged(t *testing.T) {
	assert.True(t, genRevisionHasChanged(&v1alpha1.GitGenerator{}, "master", true))
	assert.False(t, genRevisionHasChanged(&v1alpha1.GitGenerator{}, "master", false))

	assert.True(t, genRevisionHasChanged(&v1alpha1.GitGenerator{Revision: "dev"}, "dev", true))
	assert.False(t, genRevisionHasChanged(&v1alpha1.GitGenerator{Revision: "dev"}, "master", false))

	assert.True(t, genRevisionHasChanged(&v1alpha1.GitGenerator{Revision: "refs/heads/dev"}, "dev", true))
	assert.False(t, genRevisionHasChanged(&v1alpha1.GitGenerator{Revision: "refs/heads/dev"}, "master", false))
}

func fakeAppWithGitGenerator(name, namespace, repo string) *argoprojiov1alpha1.ApplicationSet {
	return &argoprojiov1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: argoprojiov1alpha1.ApplicationSetSpec{
			Generators: []argoprojiov1alpha1.ApplicationSetGenerator{
				{
					Git: &argoprojiov1alpha1.GitGenerator{
						RepoURL: repo,
					},
				},
			},
		},
	}
}

func fakeAppWithPullRequestGenerator(name, namespace, owner, repo string) *argoprojiov1alpha1.ApplicationSet {
	return &argoprojiov1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: argoprojiov1alpha1.ApplicationSetSpec{
			Generators: []argoprojiov1alpha1.ApplicationSetGenerator{
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
	}
}

func newFakeClient(ns string) *kubefake.Clientset {
	s := runtime.NewScheme()
	s.AddKnownTypes(argoprojiov1alpha1.GroupVersion, &argoprojiov1alpha1.ApplicationSet{})
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
