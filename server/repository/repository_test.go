package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8scache "k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/repository"
	appsv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	fakeapps "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/fake"
	appinformer "github.com/argoproj/argo-cd/v3/pkg/client/informers/externalversions"
	applisters "github.com/argoproj/argo-cd/v3/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient/mocks"
	"github.com/argoproj/argo-cd/v3/server/cache"
	"github.com/argoproj/argo-cd/v3/util/assets"
	cacheutil "github.com/argoproj/argo-cd/v3/util/cache"
	appstatecache "github.com/argoproj/argo-cd/v3/util/cache/appstate"
	"github.com/argoproj/argo-cd/v3/util/db"
	dbmocks "github.com/argoproj/argo-cd/v3/util/db/mocks"
	"github.com/argoproj/argo-cd/v3/util/rbac"
	"github.com/argoproj/argo-cd/v3/util/settings"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application"
)

const testNamespace = "default"

var (
	argocdCM = corev1.ConfigMap{
		Namespace: testNamespace,
		Name:      "argocd-cm",
		Labels: map[string]string{
			"app.kubernetes.io/part-of": "argocd",
		},
	}
	argocdSecret = corev1.Secret{
		Name:      "argocd-secret",
		Namespace: testNamespace,
		Data: map[string][]byte{
			"admin.password":   []byte("test"),
			"server.secretkey": []byte("test"),
		},
	}
	defaultProj = &appsv1.AppProject{
		Kind:       application.AppProjectKind,
		APIVersion: "argoproj.io/v1alpha1",
		Name:       "default",
		Namespace:  testNamespace,
		Spec: appsv1.AppProjectSpec{
			SourceRepos:  []string{"*"},
			Destinations: []appsv1.ApplicationDestination{{Server: "*", Namespace: "*"}},
		},
	}

	defaultProjNoSources = &appsv1.AppProject{
		Kind:       application.AppProjectKind,
		APIVersion: "argoproj.io/v1alpha1",
		Name:       "default",
		Namespace:  testNamespace,
		Spec: appsv1.AppProjectSpec{
			SourceRepos:  []string{},
			Destinations: []appsv1.ApplicationDestination{{Server: "*", Namespace: "*"}},
		},
	}
	fakeRepo = appsv1.Repository{
		Repo:           "https://test",
		Type:           "test",
		Name:           "test",
		Username:       "argo",
		Insecure:       false,
		EnableLFS:      false,
		EnableOCI:      false,
		Proxy:          "test",
		Project:        "argocd",
		InheritedCreds: true,
	}
	guestbookApp = &appsv1.Application{
		Kind:       application.ApplicationKind,
		APIVersion: "argoproj.io/v1alpha1",
		Name:       "guestbook",
		Namespace:  testNamespace,
		Spec: appsv1.ApplicationSpec{
			Project: "default",
			Source: &appsv1.ApplicationSource{
				RepoURL:        "https://test",
				TargetRevision: "HEAD",
				Helm: &appsv1.ApplicationSourceHelm{
					ValueFiles: []string{"values.yaml"},
				},
			},
		},
		Status: appsv1.ApplicationStatus{
			History: appsv1.RevisionHistories{
				{
					Revision: "abcdef123567",
					Source: appsv1.ApplicationSource{
						RepoURL:        "https://test",
						TargetRevision: "HEAD",
						Helm: &appsv1.ApplicationSourceHelm{
							ValueFiles: []string{"values-old.yaml"},
						},
					},
				},
			},
		},
	}
	multiSourceApp001AppName = "msa-two-helm-types"
	multiSourceApp001        = &appsv1.Application{
		Kind:       application.ApplicationKind,
		APIVersion: "argoproj.io/v1alpha1",
		Name:       multiSourceApp001AppName,
		Namespace:  testNamespace,
		Spec: appsv1.ApplicationSpec{
			Project: "default",
			Sources: []appsv1.ApplicationSource{
				{
					RepoURL:        "https://helm.elastic.co",
					TargetRevision: "7.7.0",
					Chart:          "elasticsearch",
					Helm: &appsv1.ApplicationSourceHelm{
						ValueFiles: []string{"values.yaml"},
					},
				},
				{
					RepoURL:        "https://helm.elastic.co",
					TargetRevision: "7.6.0",
					Chart:          "elasticsearch",
					Helm: &appsv1.ApplicationSourceHelm{
						ValueFiles: []string{"values.yaml"},
					},
				},
			},
		},
		Status: appsv1.ApplicationStatus{
			History: appsv1.RevisionHistories{
				{
					ID: 1,
					Revisions: []string{
						"abcdef123567",
					},
					Sources: []appsv1.ApplicationSource{
						{
							RepoURL:        "https://helm.elastic.co",
							TargetRevision: "7.6.0",
							Helm: &appsv1.ApplicationSourceHelm{
								ValueFiles: []string{"values-old.yaml"},
							},
						},
					},
				},
			},
		},
	}
	multiSourceApp002AppName = "msa-one-plugin-one-helm"
	multiSourceApp002        = &appsv1.Application{
		Kind:       application.ApplicationKind,
		APIVersion: "argoproj.io/v1alpha1",
		Name:       multiSourceApp002AppName,
		Namespace:  testNamespace,
		Spec: appsv1.ApplicationSpec{
			Project: "default",
			Sources: []appsv1.ApplicationSource{
				{
					RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
					Path:           "sock-shop",
					TargetRevision: "HEAD",
				},
				{
					RepoURL:        "https://helm.elastic.co",
					TargetRevision: "7.7.0",
					Chart:          "elasticsearch",
					Helm: &appsv1.ApplicationSourceHelm{
						ValueFiles: []string{"values.yaml"},
					},
				},
			},
		},
		Status: appsv1.ApplicationStatus{
			History: appsv1.RevisionHistories{
				{
					Revision: "HEAD",
					Sources: []appsv1.ApplicationSource{
						{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "1.0.0",
						},
					},
				},
			},
		},
	}
)

func newAppAndProjLister(objects ...runtime.Object) (applisters.ApplicationLister, k8scache.SharedIndexInformer) {
	fakeAppsClientset := fakeapps.NewSimpleClientset(objects...)
	factory := appinformer.NewSharedInformerFactoryWithOptions(fakeAppsClientset, 0, appinformer.WithNamespace(""), appinformer.WithTweakListOptions(func(_ *metav1.ListOptions) {}))
	projInformer := factory.Argoproj().V1alpha1().AppProjects()
	appsInformer := factory.Argoproj().V1alpha1().Applications()
	for _, obj := range objects {
		switch obj.(type) {
		case *appsv1.AppProject:
			_ = projInformer.Informer().GetStore().Add(obj)
		case *appsv1.Application:
			_ = appsInformer.Informer().GetStore().Add(obj)
		}
	}
	appLister := appsInformer.Lister()
	return appLister, projInformer.Informer()
}

func Test_createRBACObject(t *testing.T) {
	object := createRBACObject("test-prj", "test-repo")
	assert.Equal(t, "test-prj/test-repo", object)
	objectWithoutPrj := createRBACObject("", "test-repo")
	assert.Equal(t, "test-repo", objectWithoutPrj)
}

func TestRepositoryServer(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset(&argocdCM, &argocdSecret)
	settingsMgr := settings.NewSettingsManager(t.Context(), kubeclientset, testNamespace)
	enforcer := newEnforcer(kubeclientset)
	appLister, projInformer := newAppAndProjLister(defaultProj)
	argoDB := db.NewDB("default", settingsMgr, kubeclientset)

	t.Run("Test_getRepo", func(t *testing.T) {
		repoServerClient := mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: &repoServerClient}

		s := NewServer(&repoServerClientset, argoDB, enforcer, nil, appLister, projInformer, testNamespace, settingsMgr, false)
		url := "https://test"
		repo, _ := s.getRepo(t.Context(), url, "")
		assert.Equal(t, repo.Repo, url)
	})

	t.Run("Test_validateAccess", func(t *testing.T) {
		repoServerClient := &mocks.RepoServerServiceClient{}
		repoServerClient.EXPECT().TestRepository(mock.Anything, mock.Anything).Return(&apiclient.TestRepositoryResponse{}, nil)
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: repoServerClient}

		s := NewServer(&repoServerClientset, argoDB, enforcer, nil, appLister, projInformer, testNamespace, settingsMgr, false)
		url := "https://test"
		_, err := s.ValidateAccess(t.Context(), &repository.RepoAccessQuery{
			Repo: url,
		})
		require.NoError(t, err)
	})

	t.Run("Test_validateWriteAccess", func(t *testing.T) {
		repoServerClient := &mocks.RepoServerServiceClient{}
		repoServerClient.EXPECT().TestRepository(mock.Anything, mock.Anything).Return(&apiclient.TestRepositoryResponse{}, nil)
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: repoServerClient}

		s := NewServer(&repoServerClientset, argoDB, enforcer, nil, appLister, projInformer, testNamespace, settingsMgr, true)
		url := "https://test"
		_, err := s.ValidateWriteAccess(t.Context(), &repository.RepoAccessQuery{
			Repo: url,
		})
		require.NoError(t, err)
	})

	t.Run("Test_Get", func(t *testing.T) {
		repoServerClient := &mocks.RepoServerServiceClient{}
		repoServerClient.EXPECT().TestRepository(mock.Anything, mock.Anything).Return(&apiclient.TestRepositoryResponse{}, nil)
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: repoServerClient}

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.EXPECT().ListRepositories(mock.Anything).Return([]*appsv1.Repository{{Repo: url}}, nil)
		db.EXPECT().GetRepository(mock.Anything, url, "").Return(&appsv1.Repository{Repo: url}, nil)
		db.EXPECT().RepositoryExists(mock.Anything, url, "").Return(true, nil)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projInformer, testNamespace, settingsMgr, false)
		repo, err := s.Get(t.Context(), &repository.RepoQuery{
			Repo: url,
		})
		require.NoError(t, err)
		assert.Equal(t, repo.Repo, url)
	})

	t.Run("Test_GetInherited", func(t *testing.T) {
		repoServerClient := &mocks.RepoServerServiceClient{}
		repoServerClient.EXPECT().TestRepository(mock.Anything, mock.Anything).Return(&apiclient.TestRepositoryResponse{}, nil)
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: repoServerClient}

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		testRepo := &appsv1.Repository{
			Repo:           url,
			Type:           "git",
			Username:       "",
			InheritedCreds: true,
		}
		db.EXPECT().ListRepositories(mock.Anything).Return([]*appsv1.Repository{testRepo}, nil)
		db.EXPECT().GetRepository(mock.Anything, url, "").Return(testRepo, nil)
		db.EXPECT().RepositoryExists(mock.Anything, url, "").Return(true, nil)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projInformer, testNamespace, settingsMgr, false)
		repo, err := s.Get(t.Context(), &repository.RepoQuery{
			Repo: url,
		})
		require.NoError(t, err)

		testRepo.ConnectionState = repo.ConnectionState // overwrite connection state on our test object to simplify comparison below

		assert.Equal(t, testRepo, repo)
	})

	t.Run("Test_GetWithErrorShouldReturn403", func(t *testing.T) {
		repoServerClient := mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: &repoServerClient}

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.EXPECT().ListRepositories(mock.Anything).Return(nil, nil)
		db.EXPECT().GetRepository(mock.Anything, url, "").Return(nil, errors.New("some error"))
		db.EXPECT().RepositoryExists(mock.Anything, url, "").Return(true, nil)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projInformer, testNamespace, settingsMgr, false)
		repo, err := s.Get(t.Context(), &repository.RepoQuery{
			Repo: url,
		})
		assert.Nil(t, repo)
		assert.Equal(t, err, common.PermissionDeniedAPIError)
	})

	t.Run("Test_GetWithNotExistRepoShouldReturn404", func(t *testing.T) {
		repoServerClient := &mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: repoServerClient}
		repoServerClient.EXPECT().TestRepository(mock.Anything, mock.Anything).Return(&apiclient.TestRepositoryResponse{}, nil)

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.EXPECT().ListRepositories(mock.Anything).Return([]*appsv1.Repository{{Repo: url}}, nil)
		db.EXPECT().GetRepository(mock.Anything, url, "").Return(&appsv1.Repository{Repo: url}, nil)
		db.EXPECT().RepositoryExists(mock.Anything, url, "").Return(false, nil)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projInformer, testNamespace, settingsMgr, false)
		repo, err := s.Get(t.Context(), &repository.RepoQuery{
			Repo: url,
		})
		assert.Nil(t, repo)
		assert.EqualError(t, err, "rpc error: code = NotFound desc = repo 'https://test' not found")
	})

	t.Run("Test_GetRepoIsSanitized", func(t *testing.T) {
		repoServerClient := &mocks.RepoServerServiceClient{}
		repoServerClient.EXPECT().TestRepository(mock.Anything, mock.Anything).Return(&apiclient.TestRepositoryResponse{}, nil)
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: repoServerClient}

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.EXPECT().ListRepositories(mock.Anything).Return([]*appsv1.Repository{{Repo: url, Username: "test", Password: "it's a secret", GitHubAppEnterpriseBaseURL: "https://ghe.example.com/api/v3", GithubAppId: 123456, GithubAppInstallationId: 789}}, nil)
		db.EXPECT().GetRepository(mock.Anything, url, "").Return(&appsv1.Repository{Repo: url, Username: "test", Password: "it's a secret"}, nil)
		db.EXPECT().RepositoryExists(mock.Anything, url, "").Return(true, nil)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projInformer, testNamespace, settingsMgr, false)
		repo, err := s.Get(t.Context(), &repository.RepoQuery{
			Repo: url,
		})
		require.NoError(t, err)
		assert.Equal(t, "https://test", repo.Repo)
		assert.Equal(t, "https://ghe.example.com/api/v3", repo.GitHubAppEnterpriseBaseURL)
		assert.Equal(t, int64(123456), repo.GithubAppId)
		assert.Equal(t, int64(789), repo.GithubAppInstallationId)
		assert.Empty(t, repo.Password)
	})

	t.Run("Test_GetRepoIsNormalized", func(t *testing.T) {
		repoServerClient := &mocks.RepoServerServiceClient{}
		repoServerClient.EXPECT().TestRepository(mock.Anything, mock.Anything).Return(&apiclient.TestRepositoryResponse{}, nil)
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: repoServerClient}

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.EXPECT().ListRepositories(mock.Anything).Return([]*appsv1.Repository{{Repo: url}}, nil)
		db.EXPECT().GetRepository(mock.Anything, url, "").Return(&appsv1.Repository{Repo: url, Username: "test"}, nil)
		db.EXPECT().RepositoryExists(mock.Anything, url, "").Return(true, nil)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projInformer, testNamespace, settingsMgr, false)
		repo, err := s.Get(t.Context(), &repository.RepoQuery{
			Repo: url,
		})
		require.NoError(t, err)
		assert.Equal(t, "https://test", repo.Repo)
		assert.Equal(t, common.DefaultRepoType, repo.Type)
	})

	t.Run("Test_GetRepoHasConnectionState", func(t *testing.T) {
		repoServerClient := &mocks.RepoServerServiceClient{}
		repoServerClient.EXPECT().TestRepository(mock.Anything, mock.Anything).Return(&apiclient.TestRepositoryResponse{
			VerifiedRepository: true,
		}, nil)
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: repoServerClient}

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.EXPECT().ListRepositories(mock.Anything).Return([]*appsv1.Repository{{Repo: url}}, nil)
		db.EXPECT().GetRepository(mock.Anything, url, "").Return(&appsv1.Repository{Repo: url}, nil)
		db.EXPECT().RepositoryExists(mock.Anything, url, "").Return(true, nil)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projInformer, testNamespace, settingsMgr, false)
		repo, err := s.Get(t.Context(), &repository.RepoQuery{
			Repo: url,
		})
		require.NoError(t, err)
		require.NotNil(t, repo.ConnectionState)
		assert.Equal(t, appsv1.ConnectionStatusSuccessful, repo.ConnectionState.Status)
	})

	t.Run("Test_CreateRepositoryWithoutUpsert", func(t *testing.T) {
		repoServerClient := &mocks.RepoServerServiceClient{}
		repoServerClient.EXPECT().TestRepository(mock.Anything, mock.Anything).Return(&apiclient.TestRepositoryResponse{}, nil)
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: repoServerClient}

		db := &dbmocks.ArgoDB{}
		db.EXPECT().GetRepository(mock.Anything, "test", mock.Anything).Return(nil, errors.New("not found"))
		db.EXPECT().CreateRepository(mock.Anything, mock.Anything).Return(&appsv1.Repository{
			Repo:    "repo",
			Project: "proj",
		}, nil)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projInformer, testNamespace, settingsMgr, false)
		repo, err := s.CreateRepository(t.Context(), &repository.RepoCreateRequest{
			Repo: &appsv1.Repository{
				Repo:     "test",
				Username: "test",
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "repo", repo.Repo)
	})

	t.Run("Test_CreateRepositoryWithUpsert", func(t *testing.T) {
		repoServerClient := &mocks.RepoServerServiceClient{}
		repoServerClient.EXPECT().TestRepository(mock.Anything, mock.Anything).Return(&apiclient.TestRepositoryResponse{}, nil)
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: repoServerClient}

		r := &appsv1.Repository{
			Repo:     "test",
			Username: "test",
		}

		db := &dbmocks.ArgoDB{}
		db.EXPECT().GetRepository(mock.Anything, "test", "").Return(&appsv1.Repository{
			Repo:     "test",
			Username: "test",
		}, nil)
		db.EXPECT().CreateRepository(mock.Anything, mock.Anything).Return(nil, status.Errorf(codes.AlreadyExists, "repository already exists"))
		db.EXPECT().UpdateRepository(mock.Anything, mock.Anything).Return(r, nil)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projInformer, testNamespace, settingsMgr, false)
		repo, err := s.CreateRepository(t.Context(), &repository.RepoCreateRequest{
			Repo:   r,
			Upsert: true,
		})

		require.NoError(t, err)
		require.NotNil(t, repo)
		assert.Equal(t, "test", repo.Repo)
	})

	// https://github.com/argoproj/argo-cd/security/advisories/GHSA-mfgm-4p7x-3ccr
	t.Run("Test_CreateRepositoryWithUpsertRequiresUpdatePermission", func(t *testing.T) {
		repoServerClient := &mocks.RepoServerServiceClient{}
		repoServerClient.EXPECT().TestRepository(mock.Anything, mock.Anything).Return(&apiclient.TestRepositoryResponse{}, nil)
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: repoServerClient}
		createOnlyEnforcer := newEnforcer(kubeclientset)
		require.NoError(t, createOnlyEnforcer.SetUserPolicy("p, role:createrepos, repositories, create, *, allow"))
		createOnlyEnforcer.SetDefaultRole("role:createrepos")

		db := &dbmocks.ArgoDB{}
		db.EXPECT().GetRepository(mock.Anything, "test", "").Return(&appsv1.Repository{
			Repo:     "test",
			Username: "original",
		}, nil)
		db.EXPECT().CreateRepository(mock.Anything, mock.Anything).Return(nil, status.Errorf(codes.AlreadyExists, "repository already exists"))

		s := NewServer(&repoServerClientset, db, createOnlyEnforcer, newFixtures().Cache, appLister, projInformer, testNamespace, settingsMgr, false)
		_, err := s.CreateRepository(t.Context(), &repository.RepoCreateRequest{
			Repo:   &appsv1.Repository{Repo: "test", Username: "attacker"},
			Upsert: true,
		})

		require.Equal(t, codes.PermissionDenied, status.Code(err))
		db.AssertNotCalled(t, "UpdateRepository", mock.Anything, mock.Anything)
	})

	// https://github.com/argoproj/argo-cd/security/advisories/GHSA-mfgm-4p7x-3ccr
	t.Run("Test_CreateWriteRepositoryWithUpsertRequiresUpdatePermission", func(t *testing.T) {
		repoServerClient := &mocks.RepoServerServiceClient{}
		repoServerClient.EXPECT().TestRepository(mock.Anything, mock.Anything).Return(&apiclient.TestRepositoryResponse{}, nil)
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: repoServerClient}
		createOnlyEnforcer := newEnforcer(kubeclientset)
		require.NoError(t, createOnlyEnforcer.SetUserPolicy("p, role:createrepos, write-repositories, create, *, allow"))
		createOnlyEnforcer.SetDefaultRole("role:createrepos")

		db := &dbmocks.ArgoDB{}
		db.EXPECT().GetWriteRepository(mock.Anything, "test", "").Return(&appsv1.Repository{
			Repo:     "test",
			Username: "original",
		}, nil)
		db.EXPECT().CreateWriteRepository(mock.Anything, mock.Anything).Return(nil, status.Errorf(codes.AlreadyExists, "repository already exists"))

		s := NewServer(&repoServerClientset, db, createOnlyEnforcer, newFixtures().Cache, appLister, projInformer, testNamespace, settingsMgr, true)
		_, err := s.CreateWriteRepository(t.Context(), &repository.RepoCreateRequest{
			Repo:   &appsv1.Repository{Repo: "test", Username: "attacker", Password: "secret"},
			Upsert: true,
		})

		require.Equal(t, codes.PermissionDenied, status.Code(err))
		db.AssertNotCalled(t, "UpdateWriteRepository", mock.Anything, mock.Anything)
	})

	t.Run("Test_ListRepositories", func(t *testing.T) {
		repoServerClient := &mocks.RepoServerServiceClient{}
		repoServerClient.EXPECT().TestRepository(mock.Anything, mock.Anything).Return(&apiclient.TestRepositoryResponse{}, nil)
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: repoServerClient}
		enforcer := newEnforcer(kubeclientset)

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.EXPECT().GetRepository(mock.Anything, url, "argocd").Return(nil, nil)
		db.EXPECT().ListHelmRepositories(mock.Anything).Return(nil, nil)
		db.EXPECT().ListRepositories(mock.Anything).Return([]*appsv1.Repository{&fakeRepo, &fakeRepo}, nil)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projInformer, testNamespace, settingsMgr, false)
		resp, err := s.ListRepositories(t.Context(), &repository.RepoQuery{})
		require.NoError(t, err)
		assert.Len(t, resp.Items, 2)
	})
}

func TestRepositoryServerListApps(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset(&argocdCM, &argocdSecret)
	settingsMgr := settings.NewSettingsManager(t.Context(), kubeclientset, testNamespace)

	t.Run("Test_WithoutAppCreateUpdatePrivileges", func(t *testing.T) {
		repoServerClient := mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: &repoServerClient}
		enforcer := newEnforcer(kubeclientset)
		enforcer.SetDefaultRole("role:readonly")

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.EXPECT().GetRepository(mock.Anything, url, "default").Return(&appsv1.Repository{Repo: url}, nil)
		appLister, projLister := newAppAndProjLister(defaultProj)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, testNamespace, settingsMgr, false)
		resp, err := s.ListApps(t.Context(), &repository.RepoAppsQuery{
			Repo:       "https://test",
			Revision:   "HEAD",
			AppName:    "foo",
			AppProject: "default",
		})
		assert.Nil(t, resp)
		assert.Equal(t, err, common.PermissionDeniedAPIError)
	})

	t.Run("Test_WithAppCreateUpdatePrivileges", func(t *testing.T) {
		repoServerClient := &mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: repoServerClient}
		enforcer := newEnforcer(kubeclientset)
		enforcer.SetDefaultRole("role:admin")
		appLister, projLister := newAppAndProjLister(defaultProj)

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.EXPECT().GetRepository(mock.Anything, url, "default").Return(&appsv1.Repository{Repo: url}, nil)
		db.EXPECT().GetProjectRepositories("default").Return(nil, nil)
		db.EXPECT().GetProjectClusters(mock.Anything, "default").Return(nil, nil)
		repoServerClient.EXPECT().ListApps(mock.Anything, mock.Anything).Return(&apiclient.AppList{
			Apps: map[string]string{
				"path/to/dir": "Kustomize",
			},
		}, nil)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, testNamespace, settingsMgr, false)
		resp, err := s.ListApps(t.Context(), &repository.RepoAppsQuery{
			Repo:       "https://test",
			Revision:   "HEAD",
			AppName:    "foo",
			AppProject: "default",
		})
		require.NoError(t, err)
		require.Len(t, resp.Items, 1)
		assert.Equal(t, "path/to/dir", resp.Items[0].Path)
		assert.Equal(t, "Kustomize", resp.Items[0].Type)
	})

	t.Run("Test_WithAppCreateUpdatePrivilegesRepoNotAllowed", func(t *testing.T) {
		repoServerClient := &mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: repoServerClient}
		enforcer := newEnforcer(kubeclientset)
		enforcer.SetDefaultRole("role:admin")
		appLister, projLister := newAppAndProjLister(defaultProjNoSources)

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.EXPECT().GetRepository(mock.Anything, url, "default").Return(&appsv1.Repository{Repo: url}, nil)
		db.EXPECT().GetProjectRepositories("default").Return(nil, nil)
		db.EXPECT().GetProjectClusters(mock.Anything, "default").Return(nil, nil)
		repoServerClient.EXPECT().ListApps(mock.Anything, mock.Anything).Return(&apiclient.AppList{
			Apps: map[string]string{
				"path/to/dir": "Kustomize",
			},
		}, nil)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, testNamespace, settingsMgr, false)
		resp, err := s.ListApps(t.Context(), &repository.RepoAppsQuery{
			Repo:       "https://test",
			Revision:   "HEAD",
			AppName:    "foo",
			AppProject: "default",
		})
		assert.Nil(t, resp)
		require.Error(t, err, "repository 'https://test' not permitted in project 'default'")
	})
}

func TestRepositoryServerGetAppDetails(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset(&argocdCM, &argocdSecret)
	settingsMgr := settings.NewSettingsManager(t.Context(), kubeclientset, testNamespace)

	t.Run("Test_WithoutRepoReadPrivileges", func(t *testing.T) {
		repoServerClient := mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: &repoServerClient}
		enforcer := newEnforcer(kubeclientset)
		enforcer.SetDefaultRole("")

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.EXPECT().GetRepository(mock.Anything, url, "default").Return(&appsv1.Repository{Repo: url}, nil)
		appLister, projLister := newAppAndProjLister(defaultProj)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, testNamespace, settingsMgr, false)
		resp, err := s.GetAppDetails(t.Context(), &repository.RepoAppDetailsQuery{
			Source: &appsv1.ApplicationSource{
				RepoURL: url,
			},
			AppName:    "newapp",
			AppProject: "default",
		})
		assert.Nil(t, resp)
		require.Error(t, err, "rpc error: code = PermissionDenied desc = permission denied: repositories, get, https://test")
	})
	t.Run("Test_WithoutAppReadPrivileges", func(t *testing.T) {
		repoServerClient := mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: &repoServerClient}
		enforcer := newEnforcer(kubeclientset)
		_ = enforcer.SetUserPolicy("p, role:readrepos, repositories, get, *, allow")
		enforcer.SetDefaultRole("role:readrepos")

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.EXPECT().GetRepository(mock.Anything, url, "default").Return(&appsv1.Repository{Repo: url}, nil)
		appLister, projLister := newAppAndProjLister(defaultProj)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, testNamespace, settingsMgr, false)
		resp, err := s.GetAppDetails(t.Context(), &repository.RepoAppDetailsQuery{
			Source: &appsv1.ApplicationSource{
				RepoURL: url,
			},
			AppName:    "newapp",
			AppProject: "default",
		})
		assert.Nil(t, resp)
		require.Error(t, err, "rpc error: code = PermissionDenied desc = permission denied: applications, get, default/newapp")
	})
	t.Run("Test_WithoutCreatePrivileges", func(t *testing.T) {
		repoServerClient := mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: &repoServerClient}
		enforcer := newEnforcer(kubeclientset)
		enforcer.SetDefaultRole("role:readonly")

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.EXPECT().GetRepository(mock.Anything, url, "default").Return(&appsv1.Repository{Repo: url}, nil)
		appLister, projLister := newAppAndProjLister(defaultProj)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, testNamespace, settingsMgr, false)
		resp, err := s.GetAppDetails(t.Context(), &repository.RepoAppDetailsQuery{
			Source: &appsv1.ApplicationSource{
				RepoURL: url,
			},
			AppName:    "newapp",
			AppProject: "default",
		})
		assert.Nil(t, resp)
		require.Error(t, err, "rpc error: code = PermissionDenied desc = permission denied: applications, create, default/newapp")
	})
	t.Run("Test_WithCreatePrivileges", func(t *testing.T) {
		repoServerClient := &mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: repoServerClient}
		enforcer := newEnforcer(kubeclientset)

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.EXPECT().ListHelmRepositories(mock.Anything).Return(nil, nil)
		db.EXPECT().GetRepository(mock.Anything, url, "default").Return(&appsv1.Repository{Repo: url}, nil)
		db.EXPECT().GetProjectRepositories("default").Return(nil, nil)
		db.EXPECT().GetProjectClusters(mock.Anything, "default").Return(nil, nil)
		expectedResp := apiclient.RepoAppDetailsResponse{Type: "Directory"}
		repoServerClient.EXPECT().GetAppDetails(mock.Anything, mock.Anything).Return(&expectedResp, nil)
		appLister, projLister := newAppAndProjLister(defaultProj)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, testNamespace, settingsMgr, false)
		resp, err := s.GetAppDetails(t.Context(), &repository.RepoAppDetailsQuery{
			Source: &appsv1.ApplicationSource{
				RepoURL: url,
			},
			AppName:    "newapp",
			AppProject: "default",
		})
		require.NoError(t, err)
		assert.Equal(t, expectedResp, *resp)
	})
	t.Run("Test_RepoNotPermitted", func(t *testing.T) {
		repoServerClient := &mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: repoServerClient}
		enforcer := newEnforcer(kubeclientset)

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.EXPECT().GetRepository(mock.Anything, url, "default").Return(&appsv1.Repository{Repo: url}, nil)
		db.EXPECT().GetProjectRepositories("default").Return(nil, nil)
		db.EXPECT().GetProjectClusters(mock.Anything, "default").Return(nil, nil)
		expectedResp := apiclient.RepoAppDetailsResponse{Type: "Directory"}
		repoServerClient.EXPECT().GetAppDetails(mock.Anything, mock.Anything).Return(&expectedResp, nil)
		appLister, projLister := newAppAndProjLister(defaultProjNoSources)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, testNamespace, settingsMgr, false)
		resp, err := s.GetAppDetails(t.Context(), &repository.RepoAppDetailsQuery{
			Source: &appsv1.ApplicationSource{
				RepoURL: url,
			},
			AppName:    "newapp",
			AppProject: "default",
		})
		require.Error(t, err, "repository 'https://test' not permitted in project 'default'")
		assert.Nil(t, resp)
	})
	t.Run("Test_ExistingApp", func(t *testing.T) {
		repoServerClient := &mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: repoServerClient}
		enforcer := newEnforcer(kubeclientset)

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.EXPECT().ListHelmRepositories(mock.Anything).Return(nil, nil)
		db.EXPECT().GetRepository(mock.Anything, url, "default").Return(&appsv1.Repository{Repo: url}, nil)
		db.EXPECT().GetProjectRepositories("default").Return(nil, nil)
		db.EXPECT().GetProjectClusters(mock.Anything, "default").Return(nil, nil)
		expectedResp := apiclient.RepoAppDetailsResponse{Type: "Directory"}
		repoServerClient.EXPECT().GetAppDetails(mock.Anything, mock.Anything).Return(&expectedResp, nil)
		appLister, projLister := newAppAndProjLister(defaultProj, guestbookApp)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, testNamespace, settingsMgr, false)
		resp, err := s.GetAppDetails(t.Context(), &repository.RepoAppDetailsQuery{
			Source:     guestbookApp.Spec.GetSourcePtrByIndex(0),
			AppName:    "guestbook",
			AppProject: "default",
		})
		require.NoError(t, err)
		assert.Equal(t, expectedResp, *resp)
	})
	t.Run("Test_ExistingAppInOtherNamespaceWithReadOnlyPrivileges", func(t *testing.T) {
		repoServerClient := &mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: repoServerClient}
		enforcer := newEnforcer(kubeclientset)
		enforcer.SetDefaultRole("role:readonly")

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.EXPECT().ListHelmRepositories(mock.Anything).Return(nil, nil)
		db.EXPECT().GetRepository(mock.Anything, url, "default").Return(&appsv1.Repository{Repo: url}, nil)
		db.EXPECT().GetProjectRepositories("default").Return(nil, nil)
		db.EXPECT().GetProjectClusters(mock.Anything, "default").Return(nil, nil)
		expectedResp := apiclient.RepoAppDetailsResponse{Type: "Helm"}
		repoServerClient.EXPECT().GetAppDetails(mock.Anything, mock.MatchedBy(func(req *apiclient.RepoServerAppDetailsQuery) bool {
			return req.AppName == "apps_guestbook"
		})).Return(&expectedResp, nil)
		app := guestbookApp.DeepCopy()
		app.Namespace = "apps"
		appLister, projLister := newAppAndProjLister(defaultProj, app)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, testNamespace, settingsMgr, false)
		resp, err := s.GetAppDetails(t.Context(), &repository.RepoAppDetailsQuery{
			Source:     app.Spec.GetSourcePtrByIndex(0),
			AppName:    "apps/guestbook",
			AppProject: "default",
		})
		require.NoError(t, err)
		assert.Equal(t, expectedResp, *resp)
	})
	t.Run("Test_ExistingAppInOtherNamespaceUnqualifiedNameRequiresCreate", func(t *testing.T) {
		// Without the namespace, the app cannot be found and it is treated as an app being created.
		repoServerClient := &mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: repoServerClient}
		enforcer := newEnforcer(kubeclientset)
		enforcer.SetDefaultRole("role:readonly")

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.EXPECT().GetRepository(mock.Anything, url, "default").Return(&appsv1.Repository{Repo: url}, nil)
		app := guestbookApp.DeepCopy()
		app.Namespace = "apps"
		appLister, projLister := newAppAndProjLister(defaultProj, app)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, testNamespace, settingsMgr, false)
		resp, err := s.GetAppDetails(t.Context(), &repository.RepoAppDetailsQuery{
			Source:     app.Spec.GetSourcePtrByIndex(0),
			AppName:    "guestbook",
			AppProject: "default",
		})
		assert.Nil(t, resp)
		require.ErrorContains(t, err, "permission denied: applications, create, default/guestbook")
	})
	t.Run("Test_ExistingAppWithEmptyProjectUsesDefaultProject", func(t *testing.T) {
		// An app without spec.project belongs to the "default" project, so the RBAC object and project check must use it.
		repoServerClient := &mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: repoServerClient}
		enforcer := newEnforcer(kubeclientset)
		require.NoError(t, enforcer.SetUserPolicy("p, role:guestbook, repositories, get, *, allow\np, role:guestbook, applications, get, default/guestbook, allow"))
		enforcer.SetDefaultRole("role:guestbook")

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.EXPECT().ListHelmRepositories(mock.Anything).Return(nil, nil)
		db.EXPECT().GetRepository(mock.Anything, url, "default").Return(&appsv1.Repository{Repo: url}, nil)
		db.EXPECT().GetProjectRepositories("default").Return(nil, nil)
		db.EXPECT().GetProjectClusters(mock.Anything, "default").Return(nil, nil)
		expectedResp := apiclient.RepoAppDetailsResponse{Type: "Helm"}
		repoServerClient.EXPECT().GetAppDetails(mock.Anything, mock.Anything).Return(&expectedResp, nil)
		app := guestbookApp.DeepCopy()
		app.Spec.Project = ""
		appLister, projLister := newAppAndProjLister(defaultProj, app)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, testNamespace, settingsMgr, false)
		resp, err := s.GetAppDetails(t.Context(), &repository.RepoAppDetailsQuery{
			Source:     app.Spec.GetSourcePtrByIndex(0),
			AppName:    "guestbook",
			AppProject: "default",
		})
		require.NoError(t, err)
		assert.Equal(t, expectedResp, *resp)
	})
	t.Run("Test_ExistingAppInOtherProjectIsNotRevealed", func(t *testing.T) {
		url := "https://test"
		app := guestbookApp.DeepCopy()
		app.Spec.Project = "secret-project"
		getAppDetails := func(t *testing.T, policy string, appName string) error {
			t.Helper()
			repoServerClientset := mocks.Clientset{RepoServerServiceClient: &mocks.RepoServerServiceClient{}}
			enforcer := newEnforcer(kubeclientset)
			require.NoError(t, enforcer.SetUserPolicy(policy))
			enforcer.SetDefaultRole("role:test")
			db := &dbmocks.ArgoDB{}
			db.EXPECT().GetRepository(mock.Anything, url, "default").Return(&appsv1.Repository{Repo: url}, nil)
			appLister, projLister := newAppAndProjLister(defaultProj, app)

			s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, testNamespace, settingsMgr, false)
			resp, err := s.GetAppDetails(t.Context(), &repository.RepoAppDetailsQuery{
				Source:     app.Spec.GetSourcePtrByIndex(0),
				AppName:    appName,
				AppProject: "default",
			})
			assert.Nil(t, resp)
			require.Error(t, err)
			return err
		}

		t.Run("without get privileges", func(t *testing.T) {
			policy := "p, role:test, repositories, get, *, allow"
			existingErr := getAppDetails(t, policy, "guestbook")
			missingErr := getAppDetails(t, policy, "missing")
			assert.Contains(t, existingErr.Error(), "permission denied: applications, get, default/guestbook")
			assert.NotContains(t, existingErr.Error(), "secret-project")
			// The response for an existing app must be the same as for an app that does not exist.
			assert.Equal(t, strings.ReplaceAll(missingErr.Error(), "missing", "guestbook"), existingErr.Error())
		})
		t.Run("with get privileges in the requested project only", func(t *testing.T) {
			err := getAppDetails(t, "p, role:test, repositories, get, *, allow\np, role:test, applications, get, default/guestbook, allow", "guestbook")
			assert.Equal(t, common.PermissionDeniedAPIError, err)
			assert.NotContains(t, err.Error(), "secret-project")
		})
	})
	t.Run("Test_QualifiedAppNameUsesNormalizedRBACObject", func(t *testing.T) {
		tests := []struct {
			name         string
			namespace    string
			policy       string
			instanceName string
		}{
			{
				name:         "app in control plane namespace",
				namespace:    testNamespace,
				policy:       "p, role:guestbook, applications, get, default/guestbook, allow",
				instanceName: "guestbook",
			},
			{
				name:         "app in other namespace",
				namespace:    "apps",
				policy:       "p, role:guestbook, applications, get, default/apps/guestbook, allow",
				instanceName: "apps_guestbook",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				repoServerClient := &mocks.RepoServerServiceClient{}
				repoServerClientset := mocks.Clientset{RepoServerServiceClient: repoServerClient}
				enforcer := newEnforcer(kubeclientset)
				require.NoError(t, enforcer.SetUserPolicy("p, role:guestbook, repositories, get, *, allow\n"+tt.policy))
				enforcer.SetDefaultRole("role:guestbook")

				url := "https://test"
				db := &dbmocks.ArgoDB{}
				db.EXPECT().ListHelmRepositories(mock.Anything).Return(nil, nil)
				db.EXPECT().GetRepository(mock.Anything, url, "default").Return(&appsv1.Repository{Repo: url}, nil)
				db.EXPECT().GetProjectRepositories("default").Return(nil, nil)
				db.EXPECT().GetProjectClusters(mock.Anything, "default").Return(nil, nil)
				expectedResp := apiclient.RepoAppDetailsResponse{Type: "Helm"}
				repoServerClient.EXPECT().GetAppDetails(mock.Anything, mock.MatchedBy(func(req *apiclient.RepoServerAppDetailsQuery) bool {
					return req.AppName == tt.instanceName
				})).Return(&expectedResp, nil)
				app := guestbookApp.DeepCopy()
				app.Namespace = tt.namespace
				appLister, projLister := newAppAndProjLister(defaultProj, app)

				s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, testNamespace, settingsMgr, false)
				resp, err := s.GetAppDetails(t.Context(), &repository.RepoAppDetailsQuery{
					Source:     app.Spec.GetSourcePtrByIndex(0),
					AppName:    tt.namespace + "/guestbook",
					AppProject: "default",
				})
				require.NoError(t, err)
				assert.Equal(t, expectedResp, *resp)
			})
		}
	})
	t.Run("Test_ExistingMultiSourceApp001", func(t *testing.T) {
		repoServerClient := &mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: repoServerClient}
		enforcer := newEnforcer(kubeclientset)

		url := "https://helm.elastic.co"
		helmRepos := []*appsv1.Repository{{Repo: url}, {Repo: url}}
		db := &dbmocks.ArgoDB{}
		db.EXPECT().ListHelmRepositories(mock.Anything).Return(helmRepos, nil)
		db.EXPECT().GetRepository(mock.Anything, url, "default").Return(&appsv1.Repository{Repo: url}, nil)
		db.EXPECT().GetProjectRepositories("default").Return(nil, nil)
		db.EXPECT().GetProjectClusters(mock.Anything, "default").Return(nil, nil)
		expectedResp := apiclient.RepoAppDetailsResponse{Type: "Helm"}
		repoServerClient.EXPECT().GetAppDetails(mock.Anything, mock.Anything).Return(&expectedResp, nil)
		appLister, projLister := newAppAndProjLister(defaultProj, multiSourceApp001)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, testNamespace, settingsMgr, false)
		sources := multiSourceApp001.Spec.GetSources()
		assert.Len(t, sources, 2)
		resp, err := s.GetAppDetails(t.Context(), &repository.RepoAppDetailsQuery{
			Source:     &sources[0],
			AppName:    multiSourceApp001AppName,
			AppProject: "default",
		})
		require.NoError(t, err)
		assert.Equal(t, expectedResp, *resp)
		assert.Equal(t, "Helm", resp.Type)
		// Next source
		resp, err = s.GetAppDetails(t.Context(), &repository.RepoAppDetailsQuery{
			Source:     &sources[1],
			AppName:    multiSourceApp001AppName,
			AppProject: "default",
		})
		require.NoError(t, err)
		assert.Equal(t, expectedResp, *resp)
		assert.Equal(t, "Helm", resp.Type)
	})
	t.Run("Test_ExistingMultiSourceApp002", func(t *testing.T) {
		repoServerClient := &mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: repoServerClient}
		enforcer := newEnforcer(kubeclientset)

		url0 := "https://github.com/argoproj/argocd-example-apps.git"
		url1 := "https://helm.elastic.co"
		helmRepos := []*appsv1.Repository{{Repo: url0}, {Repo: url1}}
		db := &dbmocks.ArgoDB{}
		db.EXPECT().ListHelmRepositories(mock.Anything).Return(helmRepos, nil)
		db.EXPECT().GetRepository(mock.Anything, url0, "default").Return(&appsv1.Repository{Repo: url0}, nil)
		db.EXPECT().GetRepository(mock.Anything, url1, "default").Return(&appsv1.Repository{Repo: url1}, nil)
		db.EXPECT().GetProjectRepositories("default").Return(nil, nil)
		db.EXPECT().GetProjectClusters(mock.Anything, "default").Return(nil, nil)
		expectedResp0 := apiclient.RepoAppDetailsResponse{Type: "Plugin"}
		expectedResp1 := apiclient.RepoAppDetailsResponse{Type: "Helm"}
		repoServerClient.EXPECT().GetAppDetails(mock.Anything, mock.MatchedBy(func(req *apiclient.RepoServerAppDetailsQuery) bool { return req.Source.RepoURL == url0 })).Return(&expectedResp0, nil)
		repoServerClient.EXPECT().GetAppDetails(mock.Anything, mock.MatchedBy(func(req *apiclient.RepoServerAppDetailsQuery) bool { return req.Source.RepoURL == url1 })).Return(&expectedResp1, nil)
		appLister, projLister := newAppAndProjLister(defaultProj, multiSourceApp002)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, testNamespace, settingsMgr, false)
		sources := multiSourceApp002.Spec.GetSources()
		assert.Len(t, sources, 2)

		resp, err := s.GetAppDetails(t.Context(), &repository.RepoAppDetailsQuery{
			Source:     &sources[0],
			AppName:    multiSourceApp002AppName,
			AppProject: "default",
		})
		require.NoError(t, err)
		assert.Equal(t, "Plugin", resp.Type)
		assert.Equal(t, expectedResp0, *resp)
		// Next source
		resp, err = s.GetAppDetails(t.Context(), &repository.RepoAppDetailsQuery{
			Source:     &sources[1],
			AppName:    multiSourceApp002AppName,
			AppProject: "default",
		})
		require.NoError(t, err)
		assert.Equal(t, expectedResp1, *resp)
		assert.Equal(t, "Helm", resp.Type)
	})
	t.Run("Test_ExistingAppMismatchedProjectName", func(t *testing.T) {
		repoServerClient := mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: &repoServerClient}
		enforcer := newEnforcer(kubeclientset)

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.EXPECT().GetRepository(mock.Anything, url, "mismatch").Return(&appsv1.Repository{Repo: url}, nil)
		appLister, projLister := newAppAndProjLister(defaultProj, guestbookApp)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, testNamespace, settingsMgr, false)
		resp, err := s.GetAppDetails(t.Context(), &repository.RepoAppDetailsQuery{
			Source:     guestbookApp.Spec.GetSourcePtrByIndex(0),
			AppName:    "guestbook",
			AppProject: "mismatch",
		})
		assert.Equal(t, err, common.PermissionDeniedAPIError)
		assert.Nil(t, resp)
	})
	t.Run("Test_ExistingAppSourceNotInHistory", func(t *testing.T) {
		repoServerClient := mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: &repoServerClient}
		enforcer := newEnforcer(kubeclientset)

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.EXPECT().GetRepository(mock.Anything, url, "default").Return(&appsv1.Repository{Repo: url}, nil)
		appLister, projLister := newAppAndProjLister(defaultProj, guestbookApp)
		differentSource := guestbookApp.Spec.Source.DeepCopy()
		differentSource.Helm.ValueFiles = []string{"/etc/passwd"}

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, testNamespace, settingsMgr, false)
		resp, err := s.GetAppDetails(t.Context(), &repository.RepoAppDetailsQuery{
			Source:     differentSource,
			AppName:    "guestbook",
			AppProject: "default",
		})
		assert.Equal(t, err, common.PermissionDeniedAPIError)
		assert.Nil(t, resp)
	})
	t.Run("Test_ExistingAppSourceInHistory", func(t *testing.T) {
		repoServerClient := &mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: repoServerClient}
		enforcer := newEnforcer(kubeclientset)

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.EXPECT().GetRepository(mock.Anything, url, "default").Return(&appsv1.Repository{Repo: url}, nil)
		db.EXPECT().ListHelmRepositories(mock.Anything).Return(nil, nil)
		db.EXPECT().GetProjectRepositories("default").Return(nil, nil)
		db.EXPECT().GetProjectClusters(mock.Anything, "default").Return(nil, nil)
		expectedResp := apiclient.RepoAppDetailsResponse{Type: "Directory"}
		repoServerClient.EXPECT().GetAppDetails(mock.Anything, mock.Anything).Return(&expectedResp, nil)
		appLister, projLister := newAppAndProjLister(defaultProj, guestbookApp)
		previousSource := guestbookApp.Status.History[0].Source.DeepCopy()
		previousSource.TargetRevision = guestbookApp.Status.History[0].Revision

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, testNamespace, settingsMgr, false)
		resp, err := s.GetAppDetails(t.Context(), &repository.RepoAppDetailsQuery{
			Source:     previousSource,
			AppName:    "guestbook",
			AppProject: "default",
		})
		require.NoError(t, err)
		assert.Equal(t, expectedResp, *resp)
	})

	t.Run("Test_ExistingAppMultiSourceNotInHistory", func(t *testing.T) {
		repoServerClient := &mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: repoServerClient}
		enforcer := newEnforcer(kubeclientset)

		url := "https://helm.elastic.co"
		helmRepos := []*appsv1.Repository{{Repo: url}, {Repo: url}}
		db := &dbmocks.ArgoDB{}
		db.EXPECT().ListHelmRepositories(mock.Anything).Return(helmRepos, nil)
		db.EXPECT().GetRepository(mock.Anything, url, "default").Return(&appsv1.Repository{Repo: url}, nil)
		db.EXPECT().GetProjectRepositories("default").Return(nil, nil)
		db.EXPECT().GetProjectClusters(mock.Anything, "default").Return(nil, nil)
		expectedResp := apiclient.RepoAppDetailsResponse{Type: "Helm"}
		repoServerClient.EXPECT().GetAppDetails(mock.Anything, mock.Anything).Return(&expectedResp, nil)
		appLister, projLister := newAppAndProjLister(defaultProj, multiSourceApp001)

		differentSource := multiSourceApp001.Spec.Sources[0].DeepCopy()
		differentSource.Helm.ValueFiles = []string{"/etc/passwd"}

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, testNamespace, settingsMgr, false)
		resp, err := s.GetAppDetails(t.Context(), &repository.RepoAppDetailsQuery{
			Source:      differentSource,
			AppName:     multiSourceApp001AppName,
			AppProject:  "default",
			SourceIndex: 0,
			VersionId:   1,
		})
		assert.Equal(t, err, common.PermissionDeniedAPIError)
		assert.Nil(t, resp)
	})
	t.Run("Test_ExistingAppMultiSourceInHistory", func(t *testing.T) {
		repoServerClient := &mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: repoServerClient}
		enforcer := newEnforcer(kubeclientset)

		url := "https://helm.elastic.co"
		db := &dbmocks.ArgoDB{}
		db.EXPECT().GetRepository(mock.Anything, url, "default").Return(&appsv1.Repository{Repo: url}, nil)
		db.EXPECT().ListHelmRepositories(mock.Anything).Return(nil, nil)
		db.EXPECT().GetProjectRepositories("default").Return(nil, nil)
		db.EXPECT().GetProjectClusters(mock.Anything, "default").Return(nil, nil)
		expectedResp := apiclient.RepoAppDetailsResponse{Type: "Directory"}
		repoServerClient.EXPECT().GetAppDetails(mock.Anything, mock.Anything).Return(&expectedResp, nil)
		appLister, projLister := newAppAndProjLister(defaultProj, multiSourceApp001)
		previousSource := multiSourceApp001.Status.History[0].Sources[0].DeepCopy()
		previousSource.TargetRevision = multiSourceApp001.Status.History[0].Revisions[0]

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, testNamespace, settingsMgr, false)
		resp, err := s.GetAppDetails(t.Context(), &repository.RepoAppDetailsQuery{
			Source:      previousSource,
			AppName:     multiSourceApp001AppName,
			AppProject:  "default",
			SourceIndex: 0,
			VersionId:   1,
		})
		require.NoError(t, err)
		assert.Equal(t, expectedResp, *resp)
	})
}

func Test_isSourceInHistory(t *testing.T) {
	newApp := func() *appsv1.Application {
		return &appsv1.Application{
			Spec: appsv1.ApplicationSpec{
				Sources: []appsv1.ApplicationSource{
					{RepoURL: "https://git.example.com/repo.git", Path: "app", TargetRevision: "main"},
				},
			},
			Status: appsv1.ApplicationStatus{
				History: appsv1.RevisionHistories{
					{
						ID:        1,
						Revisions: []string{"abc123"},
						Sources: []appsv1.ApplicationSource{
							{RepoURL: "https://git.example.com/repo.git", Path: "app", TargetRevision: "HEAD"},
						},
					},
				},
			},
		}
	}
	historicalSource := appsv1.ApplicationSource{RepoURL: "https://git.example.com/repo.git", Path: "app", TargetRevision: "abc123"}

	t.Run("matches a historical source without modifying the app", func(t *testing.T) {
		app := newApp()
		assert.True(t, isSourceInHistory(app, historicalSource, 0, 1))
		assert.Equal(t, "HEAD", app.Status.History[0].Sources[0].TargetRevision)
	})
	t.Run("returns false for an out of range source index", func(t *testing.T) {
		assert.False(t, isSourceInHistory(newApp(), historicalSource, 1, 1))
		assert.False(t, isSourceInHistory(newApp(), historicalSource, -1, 1))
	})
	t.Run("returns false when revisions are missing", func(t *testing.T) {
		app := newApp()
		app.Status.History[0].Revisions = nil
		assert.False(t, isSourceInHistory(app, historicalSource, 0, 1))
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
	)}
}

func newEnforcer(kubeclientset *fake.Clientset) *rbac.Enforcer {
	enforcer := rbac.NewEnforcer(kubeclientset, testNamespace, common.ArgoCDRBACConfigMapName, nil)
	_ = enforcer.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
	enforcer.SetDefaultRole("role:admin")
	enforcer.SetClaimsEnforcerFunc(func(_ jwt.Claims, _ ...any) bool {
		return true
	})
	return enforcer
}

func TestGetRepository(t *testing.T) {
	type args struct {
		ctx              context.Context
		listRepositories func(context.Context, *repository.RepoQuery) (*appsv1.RepositoryList, error)
		q                *repository.RepoQuery
	}
	tests := []struct {
		name  string
		args  args
		want  *appsv1.Repository
		error error
	}{
		{
			name: "empty project and no repos",
			args: args{
				ctx: t.Context(),
				listRepositories: func(_ context.Context, _ *repository.RepoQuery) (*appsv1.RepositoryList, error) {
					return &appsv1.RepositoryList{
						Items: []*appsv1.Repository{
							{Repo: "something-else"},
						},
					}, nil
				},
				q: &repository.RepoQuery{},
			},
			want:  nil,
			error: common.PermissionDeniedAPIError,
		},
		{
			name: "empty project and no matching repos",
			args: args{
				ctx: t.Context(),
				listRepositories: func(_ context.Context, _ *repository.RepoQuery) (*appsv1.RepositoryList, error) {
					return &appsv1.RepositoryList{}, nil
				},
				q: &repository.RepoQuery{
					Repo: "foobar",
				},
			},
			want:  nil,
			error: common.PermissionDeniedAPIError,
		},
		{
			name: "empty project + matching repo with an empty project",
			args: args{
				ctx: t.Context(),
				listRepositories: func(_ context.Context, _ *repository.RepoQuery) (*appsv1.RepositoryList, error) {
					return &appsv1.RepositoryList{
						Items: []*appsv1.Repository{
							{Repo: "foobar", Project: ""},
						},
					}, nil
				},
				q: &repository.RepoQuery{
					Repo:       "foobar",
					AppProject: "",
				},
			},
			want: &appsv1.Repository{
				Repo:    "foobar",
				Project: "",
			},
			error: nil,
		},
		{
			name: "empty project + matching repo with a non-empty project",
			args: args{
				ctx: t.Context(),
				listRepositories: func(_ context.Context, _ *repository.RepoQuery) (*appsv1.RepositoryList, error) {
					return &appsv1.RepositoryList{
						Items: []*appsv1.Repository{
							{Repo: "foobar", Project: "foobar"},
						},
					}, nil
				},
				q: &repository.RepoQuery{
					Repo:       "foobar",
					AppProject: "",
				},
			},
			want: &appsv1.Repository{
				Repo:    "foobar",
				Project: "foobar",
			},
			error: nil,
		},
		{
			name: "non-empty project + matching repo with an empty project",
			args: args{
				ctx: t.Context(),
				listRepositories: func(_ context.Context, _ *repository.RepoQuery) (*appsv1.RepositoryList, error) {
					return &appsv1.RepositoryList{
						Items: []*appsv1.Repository{
							{Repo: "foobar", Project: ""},
						},
					}, nil
				},
				q: &repository.RepoQuery{
					Repo:       "foobar",
					AppProject: "foobar",
				},
			},
			want:  nil,
			error: errors.New(`repository not found for url "foobar" and project "foobar"`),
		},
		{
			name: "non-empty project + matching repo with a matching project",
			args: args{
				ctx: t.Context(),
				listRepositories: func(_ context.Context, _ *repository.RepoQuery) (*appsv1.RepositoryList, error) {
					return &appsv1.RepositoryList{
						Items: []*appsv1.Repository{
							{Repo: "foobar", Project: "foobar"},
						},
					}, nil
				},
				q: &repository.RepoQuery{
					Repo:       "foobar",
					AppProject: "foobar",
				},
			},
			want: &appsv1.Repository{
				Repo:    "foobar",
				Project: "foobar",
			},
			error: nil,
		},
		{
			name: "non-empty project + matching repo with a non-matching project",
			args: args{
				ctx: t.Context(),
				listRepositories: func(_ context.Context, _ *repository.RepoQuery) (*appsv1.RepositoryList, error) {
					return &appsv1.RepositoryList{
						Items: []*appsv1.Repository{
							{Repo: "foobar", Project: "something-else"},
						},
					}, nil
				},
				q: &repository.RepoQuery{
					Repo:       "foobar",
					AppProject: "foobar",
				},
			},
			want:  nil,
			error: errors.New(`repository not found for url "foobar" and project "foobar"`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getRepository(tt.args.ctx, tt.args.listRepositories, tt.args.q)
			assert.Equal(t, tt.error, err)
			assert.Equalf(t, tt.want, got, "getRepository(%v, %v) = %v", tt.args.ctx, tt.args.q, got)
		})
	}
}

func TestDeleteRepository(t *testing.T) {
	repositories := map[string]string{
		"valid": "https://bitbucket.org/workspace/repo.git",
		// Check a wrongly formatter repo as well, see https://github.com/argoproj/argo-cd/issues/20921
		"invalid": "git clone https://bitbucket.org/workspace/repo.git",
	}

	kubeclientset := fake.NewSimpleClientset(&argocdCM, &argocdSecret)
	settingsMgr := settings.NewSettingsManager(t.Context(), kubeclientset, testNamespace)

	for name, repo := range repositories {
		t.Run(name, func(t *testing.T) {
			repoServerClient := &mocks.RepoServerServiceClient{}
			repoServerClient.EXPECT().TestRepository(mock.Anything, mock.Anything).Return(&apiclient.TestRepositoryResponse{}, nil)

			repoServerClientset := mocks.Clientset{RepoServerServiceClient: repoServerClient}
			enforcer := newEnforcer(kubeclientset)

			db := &dbmocks.ArgoDB{}
			db.EXPECT().DeleteRepository(mock.Anything, repo, "default").Return(nil)
			db.EXPECT().ListRepositories(mock.Anything).Return([]*appsv1.Repository{{Repo: repo, Project: "default"}}, nil)
			db.EXPECT().GetRepository(mock.Anything, repo, "default").Return(&appsv1.Repository{Repo: repo, Project: "default"}, nil)
			appLister, projLister := newAppAndProjLister(defaultProj)

			s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, testNamespace, settingsMgr, false)
			resp, err := s.DeleteRepository(t.Context(), &repository.RepoQuery{Repo: repo, AppProject: "default"})
			require.NoError(t, err)
			assert.Equal(t, repository.RepoResponse{}, *resp)
		})
	}
}
