package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/repository"
	appsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	fakeapps "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/fake"
	appinformer "github.com/argoproj/argo-cd/v2/pkg/client/informers/externalversions"
	applisters "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient/mocks"
	"github.com/argoproj/argo-cd/v2/server/cache"
	"github.com/argoproj/argo-cd/v2/util/assets"
	cacheutil "github.com/argoproj/argo-cd/v2/util/cache"
	appstatecache "github.com/argoproj/argo-cd/v2/util/cache/appstate"
	"github.com/argoproj/argo-cd/v2/util/db"
	dbmocks "github.com/argoproj/argo-cd/v2/util/db/mocks"
	"github.com/argoproj/argo-cd/v2/util/rbac"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

const testNamespace = "default"

var (
	argocdCM = corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Namespace: testNamespace,
			Name:      "argocd-cm",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
	}
	argocdSecret = corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      "argocd-secret",
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"admin.password":   []byte("test"),
			"server.secretkey": []byte("test"),
		},
	}
	defaultProj = &appsv1.AppProject{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AppProject",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: testNamespace,
		},
		Spec: appsv1.AppProjectSpec{
			SourceRepos:  []string{"*"},
			Destinations: []appsv1.ApplicationDestination{{Server: "*", Namespace: "*"}},
		},
	}

	defaultProjNoSources = &appsv1.AppProject{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AppProject",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: testNamespace,
		},
		Spec: appsv1.AppProjectSpec{
			SourceRepos:  []string{},
			Destinations: []appsv1.ApplicationDestination{{Server: "*", Namespace: "*"}},
		},
	}

	guestbookApp = &appsv1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "guestbook",
			Namespace: testNamespace,
		},
		Spec: appsv1.ApplicationSpec{
			Project: "default",
			Source: appsv1.ApplicationSource{
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
)

func newAppAndProjLister(objects ...runtime.Object) (applisters.ApplicationNamespaceLister, applisters.AppProjectNamespaceLister) {
	fakeAppsClientset := fakeapps.NewSimpleClientset(objects...)
	factory := appinformer.NewSharedInformerFactoryWithOptions(fakeAppsClientset, 0, appinformer.WithNamespace(""), appinformer.WithTweakListOptions(func(options *metav1.ListOptions) {}))
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
	appLister := appsInformer.Lister().Applications(testNamespace)
	projLister := projInformer.Lister().AppProjects(testNamespace)
	return appLister, projLister
}

func Test_createRBACObject(t *testing.T) {
	object := createRBACObject("test-prj", "test-repo")
	assert.Equal(t, "test-prj/test-repo", object)
	objectWithoutPrj := createRBACObject("", "test-repo")
	assert.Equal(t, "test-repo", objectWithoutPrj)
}

func TestRepositoryServer(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset(&argocdCM, &argocdSecret)
	settingsMgr := settings.NewSettingsManager(context.Background(), kubeclientset, testNamespace)
	enforcer := newEnforcer(kubeclientset)
	appLister, projLister := newAppAndProjLister(defaultProj)
	argoDB := db.NewDB("default", settingsMgr, kubeclientset)

	t.Run("Test_getRepo", func(t *testing.T) {
		repoServerClient := mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: &repoServerClient}

		s := NewServer(&repoServerClientset, argoDB, enforcer, nil, appLister, projLister, settingsMgr)
		url := "https://test"
		repo, _ := s.getRepo(context.TODO(), url)
		assert.Equal(t, repo.Repo, url)
	})

	t.Run("Test_validateAccess", func(t *testing.T) {
		repoServerClient := mocks.RepoServerServiceClient{}
		repoServerClient.On("TestRepository", mock.Anything, mock.Anything).Return(&apiclient.TestRepositoryResponse{}, nil)
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: &repoServerClient}

		s := NewServer(&repoServerClientset, argoDB, enforcer, nil, appLister, projLister, settingsMgr)
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

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.On("GetRepository", context.TODO(), url).Return(&appsv1.Repository{Repo: url}, nil)
		db.On("RepositoryExists", context.TODO(), url).Return(true, nil)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, settingsMgr)
		repo, err := s.Get(context.TODO(), &repository.RepoQuery{
			Repo: url,
		})
		assert.Nil(t, err)
		assert.Equal(t, repo.Repo, url)
	})

	t.Run("Test_GetWithErrorShouldReturn403", func(t *testing.T) {
		repoServerClient := mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: &repoServerClient}

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.On("GetRepository", context.TODO(), url).Return(nil, errors.New("some error"))
		db.On("RepositoryExists", context.TODO(), url).Return(true, nil)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, settingsMgr)
		repo, err := s.Get(context.TODO(), &repository.RepoQuery{
			Repo: url,
		})
		assert.Nil(t, repo)
		assert.Equal(t, err, errPermissionDenied)
	})

	t.Run("Test_GetWithNotExistRepoShouldReturn404", func(t *testing.T) {
		repoServerClient := mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: &repoServerClient}

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.On("GetRepository", context.TODO(), url).Return(&appsv1.Repository{Repo: url}, nil)
		db.On("RepositoryExists", context.TODO(), url).Return(false, nil)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, settingsMgr)
		repo, err := s.Get(context.TODO(), &repository.RepoQuery{
			Repo: url,
		})
		assert.Nil(t, repo)
		assert.Equal(t, "rpc error: code = NotFound desc = repo 'https://test' not found", err.Error())
	})

	t.Run("Test_CreateRepositoryWithoutUpsert", func(t *testing.T) {
		repoServerClient := mocks.RepoServerServiceClient{}
		repoServerClient.On("TestRepository", mock.Anything, mock.Anything).Return(&apiclient.TestRepositoryResponse{}, nil)
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: &repoServerClient}

		db := &dbmocks.ArgoDB{}
		db.On("GetRepository", context.TODO(), "test").Return(nil, errors.New("not found"))
		db.On("CreateRepository", context.TODO(), mock.Anything).Return(&apiclient.TestRepositoryResponse{}).Return(&appsv1.Repository{
			Repo:    "repo",
			Project: "proj",
		}, nil)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, settingsMgr)
		repo, err := s.CreateRepository(context.TODO(), &repository.RepoCreateRequest{
			Repo: &appsv1.Repository{
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
		db.On("GetRepository", context.TODO(), "test").Return(&appsv1.Repository{
			Repo:     "test",
			Username: "test",
		}, nil)
		db.On("CreateRepository", context.TODO(), mock.Anything).Return(nil, status.Errorf(codes.AlreadyExists, "repository already exists"))
		db.On("UpdateRepository", context.TODO(), mock.Anything).Return(nil, nil)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, settingsMgr)
		repo, err := s.CreateRepository(context.TODO(), &repository.RepoCreateRequest{
			Repo: &appsv1.Repository{
				Repo:     "test",
				Username: "test",
			},
			Upsert: true,
		})

		assert.Nil(t, err)
		assert.Equal(t, repo.Repo, "test")
	})

}

func TestRepositoryServerListApps(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset(&argocdCM, &argocdSecret)
	settingsMgr := settings.NewSettingsManager(context.Background(), kubeclientset, testNamespace)

	t.Run("Test_WithoutAppCreateUpdatePrivileges", func(t *testing.T) {
		repoServerClient := mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: &repoServerClient}
		enforcer := newEnforcer(kubeclientset)
		enforcer.SetDefaultRole("role:readonly")

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.On("GetRepository", context.TODO(), url).Return(&appsv1.Repository{Repo: url}, nil)
		appLister, projLister := newAppAndProjLister(defaultProj)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, settingsMgr)
		resp, err := s.ListApps(context.TODO(), &repository.RepoAppsQuery{
			Repo:       "https://test",
			Revision:   "HEAD",
			AppName:    "foo",
			AppProject: "default",
		})
		assert.Nil(t, resp)
		assert.Equal(t, err, errPermissionDenied)
	})

	t.Run("Test_WithAppCreateUpdatePrivileges", func(t *testing.T) {
		repoServerClient := mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: &repoServerClient}
		enforcer := newEnforcer(kubeclientset)
		enforcer.SetDefaultRole("role:admin")
		appLister, projLister := newAppAndProjLister(defaultProj)

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.On("GetRepository", context.TODO(), url).Return(&appsv1.Repository{Repo: url}, nil)
		repoServerClient.On("ListApps", context.TODO(), mock.Anything).Return(&apiclient.AppList{
			Apps: map[string]string{
				"path/to/dir": "Kustomize",
			},
		}, nil)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, settingsMgr)
		resp, err := s.ListApps(context.TODO(), &repository.RepoAppsQuery{
			Repo:       "https://test",
			Revision:   "HEAD",
			AppName:    "foo",
			AppProject: "default",
		})
		assert.NoError(t, err)
		assert.Len(t, resp.Items, 1)
		assert.Equal(t, "path/to/dir", resp.Items[0].Path)
		assert.Equal(t, "Kustomize", resp.Items[0].Type)
	})

	t.Run("Test_WithAppCreateUpdatePrivilegesRepoNotAllowed", func(t *testing.T) {
		repoServerClient := mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: &repoServerClient}
		enforcer := newEnforcer(kubeclientset)
		enforcer.SetDefaultRole("role:admin")
		appLister, projLister := newAppAndProjLister(defaultProjNoSources)

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.On("GetRepository", context.TODO(), url).Return(&appsv1.Repository{Repo: url}, nil)
		repoServerClient.On("ListApps", context.TODO(), mock.Anything).Return(&apiclient.AppList{
			Apps: map[string]string{
				"path/to/dir": "Kustomize",
			},
		}, nil)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, settingsMgr)
		resp, err := s.ListApps(context.TODO(), &repository.RepoAppsQuery{
			Repo:       "https://test",
			Revision:   "HEAD",
			AppName:    "foo",
			AppProject: "default",
		})
		assert.Nil(t, resp)
		assert.Error(t, err, "repository 'https://test' not permitted in project 'default'")
	})
}

func TestRepositoryServerGetAppDetails(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset(&argocdCM, &argocdSecret)
	settingsMgr := settings.NewSettingsManager(context.Background(), kubeclientset, testNamespace)

	t.Run("Test_WithoutRepoReadPrivileges", func(t *testing.T) {
		repoServerClient := mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: &repoServerClient}
		enforcer := newEnforcer(kubeclientset)
		enforcer.SetDefaultRole("")

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.On("GetRepository", context.TODO(), url).Return(&appsv1.Repository{Repo: url}, nil)
		appLister, projLister := newAppAndProjLister(defaultProj)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, settingsMgr)
		resp, err := s.GetAppDetails(context.TODO(), &repository.RepoAppDetailsQuery{
			Source: &appsv1.ApplicationSource{
				RepoURL: url,
			},
			AppName:    "newapp",
			AppProject: "default",
		})
		assert.Nil(t, resp)
		assert.Error(t, err, "rpc error: code = PermissionDenied desc = permission denied: repositories, get, https://test")
	})
	t.Run("Test_WithoutAppReadPrivileges", func(t *testing.T) {
		repoServerClient := mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: &repoServerClient}
		enforcer := newEnforcer(kubeclientset)
		_ = enforcer.SetUserPolicy("p, role:readrepos, repositories, get, *, allow")
		enforcer.SetDefaultRole("role:readrepos")

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.On("GetRepository", context.TODO(), url).Return(&appsv1.Repository{Repo: url}, nil)
		appLister, projLister := newAppAndProjLister(defaultProj)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, settingsMgr)
		resp, err := s.GetAppDetails(context.TODO(), &repository.RepoAppDetailsQuery{
			Source: &appsv1.ApplicationSource{
				RepoURL: url,
			},
			AppName:    "newapp",
			AppProject: "default",
		})
		assert.Nil(t, resp)
		assert.Error(t, err, "rpc error: code = PermissionDenied desc = permission denied: applications, get, default/newapp")
	})
	t.Run("Test_WithoutCreatePrivileges", func(t *testing.T) {
		repoServerClient := mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: &repoServerClient}
		enforcer := newEnforcer(kubeclientset)
		enforcer.SetDefaultRole("role:readonly")

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.On("GetRepository", context.TODO(), url).Return(&appsv1.Repository{Repo: url}, nil)
		appLister, projLister := newAppAndProjLister(defaultProj)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, settingsMgr)
		resp, err := s.GetAppDetails(context.TODO(), &repository.RepoAppDetailsQuery{
			Source: &appsv1.ApplicationSource{
				RepoURL: url,
			},
			AppName:    "newapp",
			AppProject: "default",
		})
		assert.Nil(t, resp)
		assert.Error(t, err, "rpc error: code = PermissionDenied desc = permission denied: applications, create, default/newapp")
	})
	t.Run("Test_WithCreatePrivileges", func(t *testing.T) {
		repoServerClient := mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: &repoServerClient}
		enforcer := newEnforcer(kubeclientset)

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.On("ListHelmRepositories", context.TODO(), mock.Anything).Return(nil, nil)
		db.On("GetRepository", context.TODO(), url).Return(&appsv1.Repository{Repo: url}, nil)
		expectedResp := apiclient.RepoAppDetailsResponse{Type: "Directory"}
		repoServerClient.On("GetAppDetails", context.TODO(), mock.Anything).Return(&expectedResp, nil)
		appLister, projLister := newAppAndProjLister(defaultProj)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, settingsMgr)
		resp, err := s.GetAppDetails(context.TODO(), &repository.RepoAppDetailsQuery{
			Source: &appsv1.ApplicationSource{
				RepoURL: url,
			},
			AppName:    "newapp",
			AppProject: "default",
		})
		assert.NoError(t, err)
		assert.Equal(t, expectedResp, *resp)
	})
	t.Run("Test_RepoNotPermitted", func(t *testing.T) {
		repoServerClient := mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: &repoServerClient}
		enforcer := newEnforcer(kubeclientset)

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.On("GetRepository", context.TODO(), url).Return(&appsv1.Repository{Repo: url}, nil)
		expectedResp := apiclient.RepoAppDetailsResponse{Type: "Directory"}
		repoServerClient.On("GetAppDetails", context.TODO(), mock.Anything).Return(&expectedResp, nil)
		appLister, projLister := newAppAndProjLister(defaultProjNoSources)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, settingsMgr)
		resp, err := s.GetAppDetails(context.TODO(), &repository.RepoAppDetailsQuery{
			Source: &appsv1.ApplicationSource{
				RepoURL: url,
			},
			AppName:    "newapp",
			AppProject: "default",
		})
		assert.Error(t, err, "repository 'https://test' not permitted in project 'default'")
		assert.Nil(t, resp)
	})
	t.Run("Test_ExistingApp", func(t *testing.T) {
		repoServerClient := mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: &repoServerClient}
		enforcer := newEnforcer(kubeclientset)

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.On("ListHelmRepositories", context.TODO(), mock.Anything).Return(nil, nil)
		db.On("GetRepository", context.TODO(), url).Return(&appsv1.Repository{Repo: url}, nil)
		expectedResp := apiclient.RepoAppDetailsResponse{Type: "Directory"}
		repoServerClient.On("GetAppDetails", context.TODO(), mock.Anything).Return(&expectedResp, nil)
		appLister, projLister := newAppAndProjLister(defaultProj, guestbookApp)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, settingsMgr)
		resp, err := s.GetAppDetails(context.TODO(), &repository.RepoAppDetailsQuery{
			Source:     &guestbookApp.Spec.Source,
			AppName:    "guestbook",
			AppProject: "default",
		})
		assert.NoError(t, err)
		assert.Equal(t, expectedResp, *resp)
	})
	t.Run("Test_ExistingAppMismatchedProjectName", func(t *testing.T) {
		repoServerClient := mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: &repoServerClient}
		enforcer := newEnforcer(kubeclientset)

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.On("GetRepository", context.TODO(), url).Return(&appsv1.Repository{Repo: url}, nil)
		appLister, projLister := newAppAndProjLister(defaultProj, guestbookApp)

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, settingsMgr)
		resp, err := s.GetAppDetails(context.TODO(), &repository.RepoAppDetailsQuery{
			Source:     &guestbookApp.Spec.Source,
			AppName:    "guestbook",
			AppProject: "mismatch",
		})
		assert.Equal(t, errPermissionDenied, err)
		assert.Nil(t, resp)
	})
	t.Run("Test_ExistingAppSourceNotInHistory", func(t *testing.T) {
		repoServerClient := mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: &repoServerClient}
		enforcer := newEnforcer(kubeclientset)

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.On("GetRepository", context.TODO(), url).Return(&appsv1.Repository{Repo: url}, nil)
		appLister, projLister := newAppAndProjLister(defaultProj, guestbookApp)
		differentSource := guestbookApp.Spec.Source.DeepCopy()
		differentSource.Helm.ValueFiles = []string{"/etc/passwd"}

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, settingsMgr)
		resp, err := s.GetAppDetails(context.TODO(), &repository.RepoAppDetailsQuery{
			Source:     differentSource,
			AppName:    "guestbook",
			AppProject: "default",
		})
		assert.Equal(t, errPermissionDenied, err)
		assert.Nil(t, resp)
	})
	t.Run("Test_ExistingAppSourceInHistory", func(t *testing.T) {
		repoServerClient := mocks.RepoServerServiceClient{}
		repoServerClientset := mocks.Clientset{RepoServerServiceClient: &repoServerClient}
		enforcer := newEnforcer(kubeclientset)

		url := "https://test"
		db := &dbmocks.ArgoDB{}
		db.On("GetRepository", context.TODO(), url).Return(&appsv1.Repository{Repo: url}, nil)
		db.On("ListHelmRepositories", context.TODO(), mock.Anything).Return(nil, nil)
		expectedResp := apiclient.RepoAppDetailsResponse{Type: "Directory"}
		repoServerClient.On("GetAppDetails", context.TODO(), mock.Anything).Return(&expectedResp, nil)
		appLister, projLister := newAppAndProjLister(defaultProj, guestbookApp)
		previousSource := guestbookApp.Status.History[0].Source.DeepCopy()
		previousSource.TargetRevision = guestbookApp.Status.History[0].Revision

		s := NewServer(&repoServerClientset, db, enforcer, newFixtures().Cache, appLister, projLister, settingsMgr)
		resp, err := s.GetAppDetails(context.TODO(), &repository.RepoAppDetailsQuery{
			Source:     previousSource,
			AppName:    "guestbook",
			AppProject: "default",
		})
		assert.NoError(t, err)
		assert.Equal(t, expectedResp, *resp)
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
