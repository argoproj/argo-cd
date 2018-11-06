package application

import (
	"encoding/json"
	"fmt"
	"testing"

	"golang.org/x/net/context"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	"github.com/argoproj/argo-cd/util/diff"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/rbac"
	"github.com/argoproj/argo-cd/util/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
func newTestAppServer(app *appsv1.Application) ApplicationServiceServer {
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
	enforcer.SetClaimsEnforcerFunc(func(rvals ...interface{}) bool {
		return true
	})
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
	var clientSet *apps.Clientset
	if app != nil {
		clientSet = apps.NewSimpleClientset(defaultProj, app)
	} else {
		clientSet = apps.NewSimpleClientset(defaultProj)
	}
	return NewServer(
		testNamespace,
		kubeclientset,
		clientSet,
		mockRepoClient,
		kube.KubectlCmd{},
		db,
		enforcer,
		util.NewKeyLock(),
	)
}

func TestCreateApp(t *testing.T) {
	appServer := newTestAppServer(nil)
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

var replicaManifest = `{
	"apiVersion": "apps/v1",
	"kind": "ReplicaSet",
	"metadata": {
	  "name": "my-pod"
	}
  }
  `

var podManifest = `{
  "apiVersion": "v1",
  "kind": "Pod",
  "metadata": {
    "name": "my-pod"
  }
}
`

func TestFindResource(t *testing.T) {
	appName := "defaultApp"
	app := &appsv1.Application{
		ObjectMeta: metav1.ObjectMeta{Name: appName, Namespace: testNamespace},
		Status: appsv1.ApplicationStatus{
			ComparisonResult: appsv1.ComparisonResult{
				Resources: []appsv1.ResourceState{{TargetState: podManifest, LiveState: podManifest}},
			},
		},
	}
	liveRes := findResource(app, "my-pod", "v1", "Pod", true)
	assert.NotNil(t, liveRes)
	targetRes := findResource(app, "my-pod", "v1", "Pod", false)
	assert.NotNil(t, targetRes)

}

func TestFindChildResource(t *testing.T) {
	appName := "defaultApp"
	app := &appsv1.Application{
		ObjectMeta: metav1.ObjectMeta{Name: appName, Namespace: testNamespace},
		Status: appsv1.ApplicationStatus{
			ComparisonResult: appsv1.ComparisonResult{
				Resources: []appsv1.ResourceState{{
					TargetState: replicaManifest,
					LiveState:   replicaManifest,
					ChildLiveResources: []appsv1.ResourceNode{{
						State: podManifest,
					}},
				}},
			},
		},
	}
	liveRes := findResource(app, "my-pod", "v1", "Pod", true)
	assert.NotNil(t, liveRes)
	targetRes := findResource(app, "my-pod", "v1", "Pod", false)
	assert.Nil(t, targetRes)

}

func TestFilterDiffManifest(t *testing.T) {
	appName := "defaultApp"
	app := &appsv1.Application{
		ObjectMeta: metav1.ObjectMeta{Name: appName, Namespace: testNamespace},
		Status: appsv1.ApplicationStatus{
			ComparisonResult: appsv1.ComparisonResult{
				Resources: []appsv1.ResourceState{{
					TargetState: replicaManifest,
					LiveState:   "",
				}, {
					TargetState: podManifest,
					LiveState:   "",
				}},
			},
		},
	}
	appServer := newTestAppServer(app)
	kind := "Pod"
	apiVersion := "v1"
	resourceName := "my-pod"
	diffResponse, err := appServer.DiffManifests(context.Background(), &ApplicationManifestDiffRequest{AppName: &appName, Kind: &kind, ApiVersion: &apiVersion, ResourceName: &resourceName})
	assert.Nil(t, err)
	assert.NotNil(t, diffResponse)
	assert.Len(t, diffResponse.Diffs, 1)

	var config unstructured.Unstructured
	err = json.Unmarshal([]byte(podManifest), &config)
	assert.Nil(t, err)
	expectedDiffResult := diff.Diff(&config, nil)
	expectedJSON, err := expectedDiffResult.JSONFormat()
	assert.Nil(t, err)

	assert.Equal(t, diffResponse.Diffs[0], expectedJSON)
}
func TestFilterDiffManifestFailureMissingField(t *testing.T) {
	appName := "defaultApp"
	expectedErr := fmt.Sprint("rpc error: code = InvalidArgument desc = Filtering manifest diff requires ResouceName, ApiVersion, and Kind")

	appServer := newTestAppServer(&appsv1.Application{
		ObjectMeta: metav1.ObjectMeta{Name: appName, Namespace: testNamespace},
	})
	kind := "kind"
	kindDiff, kindErr := appServer.DiffManifests(context.Background(), &ApplicationManifestDiffRequest{AppName: &appName, Kind: &kind})
	assert.Nil(t, kindDiff)
	assert.EqualError(t, kindErr, expectedErr)

	apiVersion := "apiVersion"
	apiVersionDiff, apiVersionErr := appServer.DiffManifests(context.Background(), &ApplicationManifestDiffRequest{AppName: &appName, ApiVersion: &apiVersion})
	assert.Nil(t, apiVersionDiff)
	assert.EqualError(t, apiVersionErr, expectedErr)

	resourceName := "resource"
	resourceNameDiff, resourceNameErr := appServer.DiffManifests(context.Background(), &ApplicationManifestDiffRequest{AppName: &appName, ResourceName: &resourceName})
	assert.Nil(t, resourceNameDiff)
	assert.EqualError(t, resourceNameErr, expectedErr)
}
