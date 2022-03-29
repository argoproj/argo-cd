package application

import (
	"context"
	coreerrors "errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	synccommon "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/utils/kube/kubetest"
	"github.com/argoproj/pkg/sync"
	"github.com/ghodss/yaml"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"
	k8scache "k8s.io/client-go/tools/cache"
	"k8s.io/utils/pointer"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	appsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	apps "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/fake"
	appinformer "github.com/argoproj/argo-cd/v2/pkg/client/informers/externalversions"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient/mocks"
	"github.com/argoproj/argo-cd/v2/server/rbacpolicy"
	"github.com/argoproj/argo-cd/v2/test"
	"github.com/argoproj/argo-cd/v2/util/assets"
	"github.com/argoproj/argo-cd/v2/util/cache"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/rbac"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

const (
	testNamespace = "default"
	fakeRepoURL   = "https://git.com/repo.git"
)

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

func fakeAppList() *apiclient.AppList {
	return &apiclient.AppList{
		Apps: map[string]string{
			"some/path": "Ksonnet",
		},
	}
}

func fakeResolveRevesionResponse() *apiclient.ResolveRevisionResponse {
	return &apiclient.ResolveRevisionResponse{
		Revision:          "f9ba9e98119bf8c1176fbd65dbae26a71d044add",
		AmbiguousRevision: "HEAD (f9ba9e98119bf8c1176fbd65dbae26a71d044add)",
	}
}

func fakeResolveRevesionResponseHelm() *apiclient.ResolveRevisionResponse {
	return &apiclient.ResolveRevisionResponse{
		Revision:          "0.7.*",
		AmbiguousRevision: "0.7.* (0.7.2)",
	}
}

func fakeRepoServerClient(isHelm bool) *mocks.RepoServerServiceClient {
	mockRepoServiceClient := mocks.RepoServerServiceClient{}
	mockRepoServiceClient.On("ListApps", mock.Anything, mock.Anything).Return(fakeAppList(), nil)
	mockRepoServiceClient.On("GenerateManifest", mock.Anything, mock.Anything).Return(&apiclient.ManifestResponse{}, nil)
	mockRepoServiceClient.On("GetAppDetails", mock.Anything, mock.Anything).Return(&apiclient.RepoAppDetailsResponse{}, nil)
	mockRepoServiceClient.On("TestRepository", mock.Anything, mock.Anything).Return(&apiclient.TestRepositoryResponse{}, nil)

	if isHelm {
		mockRepoServiceClient.On("ResolveRevision", mock.Anything, mock.Anything).Return(fakeResolveRevesionResponseHelm(), nil)
	} else {
		mockRepoServiceClient.On("ResolveRevision", mock.Anything, mock.Anything).Return(fakeResolveRevesionResponse(), nil)
	}

	return &mockRepoServiceClient
}

// return an ApplicationServiceServer which returns fake data
func newTestAppServer(objects ...runtime.Object) *Server {
	f := func(enf *rbac.Enforcer) {
		_ = enf.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
		enf.SetDefaultRole("role:admin")
	}
	return newTestAppServerWithEnforcerConfigure(f, objects...)
}

func newTestAppServerWithEnforcerConfigure(f func(*rbac.Enforcer), objects ...runtime.Object) *Server {
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
	ctx := context.Background()
	db := db.NewDB(testNamespace, settings.NewSettingsManager(ctx, kubeclientset, testNamespace), kubeclientset)
	_, err := db.CreateRepository(ctx, fakeRepo())
	errors.CheckError(err)
	_, err = db.CreateCluster(ctx, fakeCluster())
	errors.CheckError(err)

	mockRepoClient := &mocks.Clientset{RepoServerServiceClient: fakeRepoServerClient(false)}

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
	projWithSyncWindows := &appsv1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: "proj-maint", Namespace: "default"},
		Spec: appsv1.AppProjectSpec{
			SourceRepos:  []string{"*"},
			Destinations: []appsv1.ApplicationDestination{{Server: "*", Namespace: "*"}},
			SyncWindows:  appsv1.SyncWindows{},
		},
	}
	matchingWindow := &appsv1.SyncWindow{
		Kind:         "allow",
		Schedule:     "* * * * *",
		Duration:     "1h",
		Applications: []string{"test-app"},
	}
	projWithSyncWindows.Spec.SyncWindows = append(projWithSyncWindows.Spec.SyncWindows, matchingWindow)

	objects = append(objects, defaultProj, myProj, projWithSyncWindows)

	fakeAppsClientset := apps.NewSimpleClientset(objects...)
	factory := appinformer.NewSharedInformerFactoryWithOptions(fakeAppsClientset, 0, appinformer.WithNamespace(""), appinformer.WithTweakListOptions(func(options *metav1.ListOptions) {}))
	fakeProjLister := factory.Argoproj().V1alpha1().AppProjects().Lister().AppProjects(testNamespace)

	enforcer := rbac.NewEnforcer(kubeclientset, testNamespace, common.ArgoCDRBACConfigMapName, nil)
	f(enforcer)
	enforcer.SetClaimsEnforcerFunc(rbacpolicy.NewRBACPolicyEnforcer(enforcer, fakeProjLister).EnforceClaims)

	settingsMgr := settings.NewSettingsManager(ctx, kubeclientset, testNamespace)

	// populate the app informer with the fake objects
	appInformer := factory.Argoproj().V1alpha1().Applications().Informer()
	// TODO(jessesuen): probably should return cancel function so tests can stop background informer
	//ctx, cancel := context.WithCancel(context.Background())
	go appInformer.Run(ctx.Done())
	if !k8scache.WaitForCacheSync(ctx.Done(), appInformer.HasSynced) {
		panic("Timed out waiting for caches to sync")
	}

	projInformer := factory.Argoproj().V1alpha1().AppProjects().Informer()
	go projInformer.Run(ctx.Done())
	if !k8scache.WaitForCacheSync(ctx.Done(), projInformer.HasSynced) {
		panic("Timed out waiting for caches to sync")
	}

	server := NewServer(
		testNamespace,
		kubeclientset,
		fakeAppsClientset,
		factory.Argoproj().V1alpha1().Applications().Lister().Applications(testNamespace),
		appInformer,
		mockRepoClient,
		nil,
		&kubetest.MockKubectlCmd{},
		db,
		enforcer,
		sync.NewKeyLock(),
		settingsMgr,
		projInformer,
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

const fakeAppWithDestName = `
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
    name: fake-cluster
`

const fakeAppWithAnnotations = `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  namespace: default
  annotations:
    test.annotation: test
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

func newTestAppWithDestName(opts ...func(app *appsv1.Application)) *appsv1.Application {
	return createTestApp(fakeAppWithDestName, opts...)
}

func newTestApp(opts ...func(app *appsv1.Application)) *appsv1.Application {
	return createTestApp(fakeApp, opts...)
}

func newTestAppWithAnnotations(opts ...func(app *appsv1.Application)) *appsv1.Application {
	return createTestApp(fakeAppWithAnnotations, opts...)
}

func createTestApp(testApp string, opts ...func(app *appsv1.Application)) *appsv1.Application {
	var app appsv1.Application
	err := yaml.Unmarshal([]byte(testApp), &app)
	if err != nil {
		panic(err)
	}
	for i := range opts {
		opts[i](&app)
	}
	return &app
}

func TestListApps(t *testing.T) {
	appServer := newTestAppServer(newTestApp(func(app *appsv1.Application) {
		app.Name = "bcd"
	}), newTestApp(func(app *appsv1.Application) {
		app.Name = "abc"
	}), newTestApp(func(app *appsv1.Application) {
		app.Name = "def"
	}))

	res, err := appServer.List(context.Background(), &application.ApplicationQuery{})
	assert.NoError(t, err)
	var names []string
	for i := range res.Items {
		names = append(names, res.Items[i].Name)
	}
	assert.Equal(t, []string{"abc", "bcd", "def"}, names)
}

func TestCoupleAppsListApps(t *testing.T) {
	var objects []runtime.Object
	ctx := context.Background()

	var groups []string
	for i := 0; i < 50; i++ {
		groups = append(groups, fmt.Sprintf("group-%d", i))
	}
	// nolint:staticcheck
	ctx = context.WithValue(ctx, "claims", &jwt.MapClaims{"groups": groups})
	for projectId := 0; projectId < 100; projectId++ {
		projectName := fmt.Sprintf("proj-%d", projectId)
		for appId := 0; appId < 100; appId++ {
			objects = append(objects, newTestApp(func(app *appsv1.Application) {
				app.Name = fmt.Sprintf("app-%d-%d", projectId, appId)
				app.Spec.Project = projectName
			}))
		}
	}

	f := func(enf *rbac.Enforcer) {
		policy := `
p, role:test, applications, *, proj-10/*, allow
g, group-45, role:test
p, role:test2, applications, *, proj-15/*, allow
g, group-47, role:test2
p, role:test3, applications, *, proj-20/*, allow
g, group-49, role:test3
`
		_ = enf.SetUserPolicy(policy)
	}
	appServer := newTestAppServerWithEnforcerConfigure(f, objects...)

	res, err := appServer.List(ctx, &application.ApplicationQuery{})

	assert.NoError(t, err)
	var names []string
	for i := range res.Items {
		names = append(names, res.Items[i].Name)
	}
	assert.Equal(t, 300, len(names))
}

func TestCreateApp(t *testing.T) {
	testApp := newTestApp()
	appServer := newTestAppServer()
	testApp.Spec.Project = ""
	createReq := application.ApplicationCreateRequest{
		Application: *testApp,
	}
	app, err := appServer.Create(context.Background(), &createReq)
	assert.NoError(t, err)
	assert.NotNil(t, app)
	assert.NotNil(t, app.Spec)
	assert.Equal(t, app.Spec.Project, "default")
}

func TestCreateAppWithDestName(t *testing.T) {
	appServer := newTestAppServer()
	testApp := newTestAppWithDestName()
	createReq := application.ApplicationCreateRequest{
		Application: *testApp,
	}
	app, err := appServer.Create(context.Background(), &createReq)
	assert.NoError(t, err)
	assert.NotNil(t, app)
	assert.Equal(t, app.Spec.Destination.Server, "https://cluster-api.com")
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

	patched = false
	deleted = false
	revertValues := func() {
		patched = false
		deleted = false
	}

	t.Run("Delete with background propagation policy", func(t *testing.T) {
		policy := backgroundPropagationPolicy
		_, err = appServer.Delete(ctx, &application.ApplicationDeleteRequest{Name: &app.Name, PropagationPolicy: &policy})
		assert.Nil(t, err)
		assert.True(t, patched)
		assert.True(t, deleted)
		t.Cleanup(revertValues)
	})

	t.Run("Delete with cascade disabled and background propagation policy", func(t *testing.T) {
		policy := backgroundPropagationPolicy
		_, err = appServer.Delete(ctx, &application.ApplicationDeleteRequest{Name: &app.Name, Cascade: &falseVar, PropagationPolicy: &policy})
		assert.EqualError(t, err, "rpc error: code = InvalidArgument desc = cannot set propagation policy when cascading is disabled")
		assert.False(t, patched)
		assert.False(t, deleted)
		t.Cleanup(revertValues)
	})

	t.Run("Delete with invalid propagation policy", func(t *testing.T) {
		invalidPolicy := "invalid"
		_, err = appServer.Delete(ctx, &application.ApplicationDeleteRequest{Name: &app.Name, Cascade: &trueVar, PropagationPolicy: &invalidPolicy})
		assert.EqualError(t, err, "rpc error: code = InvalidArgument desc = invalid propagation policy: invalid")
		assert.False(t, patched)
		assert.False(t, deleted)
		t.Cleanup(revertValues)
	})

	t.Run("Delete with foreground propagation policy", func(t *testing.T) {
		policy := foregroundPropagationPolicy
		_, err = appServer.Delete(ctx, &application.ApplicationDeleteRequest{Name: &app.Name, Cascade: &trueVar, PropagationPolicy: &policy})
		assert.Nil(t, err)
		assert.True(t, patched)
		assert.True(t, deleted)
		t.Cleanup(revertValues)
	})
}

func TestDeleteApp_InvalidName(t *testing.T) {
	appServer := newTestAppServer()
	_, err := appServer.Delete(context.Background(), &application.ApplicationDeleteRequest{
		Name: pointer.StringPtr("foo"),
	})
	if !assert.Error(t, err) {
		return
	}
	assert.True(t, apierrors.IsNotFound(err))
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

	events, err := appServer.kubeclientset.CoreV1().Events(appServer.ns).List(context.Background(), metav1.ListOptions{})
	assert.Nil(t, err)
	event := events.Items[1]

	assert.Regexp(t, ".*initiated sync to HEAD \\([0-9A-Fa-f]{40}\\).*", event.Message)

	// set status.operationState to pretend that an operation has started by controller
	app.Status.OperationState = &appsv1.OperationState{
		Operation: *app.Operation,
		Phase:     synccommon.OperationRunning,
		StartedAt: metav1.NewTime(time.Now()),
	}
	_, err = appServer.appclientset.ArgoprojV1alpha1().Applications(appServer.ns).Update(context.Background(), app, metav1.UpdateOptions{})
	assert.Nil(t, err)

	resp, err := appServer.TerminateOperation(ctx, &application.OperationTerminateRequest{Name: &app.Name})
	assert.Nil(t, err)
	assert.NotNil(t, resp)

	app, err = appServer.Get(ctx, &application.ApplicationQuery{Name: &app.Name})
	assert.Nil(t, err)
	assert.NotNil(t, app)
	assert.Equal(t, synccommon.OperationTerminating, app.Status.OperationState.Phase)
}

func TestSyncHelm(t *testing.T) {
	ctx := context.Background()
	appServer := newTestAppServer()
	testApp := newTestApp()
	testApp.Spec.Source.RepoURL = "https://argoproj.github.io/argo-helm"
	testApp.Spec.Source.Path = ""
	testApp.Spec.Source.Chart = "argo-cd"
	testApp.Spec.Source.TargetRevision = "0.7.*"

	appServer.repoClientset = &mocks.Clientset{RepoServerServiceClient: fakeRepoServerClient(true)}

	app, err := appServer.Create(ctx, &application.ApplicationCreateRequest{Application: *testApp})
	assert.NoError(t, err)

	app, err = appServer.Sync(ctx, &application.ApplicationSyncRequest{Name: &app.Name})
	assert.NoError(t, err)
	assert.NotNil(t, app)
	assert.NotNil(t, app.Operation)

	events, err := appServer.kubeclientset.CoreV1().Events(appServer.ns).List(context.Background(), metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Equal(t, "Unknown user initiated sync to 0.7.* (0.7.2)", events.Items[1].Message)
}

func TestSyncGit(t *testing.T) {
	ctx := context.Background()
	appServer := newTestAppServer()
	testApp := newTestApp()
	testApp.Spec.Source.RepoURL = "https://github.com/org/test"
	testApp.Spec.Source.Path = "deploy"
	testApp.Spec.Source.TargetRevision = "0.7.*"
	app, err := appServer.Create(ctx, &application.ApplicationCreateRequest{Application: *testApp})
	assert.NoError(t, err)
	syncReq := &application.ApplicationSyncRequest{
		Name: &app.Name,
		Manifests: []string{
			`apiVersion: v1
			kind: ServiceAccount
			metadata:
			  name: test
			  namespace: test`,
		},
	}
	app, err = appServer.Sync(ctx, syncReq)
	assert.NoError(t, err)
	assert.NotNil(t, app)
	assert.NotNil(t, app.Operation)
	events, err := appServer.kubeclientset.CoreV1().Events(appServer.ns).List(context.Background(), metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Equal(t, "Unknown user initiated sync locally", events.Items[1].Message)
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
	// nolint:staticcheck
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
	testApp := newTestAppWithAnnotations()
	ctx := context.Background()
	// nolint:staticcheck
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

	app, err = appServer.Patch(ctx, &application.ApplicationPatchRequest{Name: &testApp.Name, Patch: `[{"op": "remove", "path": "/metadata/annotations/test.annotation"}]`})
	assert.NoError(t, err)
	assert.NotContains(t, app.Annotations, "test.annotation")
}

func TestAppMergePatch(t *testing.T) {
	testApp := newTestApp()
	ctx := context.Background()
	// nolint:staticcheck
	ctx = context.WithValue(ctx, "claims", &jwt.StandardClaims{Subject: "admin"})
	appServer := newTestAppServer(testApp)
	appServer.enf.SetDefaultRole("")

	app, err := appServer.Patch(ctx, &application.ApplicationPatchRequest{
		Name: &testApp.Name, Patch: `{"spec": { "source": { "path": "foo" } }}`, PatchType: "merge"})
	assert.NoError(t, err)
	assert.Equal(t, "foo", app.Spec.Source.Path)
}

func TestServer_GetApplicationSyncWindowsState(t *testing.T) {
	t.Run("Active", func(t *testing.T) {
		testApp := newTestApp()
		testApp.Spec.Project = "proj-maint"
		appServer := newTestAppServer(testApp)

		active, err := appServer.GetApplicationSyncWindows(context.Background(), &application.ApplicationSyncWindowsQuery{Name: &testApp.Name})
		assert.NoError(t, err)
		assert.Equal(t, 1, len(active.ActiveWindows))
	})
	t.Run("Inactive", func(t *testing.T) {
		testApp := newTestApp()
		testApp.Spec.Project = "default"
		appServer := newTestAppServer(testApp)

		active, err := appServer.GetApplicationSyncWindows(context.Background(), &application.ApplicationSyncWindowsQuery{Name: &testApp.Name})
		assert.NoError(t, err)
		assert.Equal(t, 0, len(active.ActiveWindows))
	})
	t.Run("ProjectDoesNotExist", func(t *testing.T) {
		testApp := newTestApp()
		testApp.Spec.Project = "none"
		appServer := newTestAppServer(testApp)

		active, err := appServer.GetApplicationSyncWindows(context.Background(), &application.ApplicationSyncWindowsQuery{Name: &testApp.Name})
		assert.Contains(t, err.Error(), "not found")
		assert.Nil(t, active)
	})
}

func TestGetCachedAppState(t *testing.T) {
	testApp := newTestApp()
	testApp.ObjectMeta.ResourceVersion = "1"
	testApp.Spec.Project = "none"
	appServer := newTestAppServer(testApp)
	fakeClientSet := appServer.appclientset.(*apps.Clientset)
	t.Run("NoError", func(t *testing.T) {
		err := appServer.getCachedAppState(context.Background(), testApp, func() error {
			return nil
		})
		assert.NoError(t, err)
	})
	t.Run("CacheMissErrorTriggersRefresh", func(t *testing.T) {
		retryCount := 0
		patched := false
		watcher := watch.NewFakeWithChanSize(1, true)

		// Configure fakeClientSet within lock, before requesting cached app state, to avoid data race
		{
			fakeClientSet.Lock()
			fakeClientSet.ReactionChain = nil
			fakeClientSet.WatchReactionChain = nil
			fakeClientSet.AddReactor("patch", "applications", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
				patched = true
				updated := testApp.DeepCopy()
				updated.ResourceVersion = "2"
				appServer.appBroadcaster.OnUpdate(testApp, updated)
				return true, testApp, nil
			})
			fakeClientSet.Unlock()
			fakeClientSet.AddWatchReactor("applications", func(action kubetesting.Action) (handled bool, ret watch.Interface, err error) {
				return true, watcher, nil
			})
		}

		err := appServer.getCachedAppState(context.Background(), testApp, func() error {
			res := cache.ErrCacheMiss
			if retryCount == 1 {
				res = nil
			}
			retryCount++
			return res
		})
		assert.Equal(t, nil, err)
		assert.Equal(t, 2, retryCount)
		assert.True(t, patched)
	})

	t.Run("NonCacheErrorDoesNotTriggerRefresh", func(t *testing.T) {
		randomError := coreerrors.New("random error")
		err := appServer.getCachedAppState(context.Background(), testApp, func() error {
			return randomError
		})
		assert.Equal(t, randomError, err)
	})
}

func TestSplitStatusPatch(t *testing.T) {
	specPatch := `{"spec":{"aaa":"bbb"}}`
	statusPatch := `{"status":{"ccc":"ddd"}}`
	{
		nonStatus, status, err := splitStatusPatch([]byte(specPatch))
		assert.NoError(t, err)
		assert.Equal(t, specPatch, string(nonStatus))
		assert.Nil(t, status)
	}
	{
		nonStatus, status, err := splitStatusPatch([]byte(statusPatch))
		assert.NoError(t, err)
		assert.Nil(t, nonStatus)
		assert.Equal(t, statusPatch, string(status))
	}
	{
		bothPatch := `{"spec":{"aaa":"bbb"},"status":{"ccc":"ddd"}}`
		nonStatus, status, err := splitStatusPatch([]byte(bothPatch))
		assert.NoError(t, err)
		assert.Equal(t, specPatch, string(nonStatus))
		assert.Equal(t, statusPatch, string(status))
	}
	{
		otherFields := `{"operation":{"eee":"fff"},"spec":{"aaa":"bbb"},"status":{"ccc":"ddd"}}`
		nonStatus, status, err := splitStatusPatch([]byte(otherFields))
		assert.NoError(t, err)
		assert.Equal(t, `{"operation":{"eee":"fff"},"spec":{"aaa":"bbb"}}`, string(nonStatus))
		assert.Equal(t, statusPatch, string(status))
	}
}

func TestLogsGetSelectedPod(t *testing.T) {
	deployment := appsv1.ResourceRef{Group: "", Version: "v1", Kind: "Deployment", Name: "deployment", UID: "1"}
	rs := appsv1.ResourceRef{Group: "", Version: "v1", Kind: "ReplicaSet", Name: "rs", UID: "2"}
	podRS := appsv1.ResourceRef{Group: "", Version: "v1", Kind: "Pod", Name: "podrs", UID: "3"}
	pod := appsv1.ResourceRef{Group: "", Version: "v1", Kind: "Pod", Name: "pod", UID: "4"}
	treeNodes := []appsv1.ResourceNode{
		{ResourceRef: deployment, ParentRefs: nil},
		{ResourceRef: rs, ParentRefs: []appsv1.ResourceRef{deployment}},
		{ResourceRef: podRS, ParentRefs: []appsv1.ResourceRef{rs}},
		{ResourceRef: pod, ParentRefs: nil},
	}
	appName := "appName"

	t.Run("GetAllPods", func(t *testing.T) {
		podQuery := application.ApplicationPodLogsQuery{
			Name: &appName,
		}
		pods := getSelectedPods(treeNodes, &podQuery)
		assert.Equal(t, 2, len(pods))
	})

	t.Run("GetRSPods", func(t *testing.T) {
		group := ""
		kind := "ReplicaSet"
		name := "rs"
		podQuery := application.ApplicationPodLogsQuery{
			Name:         &appName,
			Group:        &group,
			Kind:         &kind,
			ResourceName: &name,
		}
		pods := getSelectedPods(treeNodes, &podQuery)
		assert.Equal(t, 1, len(pods))
	})

	t.Run("GetDeploymentPods", func(t *testing.T) {
		group := ""
		kind := "Deployment"
		name := "deployment"
		podQuery := application.ApplicationPodLogsQuery{
			Name:         &appName,
			Group:        &group,
			Kind:         &kind,
			ResourceName: &name,
		}
		pods := getSelectedPods(treeNodes, &podQuery)
		assert.Equal(t, 1, len(pods))
	})

	t.Run("NoMatchingPods", func(t *testing.T) {
		group := ""
		kind := "Service"
		name := "service"
		podQuery := application.ApplicationPodLogsQuery{
			Name:         &appName,
			Group:        &group,
			Kind:         &kind,
			ResourceName: &name,
		}
		pods := getSelectedPods(treeNodes, &podQuery)
		assert.Equal(t, 0, len(pods))
	})
}

// refreshAnnotationRemover runs an infinite loop until it detects and removes refresh annotation or given context is done
func refreshAnnotationRemover(t *testing.T, ctx context.Context, patched *int32, appServer *Server, appName string, ch chan string) {
	for ctx.Err() == nil {
		a, err := appServer.appLister.Get(appName)
		require.NoError(t, err)
		a = a.DeepCopy()
		if a.GetAnnotations() != nil && a.GetAnnotations()[appsv1.AnnotationKeyRefresh] != "" {
			a.SetAnnotations(map[string]string{})
			a.SetResourceVersion("999")
			_, err = appServer.appclientset.ArgoprojV1alpha1().Applications(a.Namespace).Update(
				context.Background(), a, metav1.UpdateOptions{})
			require.NoError(t, err)
			atomic.AddInt32(patched, 1)
			ch <- ""
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestGetAppRefresh_NormalRefresh(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	testApp := newTestApp()
	testApp.ObjectMeta.ResourceVersion = "1"
	appServer := newTestAppServer(testApp)

	var patched int32

	ch := make(chan string, 1)

	go refreshAnnotationRemover(t, ctx, &patched, appServer, testApp.Name, ch)

	_, err := appServer.Get(context.Background(), &application.ApplicationQuery{
		Name:    &testApp.Name,
		Refresh: pointer.StringPtr(string(appsv1.RefreshTypeNormal)),
	})
	assert.NoError(t, err)

	select {
	case <-ch:
		assert.Equal(t, atomic.LoadInt32(&patched), int32(1))
	case <-time.After(10 * time.Second):
		assert.Fail(t, "Out of time ( 10 seconds )")
	}

}

func TestGetAppRefresh_HardRefresh(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	testApp := newTestApp()
	testApp.ObjectMeta.ResourceVersion = "1"
	appServer := newTestAppServer(testApp)

	var getAppDetailsQuery *apiclient.RepoServerAppDetailsQuery
	mockRepoServiceClient := mocks.RepoServerServiceClient{}
	mockRepoServiceClient.On("GetAppDetails", mock.Anything, mock.MatchedBy(func(q *apiclient.RepoServerAppDetailsQuery) bool {
		getAppDetailsQuery = q
		return true
	})).Return(&apiclient.RepoAppDetailsResponse{}, nil)
	appServer.repoClientset = &mocks.Clientset{RepoServerServiceClient: &mockRepoServiceClient}

	var patched int32

	ch := make(chan string, 1)

	go refreshAnnotationRemover(t, ctx, &patched, appServer, testApp.Name, ch)

	_, err := appServer.Get(context.Background(), &application.ApplicationQuery{
		Name:    &testApp.Name,
		Refresh: pointer.StringPtr(string(appsv1.RefreshTypeHard)),
	})
	assert.NoError(t, err)
	require.NotNil(t, getAppDetailsQuery)
	assert.True(t, getAppDetailsQuery.NoCache)
	assert.Equal(t, &testApp.Spec.Source, getAppDetailsQuery.Source)

	assert.NoError(t, err)
	select {
	case <-ch:
		assert.Equal(t, atomic.LoadInt32(&patched), int32(1))
	case <-time.After(10 * time.Second):
		assert.Fail(t, "Out of time ( 10 seconds )")
	}
}
