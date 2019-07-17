package application

import (
	"context"
	"testing"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/pkg/apiclient/application"
	appsv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	apps "github.com/argoproj/argo-cd/pkg/client/clientset/versioned/fake"
	appinformer "github.com/argoproj/argo-cd/pkg/client/informers/externalversions"
	"github.com/argoproj/argo-cd/reposerver/apiclient"
	"github.com/argoproj/argo-cd/reposerver/apiclient/mocks"
	mockrepo "github.com/argoproj/argo-cd/reposerver/mocks"
	"github.com/argoproj/argo-cd/server/rbacpolicy"
	"github.com/argoproj/argo-cd/test"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/assets"
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

func fakeFileResponse() *apiclient.GetFileResponse {
	return &apiclient.GetFileResponse{
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

func fakeListDirResponse() *apiclient.FileList {
	return &apiclient.FileList{
		Items: []string{
			"some/path/app.yaml",
		},
	}
}

// return an ApplicationServiceServer which returns fake data
func newTestAppServer(objects ...runtime.Object) *Server {
	kubeclientset := fake.NewSimpleClientset(&v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      "argocd-cm",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
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
	db := db.NewDB(testNamespace, settings.NewSettingsManager(context.Background(), kubeclientset, testNamespace), kubeclientset)
	ctx := context.Background()
	_, err := db.CreateRepository(ctx, fakeRepo())
	errors.CheckError(err)
	_, err = db.CreateCluster(ctx, fakeCluster())
	errors.CheckError(err)

	mockRepoServiceClient := mocks.RepoServerServiceClient{}
	mockRepoServiceClient.On("GetFile", mock.Anything, mock.Anything).Return(fakeFileResponse(), nil)
	mockRepoServiceClient.On("ListDir", mock.Anything, mock.Anything).Return(fakeListDirResponse(), nil)
	mockRepoServiceClient.On("GenerateManifest", mock.Anything, mock.Anything).Return(&apiclient.ManifestResponse{}, nil)

	mockRepoClient := &mockrepo.Clientset{}
	mockRepoClient.On("NewRepoServerClient").Return(&fakeCloser{}, &mockRepoServiceClient, nil)

	defaultProj := &appsv1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "default"},
		Spec: appsv1.AppProjectSpec{
			SourceRepos:  []string{"*"},
			Destinations: []appsv1.ApplicationDestination{{Server: "*", Namespace: "*"}},
		},
	}
	myProj := &appsv1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: "my-proj", Namespace: "default"},
		Spec: appsv1.AppProjectSpec{
			SourceRepos:  []string{"*"},
			Destinations: []appsv1.ApplicationDestination{{Server: "*", Namespace: "*"}},
		},
	}
	objects = append(objects, defaultProj, myProj)

	fakeAppsClientset := apps.NewSimpleClientset(objects...)
	factory := appinformer.NewFilteredSharedInformerFactory(fakeAppsClientset, 0, "", func(options *metav1.ListOptions) {})
	fakeProjLister := factory.Argoproj().V1alpha1().AppProjects().Lister().AppProjects(testNamespace)

	enforcer := rbac.NewEnforcer(kubeclientset, testNamespace, common.ArgoCDRBACConfigMapName, nil)
	_ = enforcer.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
	enforcer.SetDefaultRole("role:admin")
	enforcer.SetClaimsEnforcerFunc(rbacpolicy.NewRBACPolicyEnforcer(enforcer, fakeProjLister).EnforceClaims)

	settingsMgr := settings.NewSettingsManager(context.Background(), kubeclientset, testNamespace)

	server := NewServer(
		testNamespace,
		kubeclientset,
		fakeAppsClientset,
		mockRepoClient,
		nil,
		kube.KubectlCmd{},
		db,
		enforcer,
		util.NewKeyLock(),
		settingsMgr,
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
    repoURL: https://github.com/argoproj/argocd-example-apps.git
    targetRevision: HEAD
    ksonnet:
      environment: default
  destination:
    namespace: ` + test.FakeDestNamespace + `
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
	testApp := newTestApp()
	appServer := newTestAppServer()
	testApp.Spec.Project = ""
	createReq := application.ApplicationCreateRequest{
		Application: *testApp,
	}
	app, err := appServer.Create(context.Background(), &createReq)
	assert.Nil(t, err)
	assert.Equal(t, app.Spec.Project, "default")
}

func TestUpdateApp(t *testing.T) {
	testApp := newTestApp()
	appServer := newTestAppServer(testApp)
	testApp.Spec.Project = ""
	app, err := appServer.Update(context.Background(), &application.ApplicationUpdateRequest{
		Application: testApp,
	})
	assert.Nil(t, err)
	assert.Equal(t, app.Spec.Project, "default")
}

func TestUpdateAppSpec(t *testing.T) {
	testApp := newTestApp()
	appServer := newTestAppServer(testApp)
	testApp.Spec.Project = ""
	spec, err := appServer.UpdateSpec(context.Background(), &application.ApplicationUpdateSpecRequest{
		Name: &testApp.Name,
		Spec: testApp.Spec,
	})
	assert.NoError(t, err)
	assert.Equal(t, "default", spec.Project)
	app, err := appServer.Get(context.Background(), &application.ApplicationQuery{Name: &testApp.Name})
	assert.NoError(t, err)
	assert.Equal(t, "default", app.Spec.Project)
}

func TestDeleteApp(t *testing.T) {
	ctx := context.Background()
	appServer := newTestAppServer()
	createReq := application.ApplicationCreateRequest{
		Application: *newTestApp(),
	}
	app, err := appServer.Create(ctx, &createReq)
	assert.Nil(t, err)

	app, err = appServer.Get(ctx, &application.ApplicationQuery{Name: &app.Name})
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
	_, err = appServer.Delete(ctx, &application.ApplicationDeleteRequest{Name: &app.Name, Cascade: &trueVar})
	assert.Nil(t, err)
	assert.True(t, patched)
	assert.True(t, deleted)

	// now call delete with cascade=false. patch should not be called
	falseVar := false
	patched = false
	deleted = false
	_, err = appServer.Delete(ctx, &application.ApplicationDeleteRequest{Name: &app.Name, Cascade: &falseVar})
	assert.Nil(t, err)
	assert.False(t, patched)
	assert.True(t, deleted)
}

func TestSyncAndTerminate(t *testing.T) {
	ctx := context.Background()
	appServer := newTestAppServer()
	testApp := newTestApp()
	testApp.Spec.Source.RepoURL = "https://github.com/argoproj/argo-cd.git"
	createReq := application.ApplicationCreateRequest{
		Application: *testApp,
	}
	app, err := appServer.Create(ctx, &createReq)
	assert.Nil(t, err)

	app, err = appServer.Sync(ctx, &application.ApplicationSyncRequest{Name: &app.Name})
	assert.Nil(t, err)
	assert.NotNil(t, app)
	assert.NotNil(t, app.Operation)

	events, err := appServer.kubeclientset.CoreV1().Events(appServer.ns).List(metav1.ListOptions{})
	assert.Nil(t, err)
	event := events.Items[1]

	assert.Regexp(t, ".*initiated sync to HEAD \\([0-9A-Fa-f]{40}\\).*", event.Message)

	// set status.operationState to pretend that an operation has started by controller
	app.Status.OperationState = &appsv1.OperationState{
		Operation: *app.Operation,
		Phase:     appsv1.OperationRunning,
		StartedAt: metav1.NewTime(time.Now()),
	}
	_, err = appServer.appclientset.ArgoprojV1alpha1().Applications(appServer.ns).Update(app)
	assert.Nil(t, err)

	resp, err := appServer.TerminateOperation(ctx, &application.OperationTerminateRequest{Name: &app.Name})
	assert.Nil(t, err)
	assert.NotNil(t, resp)

	app, err = appServer.Get(ctx, &application.ApplicationQuery{Name: &app.Name})
	assert.Nil(t, err)
	assert.NotNil(t, app)
	assert.Equal(t, appsv1.OperationTerminating, app.Status.OperationState.Phase)
}

func TestRollbackApp(t *testing.T) {
	testApp := newTestApp()
	testApp.Status.History = []appsv1.RevisionHistory{{
		ID:       1,
		Revision: "abc",
		Source:   *testApp.Spec.Source.DeepCopy(),
	}}
	appServer := newTestAppServer(testApp)

	updatedApp, err := appServer.Rollback(context.Background(), &application.ApplicationRollbackRequest{
		Name: &testApp.Name,
		ID:   1,
	})

	assert.Nil(t, err)

	assert.NotNil(t, updatedApp.Operation)
	assert.NotNil(t, updatedApp.Operation.Sync)
	assert.NotNil(t, updatedApp.Operation.Sync.Source)
	assert.Equal(t, "abc", updatedApp.Operation.Sync.Revision)
}

func TestUpdateAppProject(t *testing.T) {
	testApp := newTestApp()
	ctx := context.Background()
	ctx = context.WithValue(ctx, "claims", &jwt.StandardClaims{Subject: "admin"})
	appServer := newTestAppServer(testApp)
	appServer.enf.SetDefaultRole("")

	// Verify normal update works (without changing project)
	_ = appServer.enf.SetBuiltinPolicy(`p, admin, applications, update, default/test-app, allow`)
	_, err := appServer.Update(ctx, &application.ApplicationUpdateRequest{Application: testApp})
	assert.NoError(t, err)

	// Verify caller cannot update to another project
	testApp.Spec.Project = "my-proj"
	_, err = appServer.Update(ctx, &application.ApplicationUpdateRequest{Application: testApp})
	assert.Equal(t, status.Code(err), codes.PermissionDenied)

	// Verify inability to change projects without create privileges in new project
	_ = appServer.enf.SetBuiltinPolicy(`
p, admin, applications, update, default/test-app, allow
p, admin, applications, update, my-proj/test-app, allow
`)
	_, err = appServer.Update(ctx, &application.ApplicationUpdateRequest{Application: testApp})
	assert.Equal(t, status.Code(err), codes.PermissionDenied)

	// Verify inability to change projects without update privileges in new project
	_ = appServer.enf.SetBuiltinPolicy(`
p, admin, applications, update, default/test-app, allow
p, admin, applications, create, my-proj/test-app, allow
`)
	_, err = appServer.Update(ctx, &application.ApplicationUpdateRequest{Application: testApp})
	assert.Equal(t, status.Code(err), codes.PermissionDenied)

	// Verify inability to change projects without update privileges in old project
	_ = appServer.enf.SetBuiltinPolicy(`
p, admin, applications, create, my-proj/test-app, allow
p, admin, applications, update, my-proj/test-app, allow
`)
	_, err = appServer.Update(ctx, &application.ApplicationUpdateRequest{Application: testApp})
	assert.Equal(t, status.Code(err), codes.PermissionDenied)

	// Verify can update project with proper permissions
	_ = appServer.enf.SetBuiltinPolicy(`
p, admin, applications, update, default/test-app, allow
p, admin, applications, create, my-proj/test-app, allow
p, admin, applications, update, my-proj/test-app, allow
`)
	updatedApp, err := appServer.Update(ctx, &application.ApplicationUpdateRequest{Application: testApp})
	assert.NoError(t, err)
	assert.Equal(t, "my-proj", updatedApp.Spec.Project)
}

func TestAppJsonPatch(t *testing.T) {
	testApp := newTestApp()
	ctx := context.Background()
	ctx = context.WithValue(ctx, "claims", &jwt.StandardClaims{Subject: "admin"})
	appServer := newTestAppServer(testApp)
	appServer.enf.SetDefaultRole("")

	app, err := appServer.Patch(ctx, &application.ApplicationPatchRequest{Name: &testApp.Name, Patch: "garbage"})
	assert.Error(t, err)
	assert.Nil(t, app)

	app, err = appServer.Patch(ctx, &application.ApplicationPatchRequest{Name: &testApp.Name, Patch: "[]"})
	assert.NoError(t, err)
	assert.NotNil(t, app)

	app, err = appServer.Patch(ctx, &application.ApplicationPatchRequest{Name: &testApp.Name, Patch: `[{"op": "replace", "path": "/spec/source/path", "value": "foo"}]`})
	assert.NoError(t, err)
	assert.Equal(t, "foo", app.Spec.Source.Path)
}

func TestAppMergePatch(t *testing.T) {
	testApp := newTestApp()
	ctx := context.Background()
	ctx = context.WithValue(ctx, "claims", &jwt.StandardClaims{Subject: "admin"})
	appServer := newTestAppServer(testApp)
	appServer.enf.SetDefaultRole("")

	app, err := appServer.Patch(ctx, &application.ApplicationPatchRequest{
		Name: &testApp.Name, Patch: `{"spec": { "source": { "path": "foo" } }}`, PatchType: "merge"})
	assert.NoError(t, err)
	assert.Equal(t, "foo", app.Spec.Source.Path)
}
