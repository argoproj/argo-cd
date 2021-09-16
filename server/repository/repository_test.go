package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

	"github.com/argoproj/argo-cd/v2/server/cache"

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

	cacheutil "github.com/argoproj/argo-cd/v2/util/cache"
	appstatecache "github.com/argoproj/argo-cd/v2/util/cache/appstate"
	dbmocks "github.com/argoproj/argo-cd/v2/util/db/mocks"
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

	t.Run("Test_Get", func(t *testing.T) {
		repoServerClient := mocks.RepoServerServiceClient{}
		repoServerClient.On("TestRepository", mock.Anything, mock.Anything).Return(&apiclient.TestRepositoryResponse{}, nil)
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: &repoServerClient}

		s := NewServer(&repoServerClientset, argoDB, enforcer, newFixtures().Cache, settingsMgr)
		url := "https://test"
		repo, err := s.Get(context.TODO(), &repository.RepoQuery{
			Repo: url,
		})
		assert.Nil(t, err)
		assert.Equal(t, repo.Repo, url)
	})

	t.Run("Test_GetWithNotExistRepoShouldReturn403", func(t *testing.T) {
		repoServerClient := mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: &repoServerClient}

		db := &dbmocks.ArgoDB{}
		db.On("GetRepository", context.TODO(), "test").Return(nil, errors.New("not found"))

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, settingsMgr)
		repo, err := s.Get(context.TODO(), &repository.RepoQuery{
			Repo: "test",
		})
		assert.Nil(t, repo)
		assert.Equal(t, err.Error(), "rpc error: code = PermissionDenied desc = permission denied")
	})

	t.Run("Test_CreateRepositoryWithoutUpsert", func(t *testing.T) {
		repoServerClient := mocks.RepoServerServiceClient{}
		repoServerClient.On("TestRepository", mock.Anything, mock.Anything).Return(&apiclient.TestRepositoryResponse{}, nil)
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: &repoServerClient}

		db := &dbmocks.ArgoDB{}
		db.On("GetRepository", context.TODO(), "test").Return(nil, errors.New("not found"))
		db.On("CreateRepository", context.TODO(), mock.Anything).Return(&apiclient.TestRepositoryResponse{}).Return(&v1alpha1.Repository{
			Repo:    "repo",
			Project: "proj",
		}, nil)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, settingsMgr)
		repo, err := s.CreateRepository(context.TODO(), &repository.RepoCreateRequest{
			Repo: &v1alpha1.Repository{
				Repo:     "test",
				Username: "test",
			},
		})
		assert.Nil(t, err)
		assert.Equal(t, repo.Repo, "repo")
	})

	t.Run("Test_CreateRepositoryWithUpsert", func(t *testing.T) {
		repoServerClient := mocks.RepoServerServiceClient{}
		repoServerClient.On("TestRepository", mock.Anything, mock.Anything).Return(&apiclient.TestRepositoryResponse{}, nil)
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: &repoServerClient}

		db := &dbmocks.ArgoDB{}
		db.On("GetRepository", context.TODO(), "test").Return(&v1alpha1.Repository{
			Repo:     "test",
			Username: "test",
		}, nil)
		db.On("CreateRepository", context.TODO(), mock.Anything).Return(nil, status.Errorf(codes.AlreadyExists, "repository already exists"))
		db.On("UpdateRepository", context.TODO(), mock.Anything).Return(nil, nil)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, settingsMgr)
		repo, err := s.CreateRepository(context.TODO(), &repository.RepoCreateRequest{
			Repo: &v1alpha1.Repository{
				Repo:     "test",
				Username: "test",
			},
			Upsert: true,
		})

		assert.Nil(t, err)
		assert.Equal(t, repo.Repo, "test")
	})

}

type fixtures struct {
	*cache.Cache
}

func newFixtures() *fixtures {
	return &fixtures{cache.NewCache(
		appstatecache.NewCache(
			cacheutil.NewCache(cacheutil.NewInMemoryCache(1*time.Hour)),
			1*time.Minute,
		),
		1*time.Minute,
		1*time.Minute,
		1*time.Minute,
	)}
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
