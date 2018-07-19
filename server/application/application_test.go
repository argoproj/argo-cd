package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/net/context"
	"k8s.io/client-go/kubernetes/fake"

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
	"github.com/argoproj/argo-cd/util/rbac"
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
func newTestAppServer() ApplicationServiceServer {
	kubeclientset := fake.NewSimpleClientset()
	enforcer := rbac.NewEnforcer(kubeclientset, testNamespace, common.ArgoCDRBACConfigMapName, nil)
	enforcer.SetBuiltinPolicy(test.BuiltinPolicy)
	enforcer.SetDefaultRole("role:admin")

	db := db.NewDB(testNamespace, kubeclientset)
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

	return NewServer(
		testNamespace,
		kubeclientset,
		apps.NewSimpleClientset(),
		mockRepoClient,
		db,
		enforcer,
		util.NewKeyLock(),
	)
}

func TestCreateApp(t *testing.T) {
	appServer := newTestAppServer()
	createReq := ApplicationCreateRequest{
		Application: appsv1.Application{
			Spec: appsv1.ApplicationSpec{
				Source: appsv1.ApplicationSource{
					RepoURL:        fakeRepoURL,
					Path:           "some/path",
					Environment:    "default",
					TargetRevision: "HEAD",
				},
				Destination: appsv1.ApplicationDestination{
					Server:    "https://cluster-api.com",
					Namespace: "default",
				},
			},
		},
	}
	app, err := appServer.Create(context.Background(), &createReq)
	assert.Nil(t, err)
	assert.Equal(t, app.Spec.Project, "default")
}
