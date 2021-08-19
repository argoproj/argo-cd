package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/repository"

	"github.com/argoproj/argo-cd/v2/util/db"

	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/util/assets"
	"github.com/argoproj/argo-cd/v2/util/rbac"
	"github.com/argoproj/argo-cd/v2/util/settings"

	"github.com/argoproj/argo-cd/v2/reposerver/apiclient/mocks"

	"github.com/stretchr/testify/assert"

	"github.com/dgrijalva/jwt-go/v4"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const testNamespace = "default"

func Test_createRBACObject(t *testing.T) {
	object := createRBACObject("test-prj", "test-repo")
	assert.Equal(t, "test-prj/test-repo", object)
	objectWithoutPrj := createRBACObject("", "test-repo")
	assert.Equal(t, "test-repo", objectWithoutPrj)
}

func TestRepositoryServer(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Namespace: testNamespace,
			Name:      "argocd-cm",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
	}, &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      "argocd-secret",
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"admin.password":   []byte("test"),
			"server.secretkey": []byte("test"),
		},
	})
	settingsMgr := settings.NewSettingsManager(context.Background(), kubeclientset, testNamespace)
	enforcer := newEnforcer(kubeclientset)

	argoDB := db.NewDB("default", settingsMgr, kubeclientset)

	t.Run("Test_getRepo", func(t *testing.T) {
		repoServerClient := mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: &repoServerClient}

		s := NewServer(&repoServerClientset, argoDB, enforcer, nil, settingsMgr)
		url := "https://test"
		repo, _ := s.getRepo(context.TODO(), url)
		assert.Equal(t, repo.Repo, url)
	})

	t.Run("Test_validateAccess", func(t *testing.T) {
		repoServerClient := mocks.RepoServerServiceClient{}
		repoServerClient.On("TestRepository", mock.Anything, mock.Anything).Return(&apiclient.TestRepositoryResponse{}, nil)
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: &repoServerClient}

		s := NewServer(&repoServerClientset, argoDB, enforcer, nil, settingsMgr)
		url := "https://test"
		_, err := s.ValidateAccess(context.TODO(), &repository.RepoAccessQuery{
			Repo: url,
		})
		assert.Nil(t, err)
	})
}

func newEnforcer(kubeclientset *fake.Clientset) *rbac.Enforcer {
	enforcer := rbac.NewEnforcer(kubeclientset, testNamespace, common.ArgoCDRBACConfigMapName, nil)
	_ = enforcer.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
	enforcer.SetDefaultRole("role:admin")
	enforcer.SetClaimsEnforcerFunc(func(claims jwt.Claims, rvals ...interface{}) bool {
		return true
	})
	return enforcer
}
