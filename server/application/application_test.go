package application

import (
	"context"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/errors"
	appsv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	apps "github.com/argoproj/argo-cd/pkg/client/clientset/versioned/fake"
	mockrepo "github.com/argoproj/argo-cd/reposerver/mocks"
	"github.com/argoproj/argo-cd/reposerver/repository"
	mockreposerver "github.com/argoproj/argo-cd/reposerver/repository/mocks"
	"github.com/argoproj/argo-cd/test"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/rbac"
	"github.com/argoproj/argo-cd/util/settings"
)

const (
	testNamespace = "default"
	fakeRepoURL   = "https://git.com/repo.git"
)

type fakeCloser struct{}

func (f fakeCloser) Close() error {
	return nil
}

func fakeRepo() *appsv1.Repository {
	return &appsv1.Repository{
		Repo: fakeRepoURL,
	}
}

func fakeCluster() *appsv1.Cluster {
	return &appsv1.Cluster{
		Server: "https://cluster-api.com",
		Name:   "fake-cluster",
		Config: appsv1.ClusterConfig{},
	}
}

func fakeFileResponse() *repository.GetFileResponse {
	return &repository.GetFileResponse{
		Data: []byte(`
apiVersion: 0.1.0
environments:
  default:
    destination:
      namespace: default
      server: https://cluster-api.com
    k8sVersion: v1.10.0
    path: minikube
kind: ksonnet.io/app
name: test-app
version: 0.0.1
`),
	}
}

func fakeListDirResponse() *repository.FileList {
	return &repository.FileList{
		Items: []string{
			"some/path/app.yaml",
		},
	}
}

// return an ApplicationServiceServer which returns fake data
func newTestAppServer() *Server {
	kubeclientset := fake.NewSimpleClientset(&v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: "argocd-cm"},
	}, &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-secret",
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"admin.password":   []byte("test"),
			"server.secretkey": []byte("test"),
		},
	})
	enforcer := rbac.NewEnforcer(kubeclientset, testNamespace, common.ArgoCDRBACConfigMapName, nil)
	enforcer.SetBuiltinPolicy(test.BuiltinPolicy)
	enforcer.SetDefaultRole("role:admin")
	db := db.NewDB(testNamespace, settings.NewSettingsManager(kubeclientset, testNamespace), kubeclientset)
	ctx := context.Background()
	_, err := db.CreateRepository(ctx, fakeRepo())
	errors.CheckError(err)
	_, err = db.CreateCluster(ctx, fakeCluster())
	errors.CheckError(err)

	mockRepoServiceClient := mockreposerver.RepositoryServiceClient{}
	mockRepoServiceClient.On("GetFile", mock.Anything, mock.Anything).Return(fakeFileResponse(), nil)
	mockRepoServiceClient.On("ListDir", mock.Anything, mock.Anything).Return(fakeListDirResponse(), nil)

	mockRepoClient := &mockrepo.Clientset{}
	mockRepoClient.On("NewRepositoryClient").Return(&fakeCloser{}, &mockRepoServiceClient, nil)

	defaultProj := &appsv1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "default"},
		Spec: appsv1.AppProjectSpec{
			SourceRepos:  []string{"*"},
			Destinations: []appsv1.ApplicationDestination{{Server: "*", Namespace: "*"}},
		},
	}

	server := NewServer(
		testNamespace,
		kubeclientset,
		apps.NewSimpleClientset(defaultProj),
		mockRepoClient,
		nil,
		kube.KubectlCmd{},
		db,
		enforcer,
		util.NewKeyLock(),
	)
	return server.(*Server)
}

const fakeApp = `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  namespace: default
spec:
  source:
    path: some/path
    repoURL: https://git.com/repo.git
    targetRevision: HEAD
    environment: default
  destination:
    namespace: dummy-namespace
    server: https://cluster-api.com
`

func newTestApp() *appsv1.Application {
	var app appsv1.Application
	err := yaml.Unmarshal([]byte(fakeApp), &app)
	if err != nil {
		panic(err)
	}
	return &app
}

func TestCreateApp(t *testing.T) {
	appServer := newTestAppServer()
	createReq := ApplicationCreateRequest{
		Application: *newTestApp(),
	}
	app, err := appServer.Create(context.Background(), &createReq)
	assert.Nil(t, err)
	assert.Equal(t, app.Spec.Project, "default")
}

func TestDeleteApp(t *testing.T) {
	appServer := newTestAppServer()
	createReq := ApplicationCreateRequest{
		Application: *newTestApp(),
	}
	app, err := appServer.Create(context.Background(), &createReq)
	assert.Nil(t, err)

	app, err = appServer.Get(context.Background(), &ApplicationQuery{Name: &app.Name})
	assert.Nil(t, err)
	assert.NotNil(t, app)

	fakeAppCs := appServer.appclientset.(*apps.Clientset)
	// this removes the default */* reactor so we can set our own patch/delete reactor
	fakeAppCs.ReactionChain = nil
	patched := false
	deleted := false
	fakeAppCs.AddReactor("patch", "applications", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		patched = true
		return true, nil, nil
	})
	fakeAppCs.AddReactor("delete", "applications", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		deleted = true
		return true, nil, nil
	})
	appServer.appclientset = fakeAppCs

	trueVar := true
	_, err = appServer.Delete(context.Background(), &ApplicationDeleteRequest{Name: &app.Name, Cascade: &trueVar})
	assert.Nil(t, err)
	assert.True(t, patched)
	assert.True(t, deleted)

	// now call delete with cascade=false. patch should not be called
	falseVar := false
	patched = false
	deleted = false
	_, err = appServer.Delete(context.Background(), &ApplicationDeleteRequest{Name: &app.Name, Cascade: &falseVar})
	assert.Nil(t, err)
	assert.False(t, patched)
	assert.True(t, deleted)

}
