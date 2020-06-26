package application

import (
	"context"
	coreerrors "errors"
	"testing"
	"time"

	synccommon "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/utils/kube/kubetest"
	"github.com/argoproj/pkg/sync"
	"github.com/dgrijalva/jwt-go"
	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apiclient/application"
	appsv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	apps "github.com/argoproj/argo-cd/pkg/client/clientset/versioned/fake"
	appinformer "github.com/argoproj/argo-cd/pkg/client/informers/externalversions"
	"github.com/argoproj/argo-cd/reposerver/apiclient"
	"github.com/argoproj/argo-cd/reposerver/apiclient/mocks"
	"github.com/argoproj/argo-cd/server/rbacpolicy"
	"github.com/argoproj/argo-cd/test"
	"github.com/argoproj/argo-cd/util/assets"
	"github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/errors"
	"github.com/argoproj/argo-cd/util/rbac"
	"github.com/argoproj/argo-cd/util/settings"
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
	ctx := context.Background()
	db := db.NewDB(testNamespace, settings.NewSettingsManager(ctx, kubeclientset, testNamespace), kubeclientset)
	_, err := db.CreateRepository(ctx, fakeRepo())
	errors.CheckError(err)
	_, err = db.CreateCluster(ctx, fakeCluster())
	errors.CheckError(err)

	mockRepoServiceClient := mocks.RepoServerServiceClient{}
	mockRepoServiceClient.On("ListApps", mock.Anything, mock.Anything).Return(fakeAppList(), nil)
	mockRepoServiceClient.On("GenerateManifest", mock.Anything, mock.Anything).Return(&apiclient.ManifestResponse{}, nil)
	mockRepoServiceClient.On("GetAppDetails", mock.Anything, mock.Anything).Return(&apiclient.RepoAppDetailsResponse{}, nil)

	mockRepoClient := &mocks.Clientset{RepoServerServiceClient: &mockRepoServiceClient}

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
	factory := appinformer.NewFilteredSharedInformerFactory(fakeAppsClientset, 0, "", func(options *metav1.ListOptions) {})
	fakeProjLister := factory.Argoproj().V1alpha1().AppProjects().Lister().AppProjects(testNamespace)

	enforcer := rbac.NewEnforcer(kubeclientset, testNamespace, common.ArgoCDRBACConfigMapName, nil)
	_ = enforcer.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
	enforcer.SetDefaultRole("role:admin")
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

func newTestAppWithDestName(opts ...func(app *appsv1.Application)) *appsv1.Application {
	return createTestApp(fakeAppWithDestName, opts...)
}

func newTestApp(opts ...func(app *appsv1.Application)) *appsv1.Application {
	return createTestApp(fakeApp, opts...)
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
	testApp := newTestApp()
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
			fakeClientSet.AddWatchReactor("applications", func(action kubetesting.Action) (handled bool, ret watch.Interface, err error) {
				return true, watcher, nil
			})
			fakeClientSet.Unlock()
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
