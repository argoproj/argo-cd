package application

import (
	"context"
	coreerrors "errors"
	"fmt"
	"io"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/argoproj/gitops-engine/pkg/health"
	synccommon "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/gitops-engine/pkg/utils/kube/kubetest"
	"github.com/argoproj/pkg/sync"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	k8sappsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	kubetesting "k8s.io/client-go/testing"
	k8scache "k8s.io/client-go/tools/cache"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	appsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	apps "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/fake"
	appinformer "github.com/argoproj/argo-cd/v2/pkg/client/informers/externalversions"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient/mocks"
	appmocks "github.com/argoproj/argo-cd/v2/server/application/mocks"
	servercache "github.com/argoproj/argo-cd/v2/server/cache"
	"github.com/argoproj/argo-cd/v2/server/rbacpolicy"
	"github.com/argoproj/argo-cd/v2/test"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/assets"
	"github.com/argoproj/argo-cd/v2/util/cache"
	cacheutil "github.com/argoproj/argo-cd/v2/util/cache"
	"github.com/argoproj/argo-cd/v2/util/cache/appstate"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/grpc"
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
	mockRepoServiceClient.On("GetRevisionMetadata", mock.Anything, mock.Anything).Return(&appsv1.RevisionMetadata{}, nil)
	mockWithFilesClient := &mocks.RepoServerService_GenerateManifestWithFilesClient{}
	mockWithFilesClient.On("Send", mock.Anything).Return(nil)
	mockWithFilesClient.On("CloseAndRecv").Return(&apiclient.ManifestResponse{}, nil)
	mockRepoServiceClient.On("GenerateManifestWithFiles", mock.Anything, mock.Anything).Return(mockWithFilesClient, nil)

	if isHelm {
		mockRepoServiceClient.On("ResolveRevision", mock.Anything, mock.Anything).Return(fakeResolveRevesionResponseHelm(), nil)
	} else {
		mockRepoServiceClient.On("ResolveRevision", mock.Anything, mock.Anything).Return(fakeResolveRevesionResponse(), nil)
	}

	return &mockRepoServiceClient
}

// return an ApplicationServiceServer which returns fake data
func newTestAppServer(t *testing.T, objects ...runtime.Object) *Server {
	f := func(enf *rbac.Enforcer) {
		_ = enf.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
		enf.SetDefaultRole("role:admin")
	}
	return newTestAppServerWithEnforcerConfigure(f, t, objects...)
}

func newTestAppServerWithEnforcerConfigure(f func(*rbac.Enforcer), t *testing.T, objects ...runtime.Object) *Server {
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

	broadcaster := new(appmocks.Broadcaster)
	broadcaster.On("Subscribe", mock.Anything, mock.Anything).Return(func() {}).Run(func(args mock.Arguments) {
		// Simulate the broadcaster notifying the subscriber of an application update.
		// The second parameter to Subscribe is filters. For the purposes of tests, we ignore the filters. Future tests
		// might require implementing those.
		go func() {
			events := args.Get(0).(chan *appsv1.ApplicationWatchEvent)
			for _, obj := range objects {
				app, ok := obj.(*appsv1.Application)
				if ok {
					oldVersion, err := strconv.Atoi(app.ResourceVersion)
					if err != nil {
						oldVersion = 0
					}
					clonedApp := app.DeepCopy()
					clonedApp.ResourceVersion = fmt.Sprintf("%d", oldVersion+1)
					events <- &appsv1.ApplicationWatchEvent{Type: watch.Added, Application: *clonedApp}
				}
			}
		}()
	})
	broadcaster.On("OnAdd", mock.Anything).Return()
	broadcaster.On("OnUpdate", mock.Anything, mock.Anything).Return()
	broadcaster.On("OnDelete", mock.Anything).Return()

	appStateCache := appstate.NewCache(cache.NewCache(cache.NewInMemoryCache(time.Hour)), time.Hour)
	// pre-populate the app cache
	for _, obj := range objects {
		app, ok := obj.(*appsv1.Application)
		if ok {
			err := appStateCache.SetAppManagedResources(app.Name, []*appsv1.ResourceDiff{})
			require.NoError(t, err)

			// Pre-populate the resource tree based on the app's resources.
			nodes := make([]appsv1.ResourceNode, len(app.Status.Resources))
			for i, res := range app.Status.Resources {
				nodes[i] = appsv1.ResourceNode{
					ResourceRef: appsv1.ResourceRef{
						Group:     res.Group,
						Kind:      res.Kind,
						Version:   res.Version,
						Name:      res.Name,
						Namespace: res.Namespace,
						UID:       "fake",
					},
				}
			}
			err = appStateCache.SetAppResourcesTree(app.Name, &appsv1.ApplicationTree{
				Nodes: nodes,
			})
			require.NoError(t, err)
		}
	}
	appCache := servercache.NewCache(appStateCache, time.Hour, time.Hour, time.Hour)

	kubectl := &kubetest.MockKubectlCmd{}
	kubectl = kubectl.WithGetResourceFunc(func(_ context.Context, _ *rest.Config, gvk schema.GroupVersionKind, name string, namespace string) (*unstructured.Unstructured, error) {
		for _, obj := range objects {
			if obj.GetObjectKind().GroupVersionKind().GroupKind() == gvk.GroupKind() {
				if obj, ok := obj.(*unstructured.Unstructured); ok && obj.GetName() == name && obj.GetNamespace() == namespace {
					return obj, nil
				}
			}
		}
		return nil, nil
	})

	server, _ := NewServer(
		testNamespace,
		kubeclientset,
		fakeAppsClientset,
		factory.Argoproj().V1alpha1().Applications().Lister(),
		appInformer,
		broadcaster,
		mockRepoClient,
		appCache,
		kubectl,
		db,
		enforcer,
		sync.NewKeyLock(),
		settingsMgr,
		projInformer,
		[]string{},
	)
	return server.(*Server)
}

// return an ApplicationServiceServer which returns fake data
func newTestAppServerWithBenchmark(b *testing.B, objects ...runtime.Object) *Server {
	f := func(enf *rbac.Enforcer) {
		_ = enf.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
		enf.SetDefaultRole("role:admin")
	}
	return newTestAppServerWithEnforcerConfigureWithBenchmark(f, b, objects...)
}

func newTestAppServerWithEnforcerConfigureWithBenchmark(f func(*rbac.Enforcer), b *testing.B, objects ...runtime.Object) *Server {
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
	require.NoError(b, err)
	_, err = db.CreateCluster(ctx, fakeCluster())
	require.NoError(b, err)

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

	go appInformer.Run(ctx.Done())
	if !k8scache.WaitForCacheSync(ctx.Done(), appInformer.HasSynced) {
		panic("Timed out waiting for caches to sync")
	}

	projInformer := factory.Argoproj().V1alpha1().AppProjects().Informer()
	go projInformer.Run(ctx.Done())
	if !k8scache.WaitForCacheSync(ctx.Done(), projInformer.HasSynced) {
		panic("Timed out waiting for caches to sync")
	}

	broadcaster := new(appmocks.Broadcaster)
	broadcaster.On("Subscribe", mock.Anything, mock.Anything).Return(func() {}).Run(func(args mock.Arguments) {
		// Simulate the broadcaster notifying the subscriber of an application update.
		// The second parameter to Subscribe is filters. For the purposes of tests, we ignore the filters. Future tests
		// might require implementing those.
		go func() {
			events := args.Get(0).(chan *appsv1.ApplicationWatchEvent)
			for _, obj := range objects {
				app, ok := obj.(*appsv1.Application)
				if ok {
					oldVersion, err := strconv.Atoi(app.ResourceVersion)
					if err != nil {
						oldVersion = 0
					}
					clonedApp := app.DeepCopy()
					clonedApp.ResourceVersion = fmt.Sprintf("%d", oldVersion+1)
					events <- &appsv1.ApplicationWatchEvent{Type: watch.Added, Application: *clonedApp}
				}
			}
		}()
	})
	broadcaster.On("OnAdd", mock.Anything).Return()
	broadcaster.On("OnUpdate", mock.Anything, mock.Anything).Return()
	broadcaster.On("OnDelete", mock.Anything).Return()

	appStateCache := appstate.NewCache(cache.NewCache(cache.NewInMemoryCache(time.Hour)), time.Hour)
	// pre-populate the app cache
	for _, obj := range objects {
		app, ok := obj.(*appsv1.Application)
		if ok {
			err := appStateCache.SetAppManagedResources(app.Name, []*appsv1.ResourceDiff{})
			require.NoError(b, err)

			// Pre-populate the resource tree based on the app's resources.
			nodes := make([]appsv1.ResourceNode, len(app.Status.Resources))
			for i, res := range app.Status.Resources {
				nodes[i] = appsv1.ResourceNode{
					ResourceRef: appsv1.ResourceRef{
						Group:     res.Group,
						Kind:      res.Kind,
						Version:   res.Version,
						Name:      res.Name,
						Namespace: res.Namespace,
						UID:       "fake",
					},
				}
			}
			err = appStateCache.SetAppResourcesTree(app.Name, &appsv1.ApplicationTree{
				Nodes: nodes,
			})
			require.NoError(b, err)
		}
	}
	appCache := servercache.NewCache(appStateCache, time.Hour, time.Hour, time.Hour)

	kubectl := &kubetest.MockKubectlCmd{}
	kubectl = kubectl.WithGetResourceFunc(func(_ context.Context, _ *rest.Config, gvk schema.GroupVersionKind, name string, namespace string) (*unstructured.Unstructured, error) {
		for _, obj := range objects {
			if obj.GetObjectKind().GroupVersionKind().GroupKind() == gvk.GroupKind() {
				if obj, ok := obj.(*unstructured.Unstructured); ok && obj.GetName() == name && obj.GetNamespace() == namespace {
					return obj, nil
				}
			}
		}
		return nil, nil
	})

	server, _ := NewServer(
		testNamespace,
		kubeclientset,
		fakeAppsClientset,
		factory.Argoproj().V1alpha1().Applications().Lister(),
		appInformer,
		broadcaster,
		mockRepoClient,
		appCache,
		kubectl,
		db,
		enforcer,
		sync.NewKeyLock(),
		settingsMgr,
		projInformer,
		[]string{},
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

type TestServerStream struct {
	ctx        context.Context
	appName    string
	headerSent bool
}

func (t *TestServerStream) SetHeader(metadata.MD) error {
	return nil
}

func (t *TestServerStream) SendHeader(metadata.MD) error {
	return nil
}

func (t *TestServerStream) SetTrailer(metadata.MD) {}

func (t *TestServerStream) Context() context.Context {
	return t.ctx
}

func (t *TestServerStream) SendMsg(m interface{}) error {
	return nil
}

func (t *TestServerStream) RecvMsg(m interface{}) error {
	return nil
}

func (t *TestServerStream) SendAndClose(r *apiclient.ManifestResponse) error {
	return nil
}

func (t *TestServerStream) Recv() (*application.ApplicationManifestQueryWithFilesWrapper, error) {
	if !t.headerSent {
		t.headerSent = true
		return &application.ApplicationManifestQueryWithFilesWrapper{Part: &application.ApplicationManifestQueryWithFilesWrapper_Query{
			Query: &application.ApplicationManifestQueryWithFiles{
				Name:     pointer.String(t.appName),
				Checksum: pointer.String(""),
			},
		}}, nil
	}
	return nil, io.EOF
}

func (t *TestServerStream) ServerStream() TestServerStream {
	return TestServerStream{}
}

type TestResourceTreeServer struct {
	ctx context.Context
}

func (t *TestResourceTreeServer) Send(tree *appsv1.ApplicationTree) error {
	return nil
}

func (t *TestResourceTreeServer) SetHeader(metadata.MD) error {
	return nil
}

func (t *TestResourceTreeServer) SendHeader(metadata.MD) error {
	return nil
}

func (t *TestResourceTreeServer) SetTrailer(metadata.MD) {}

func (t *TestResourceTreeServer) Context() context.Context {
	return t.ctx
}

func (t *TestResourceTreeServer) SendMsg(m interface{}) error {
	return nil
}

func (t *TestResourceTreeServer) RecvMsg(m interface{}) error {
	return nil
}

type TestPodLogsServer struct {
	ctx context.Context
}

func (t *TestPodLogsServer) Send(log *application.LogEntry) error {
	return nil
}

func (t *TestPodLogsServer) SetHeader(metadata.MD) error {
	return nil
}

func (t *TestPodLogsServer) SendHeader(metadata.MD) error {
	return nil
}

func (t *TestPodLogsServer) SetTrailer(metadata.MD) {}

func (t *TestPodLogsServer) Context() context.Context {
	return t.ctx
}

func (t *TestPodLogsServer) SendMsg(m interface{}) error {
	return nil
}

func (t *TestPodLogsServer) RecvMsg(m interface{}) error {
	return nil
}

func TestNoAppEnumeration(t *testing.T) {
	// This test ensures that malicious users can't infer the existence or non-existence of Applications by inspecting
	// error messages. The errors for "app does not exist" must be the same as errors for "you aren't allowed to
	// interact with this app."

	// These tests are only important on API calls where the full app RBAC name (project, namespace, and name) is _not_
	// known based on the query parameters. For example, the Create call cannot leak existence of Applications, because
	// the Application's project, namespace, and name are all specified in the API call. The call can be rejected
	// immediately if the user does not have access. But the Delete endpoint may be called with just the Application
	// name. So we cannot return a different error message for "does not exist" and "you don't have delete permissions,"
	// because the user could infer that the Application exists if they do not get the "does not exist" message. For
	// endpoints that do not require the full RBAC name, we must return a generic "permission denied" for both "does not
	// exist" and "no access."

	f := func(enf *rbac.Enforcer) {
		_ = enf.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
		enf.SetDefaultRole("role:none")
	}
	deployment := k8sappsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
	}
	testApp := newTestApp(func(app *appsv1.Application) {
		app.Name = "test"
		app.Status.Resources = []appsv1.ResourceStatus{
			{
				Group:     deployment.GroupVersionKind().Group,
				Kind:      deployment.GroupVersionKind().Kind,
				Version:   deployment.GroupVersionKind().Version,
				Name:      deployment.Name,
				Namespace: deployment.Namespace,
				Status:    "Synced",
			},
		}
		app.Status.History = []appsv1.RevisionHistory{
			{
				ID: 0,
				Source: appsv1.ApplicationSource{
					TargetRevision: "something-old",
				},
			},
		}
	})
	testDeployment := kube.MustToUnstructured(&deployment)
	appServer := newTestAppServerWithEnforcerConfigure(f, t, testApp, testDeployment)

	noRoleCtx := context.Background()
	// nolint:staticcheck
	adminCtx := context.WithValue(noRoleCtx, "claims", &jwt.MapClaims{"groups": []string{"admin"}})

	t.Run("Get", func(t *testing.T) {
		// nolint:staticcheck
		_, err := appServer.Get(adminCtx, &application.ApplicationQuery{Name: pointer.String("test")})
		assert.NoError(t, err)
		// nolint:staticcheck
		_, err = appServer.Get(noRoleCtx, &application.ApplicationQuery{Name: pointer.String("test")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		// nolint:staticcheck
		_, err = appServer.Get(adminCtx, &application.ApplicationQuery{Name: pointer.String("doest-not-exist")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
	})

	t.Run("GetManifests", func(t *testing.T) {
		_, err := appServer.GetManifests(adminCtx, &application.ApplicationManifestQuery{Name: pointer.String("test")})
		assert.NoError(t, err)
		_, err = appServer.GetManifests(noRoleCtx, &application.ApplicationManifestQuery{Name: pointer.String("test")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.GetManifests(adminCtx, &application.ApplicationManifestQuery{Name: pointer.String("doest-not-exist")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
	})

	t.Run("ListResourceEvents", func(t *testing.T) {
		_, err := appServer.ListResourceEvents(adminCtx, &application.ApplicationResourceEventsQuery{Name: pointer.String("test")})
		assert.NoError(t, err)
		_, err = appServer.ListResourceEvents(noRoleCtx, &application.ApplicationResourceEventsQuery{Name: pointer.String("test")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.ListResourceEvents(adminCtx, &application.ApplicationResourceEventsQuery{Name: pointer.String("doest-not-exist")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
	})

	t.Run("UpdateSpec", func(t *testing.T) {
		_, err := appServer.UpdateSpec(adminCtx, &application.ApplicationUpdateSpecRequest{Name: pointer.String("test"), Spec: &appsv1.ApplicationSpec{
			Destination: appsv1.ApplicationDestination{Namespace: "default", Server: "https://cluster-api.com"},
			Source:      &appsv1.ApplicationSource{RepoURL: "https://some-fake-source", Path: "."},
		}})
		assert.NoError(t, err)
		_, err = appServer.UpdateSpec(noRoleCtx, &application.ApplicationUpdateSpecRequest{Name: pointer.String("test"), Spec: &appsv1.ApplicationSpec{
			Destination: appsv1.ApplicationDestination{Namespace: "default", Server: "https://cluster-api.com"},
			Source:      &appsv1.ApplicationSource{RepoURL: "https://some-fake-source", Path: "."},
		}})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.UpdateSpec(adminCtx, &application.ApplicationUpdateSpecRequest{Name: pointer.String("doest-not-exist"), Spec: &appsv1.ApplicationSpec{
			Destination: appsv1.ApplicationDestination{Namespace: "default", Server: "https://cluster-api.com"},
			Source:      &appsv1.ApplicationSource{RepoURL: "https://some-fake-source", Path: "."},
		}})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
	})

	t.Run("Patch", func(t *testing.T) {
		_, err := appServer.Patch(adminCtx, &application.ApplicationPatchRequest{Name: pointer.String("test"), Patch: pointer.String(`[{"op": "replace", "path": "/spec/source/path", "value": "foo"}]`)})
		assert.NoError(t, err)
		_, err = appServer.Patch(noRoleCtx, &application.ApplicationPatchRequest{Name: pointer.String("test"), Patch: pointer.String(`[{"op": "replace", "path": "/spec/source/path", "value": "foo"}]`)})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.Patch(adminCtx, &application.ApplicationPatchRequest{Name: pointer.String("doest-not-exist")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
	})

	t.Run("GetResource", func(t *testing.T) {
		_, err := appServer.GetResource(adminCtx, &application.ApplicationResourceRequest{Name: pointer.String("test"), ResourceName: pointer.String("test"), Group: pointer.String("apps"), Kind: pointer.String("Deployment"), Namespace: pointer.String("test")})
		assert.NoError(t, err)
		_, err = appServer.GetResource(noRoleCtx, &application.ApplicationResourceRequest{Name: pointer.String("test"), ResourceName: pointer.String("test"), Group: pointer.String("apps"), Kind: pointer.String("Deployment"), Namespace: pointer.String("test")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.GetResource(adminCtx, &application.ApplicationResourceRequest{Name: pointer.String("doest-not-exist"), ResourceName: pointer.String("test"), Group: pointer.String("apps"), Kind: pointer.String("Deployment"), Namespace: pointer.String("test")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
	})

	t.Run("PatchResource", func(t *testing.T) {
		_, err := appServer.PatchResource(adminCtx, &application.ApplicationResourcePatchRequest{Name: pointer.String("test"), ResourceName: pointer.String("test"), Group: pointer.String("apps"), Kind: pointer.String("Deployment"), Namespace: pointer.String("test"), Patch: pointer.String(`[{"op": "replace", "path": "/spec/replicas", "value": 3}]`)})
		// This will always throw an error, because the kubectl mock for PatchResource is hard-coded to return nil.
		// The best we can do is to confirm we get past the permission check.
		assert.NotEqual(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.PatchResource(noRoleCtx, &application.ApplicationResourcePatchRequest{Name: pointer.String("test"), ResourceName: pointer.String("test"), Group: pointer.String("apps"), Kind: pointer.String("Deployment"), Namespace: pointer.String("test"), Patch: pointer.String(`[{"op": "replace", "path": "/spec/replicas", "value": 3}]`)})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.PatchResource(adminCtx, &application.ApplicationResourcePatchRequest{Name: pointer.String("doest-not-exist"), ResourceName: pointer.String("test"), Group: pointer.String("apps"), Kind: pointer.String("Deployment"), Namespace: pointer.String("test"), Patch: pointer.String(`[{"op": "replace", "path": "/spec/replicas", "value": 3}]`)})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
	})

	t.Run("DeleteResource", func(t *testing.T) {
		_, err := appServer.DeleteResource(adminCtx, &application.ApplicationResourceDeleteRequest{Name: pointer.String("test"), ResourceName: pointer.String("test"), Group: pointer.String("apps"), Kind: pointer.String("Deployment"), Namespace: pointer.String("test")})
		assert.NoError(t, err)
		_, err = appServer.DeleteResource(noRoleCtx, &application.ApplicationResourceDeleteRequest{Name: pointer.String("test"), ResourceName: pointer.String("test"), Group: pointer.String("apps"), Kind: pointer.String("Deployment"), Namespace: pointer.String("test")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.DeleteResource(adminCtx, &application.ApplicationResourceDeleteRequest{Name: pointer.String("doest-not-exist"), ResourceName: pointer.String("test"), Group: pointer.String("apps"), Kind: pointer.String("Deployment"), Namespace: pointer.String("test")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
	})

	t.Run("ResourceTree", func(t *testing.T) {
		_, err := appServer.ResourceTree(adminCtx, &application.ResourcesQuery{ApplicationName: pointer.String("test")})
		assert.NoError(t, err)
		_, err = appServer.ResourceTree(noRoleCtx, &application.ResourcesQuery{ApplicationName: pointer.String("test")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.ResourceTree(adminCtx, &application.ResourcesQuery{ApplicationName: pointer.String("doest-not-exist")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
	})

	t.Run("RevisionMetadata", func(t *testing.T) {
		_, err := appServer.RevisionMetadata(adminCtx, &application.RevisionMetadataQuery{Name: pointer.String("test")})
		assert.NoError(t, err)
		_, err = appServer.RevisionMetadata(noRoleCtx, &application.RevisionMetadataQuery{Name: pointer.String("test")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.RevisionMetadata(adminCtx, &application.RevisionMetadataQuery{Name: pointer.String("doest-not-exist")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
	})

	t.Run("ManagedResources", func(t *testing.T) {
		_, err := appServer.ManagedResources(adminCtx, &application.ResourcesQuery{ApplicationName: pointer.String("test")})
		assert.NoError(t, err)
		_, err = appServer.ManagedResources(noRoleCtx, &application.ResourcesQuery{ApplicationName: pointer.String("test")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.ManagedResources(adminCtx, &application.ResourcesQuery{ApplicationName: pointer.String("doest-not-exist")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
	})

	t.Run("Sync", func(t *testing.T) {
		_, err := appServer.Sync(adminCtx, &application.ApplicationSyncRequest{Name: pointer.String("test")})
		assert.NoError(t, err)
		_, err = appServer.Sync(noRoleCtx, &application.ApplicationSyncRequest{Name: pointer.String("test")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.Sync(adminCtx, &application.ApplicationSyncRequest{Name: pointer.String("doest-not-exist")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
	})

	t.Run("TerminateOperation", func(t *testing.T) {
		// The sync operation is already started from the previous test. We just need to set the field that the
		// controller would set if this were an actual Argo CD environment.
		setSyncRunningOperationState(t, appServer)
		_, err := appServer.TerminateOperation(adminCtx, &application.OperationTerminateRequest{Name: pointer.String("test")})
		assert.NoError(t, err)
		_, err = appServer.TerminateOperation(noRoleCtx, &application.OperationTerminateRequest{Name: pointer.String("test")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.TerminateOperation(adminCtx, &application.OperationTerminateRequest{Name: pointer.String("doest-not-exist")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
	})

	t.Run("Rollback", func(t *testing.T) {
		unsetSyncRunningOperationState(t, appServer)
		_, err := appServer.Rollback(adminCtx, &application.ApplicationRollbackRequest{Name: pointer.String("test")})
		assert.NoError(t, err)
		_, err = appServer.Rollback(noRoleCtx, &application.ApplicationRollbackRequest{Name: pointer.String("test")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.Rollback(adminCtx, &application.ApplicationRollbackRequest{Name: pointer.String("doest-not-exist")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
	})

	t.Run("ListResourceActions", func(t *testing.T) {
		_, err := appServer.ListResourceActions(adminCtx, &application.ApplicationResourceRequest{Name: pointer.String("test"), ResourceName: pointer.String("test"), Group: pointer.String("apps"), Kind: pointer.String("Deployment"), Namespace: pointer.String("test")})
		assert.NoError(t, err)
		_, err = appServer.ListResourceActions(noRoleCtx, &application.ApplicationResourceRequest{Name: pointer.String("test")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.ListResourceActions(noRoleCtx, &application.ApplicationResourceRequest{Group: pointer.String("argoproj.io"), Kind: pointer.String("Application"), Name: pointer.String("test")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.ListResourceActions(adminCtx, &application.ApplicationResourceRequest{Name: pointer.String("doest-not-exist")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
	})

	t.Run("RunResourceAction", func(t *testing.T) {
		_, err := appServer.RunResourceAction(adminCtx, &application.ResourceActionRunRequest{Name: pointer.String("test"), ResourceName: pointer.String("test"), Group: pointer.String("apps"), Kind: pointer.String("Deployment"), Namespace: pointer.String("test"), Action: pointer.String("restart")})
		assert.NoError(t, err)
		_, err = appServer.RunResourceAction(noRoleCtx, &application.ResourceActionRunRequest{Name: pointer.String("test")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.RunResourceAction(noRoleCtx, &application.ResourceActionRunRequest{Group: pointer.String("argoproj.io"), Kind: pointer.String("Application"), Name: pointer.String("test")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.RunResourceAction(adminCtx, &application.ResourceActionRunRequest{Name: pointer.String("doest-not-exist")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
	})

	t.Run("GetApplicationSyncWindows", func(t *testing.T) {
		_, err := appServer.GetApplicationSyncWindows(adminCtx, &application.ApplicationSyncWindowsQuery{Name: pointer.String("test")})
		assert.NoError(t, err)
		_, err = appServer.GetApplicationSyncWindows(noRoleCtx, &application.ApplicationSyncWindowsQuery{Name: pointer.String("test")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.GetApplicationSyncWindows(adminCtx, &application.ApplicationSyncWindowsQuery{Name: pointer.String("doest-not-exist")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
	})

	t.Run("GetManifestsWithFiles", func(t *testing.T) {
		err := appServer.GetManifestsWithFiles(&TestServerStream{ctx: adminCtx, appName: "test"})
		assert.NoError(t, err)
		err = appServer.GetManifestsWithFiles(&TestServerStream{ctx: noRoleCtx, appName: "test"})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		err = appServer.GetManifestsWithFiles(&TestServerStream{ctx: adminCtx, appName: "does-not-exist"})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
	})

	t.Run("WatchResourceTree", func(t *testing.T) {
		err := appServer.WatchResourceTree(&application.ResourcesQuery{ApplicationName: pointer.String("test")}, &TestResourceTreeServer{ctx: adminCtx})
		assert.NoError(t, err)
		err = appServer.WatchResourceTree(&application.ResourcesQuery{ApplicationName: pointer.String("test")}, &TestResourceTreeServer{ctx: noRoleCtx})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		err = appServer.WatchResourceTree(&application.ResourcesQuery{ApplicationName: pointer.String("does-not-exist")}, &TestResourceTreeServer{ctx: adminCtx})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
	})

	t.Run("PodLogs", func(t *testing.T) {
		err := appServer.PodLogs(&application.ApplicationPodLogsQuery{Name: pointer.String("test")}, &TestPodLogsServer{ctx: adminCtx})
		assert.NoError(t, err)
		err = appServer.PodLogs(&application.ApplicationPodLogsQuery{Name: pointer.String("test")}, &TestPodLogsServer{ctx: noRoleCtx})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		err = appServer.PodLogs(&application.ApplicationPodLogsQuery{Name: pointer.String("does-not-exist")}, &TestPodLogsServer{ctx: adminCtx})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
	})

	t.Run("ListLinks", func(t *testing.T) {
		_, err := appServer.ListLinks(adminCtx, &application.ListAppLinksRequest{Name: pointer.String("test")})
		assert.NoError(t, err)
		_, err = appServer.ListLinks(noRoleCtx, &application.ListAppLinksRequest{Name: pointer.String("test")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.ListLinks(adminCtx, &application.ListAppLinksRequest{Name: pointer.String("does-not-exist")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
	})

	t.Run("ListResourceLinks", func(t *testing.T) {
		_, err := appServer.ListResourceLinks(adminCtx, &application.ApplicationResourceRequest{Name: pointer.String("test"), ResourceName: pointer.String("test"), Group: pointer.String("apps"), Kind: pointer.String("Deployment"), Namespace: pointer.String("test")})
		assert.NoError(t, err)
		_, err = appServer.ListResourceLinks(noRoleCtx, &application.ApplicationResourceRequest{Name: pointer.String("test"), ResourceName: pointer.String("test"), Group: pointer.String("apps"), Kind: pointer.String("Deployment"), Namespace: pointer.String("test")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.ListResourceLinks(adminCtx, &application.ApplicationResourceRequest{Name: pointer.String("does-not-exist"), ResourceName: pointer.String("test"), Group: pointer.String("apps"), Kind: pointer.String("Deployment"), Namespace: pointer.String("test")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
	})

	// Do this last so other stuff doesn't fail.
	t.Run("Delete", func(t *testing.T) {
		_, err := appServer.Delete(adminCtx, &application.ApplicationDeleteRequest{Name: pointer.String("test")})
		assert.NoError(t, err)
		_, err = appServer.Delete(noRoleCtx, &application.ApplicationDeleteRequest{Name: pointer.String("test")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.Delete(adminCtx, &application.ApplicationDeleteRequest{Name: pointer.String("doest-not-exist")})
		assert.Equal(t, permissionDeniedErr.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
	})
}

// setSyncRunningOperationState simulates starting a sync operation on the given app.
func setSyncRunningOperationState(t *testing.T, appServer *Server) {
	appIf := appServer.appclientset.ArgoprojV1alpha1().Applications("default")
	app, err := appIf.Get(context.Background(), "test", metav1.GetOptions{})
	require.NoError(t, err)
	// This sets the status that would be set by the controller usually.
	app.Status.OperationState = &appsv1.OperationState{Phase: synccommon.OperationRunning, Operation: appsv1.Operation{Sync: &appsv1.SyncOperation{}}}
	_, err = appIf.Update(context.Background(), app, metav1.UpdateOptions{})
	require.NoError(t, err)
}

// unsetSyncRunningOperationState simulates finishing a sync operation on the given app.
func unsetSyncRunningOperationState(t *testing.T, appServer *Server) {
	appIf := appServer.appclientset.ArgoprojV1alpha1().Applications("default")
	app, err := appIf.Get(context.Background(), "test", metav1.GetOptions{})
	require.NoError(t, err)
	app.Operation = nil
	app.Status.OperationState = nil
	_, err = appIf.Update(context.Background(), app, metav1.UpdateOptions{})
	require.NoError(t, err)
}

func TestListAppsInNamespaceWithLabels(t *testing.T) {
	appServer := newTestAppServer(t, newTestApp(func(app *appsv1.Application) {
		app.Name = "App1"
		app.ObjectMeta.Namespace = "test-namespace"
		app.SetLabels(map[string]string{"key1": "value1", "key2": "value1"})
	}), newTestApp(func(app *appsv1.Application) {
		app.Name = "App2"
		app.ObjectMeta.Namespace = "test-namespace"
		app.SetLabels(map[string]string{"key1": "value2"})
	}), newTestApp(func(app *appsv1.Application) {
		app.Name = "App3"
		app.ObjectMeta.Namespace = "test-namespace"
		app.SetLabels(map[string]string{"key1": "value3"})
	}))
	appServer.ns = "test-namespace"
	appQuery := application.ApplicationQuery{}
	namespace := "test-namespace"
	appQuery.AppNamespace = &namespace
	testListAppsWithLabels(t, appQuery, appServer)
}

func TestListAppsInDefaultNSWithLabels(t *testing.T) {
	appServer := newTestAppServer(t, newTestApp(func(app *appsv1.Application) {
		app.Name = "App1"
		app.SetLabels(map[string]string{"key1": "value1", "key2": "value1"})
	}), newTestApp(func(app *appsv1.Application) {
		app.Name = "App2"
		app.SetLabels(map[string]string{"key1": "value2"})
	}), newTestApp(func(app *appsv1.Application) {
		app.Name = "App3"
		app.SetLabels(map[string]string{"key1": "value3"})
	}))
	appQuery := application.ApplicationQuery{}
	testListAppsWithLabels(t, appQuery, appServer)
}

func testListAppsWithLabels(t *testing.T, appQuery application.ApplicationQuery, appServer *Server) {
	validTests := []struct {
		testName       string
		label          string
		expectedResult []string
	}{
		{testName: "Equality based filtering using '=' operator",
			label:          "key1=value1",
			expectedResult: []string{"App1"}},
		{testName: "Equality based filtering using '==' operator",
			label:          "key1==value1",
			expectedResult: []string{"App1"}},
		{testName: "Equality based filtering using '!=' operator",
			label:          "key1!=value1",
			expectedResult: []string{"App2", "App3"}},
		{testName: "Set based filtering using 'in' operator",
			label:          "key1 in (value1, value3)",
			expectedResult: []string{"App1", "App3"}},
		{testName: "Set based filtering using 'notin' operator",
			label:          "key1 notin (value1, value3)",
			expectedResult: []string{"App2"}},
		{testName: "Set based filtering using 'exists' operator",
			label:          "key1",
			expectedResult: []string{"App1", "App2", "App3"}},
		{testName: "Set based filtering using 'not exists' operator",
			label:          "!key2",
			expectedResult: []string{"App2", "App3"}},
	}
	//test valid scenarios
	for _, validTest := range validTests {
		t.Run(validTest.testName, func(t *testing.T) {
			appQuery.Selector = &validTest.label
			res, err := appServer.List(context.Background(), &appQuery)
			assert.NoError(t, err)
			apps := []string{}
			for i := range res.Items {
				apps = append(apps, res.Items[i].Name)
			}
			assert.Equal(t, validTest.expectedResult, apps)
		})
	}

	invalidTests := []struct {
		testName    string
		label       string
		errorMesage string
	}{
		{testName: "Set based filtering using '>' operator",
			label:       "key1>value1",
			errorMesage: "error parsing the selector"},
		{testName: "Set based filtering using '<' operator",
			label:       "key1<value1",
			errorMesage: "error parsing the selector"},
	}
	//test invalid scenarios
	for _, invalidTest := range invalidTests {
		t.Run(invalidTest.testName, func(t *testing.T) {
			appQuery.Selector = &invalidTest.label
			_, err := appServer.List(context.Background(), &appQuery)
			assert.ErrorContains(t, err, invalidTest.errorMesage)
		})
	}
}

func TestListAppWithProjects(t *testing.T) {
	appServer := newTestAppServer(t, newTestApp(func(app *appsv1.Application) {
		app.Name = "App1"
		app.Spec.Project = "test-project1"
	}), newTestApp(func(app *appsv1.Application) {
		app.Name = "App2"
		app.Spec.Project = "test-project2"
	}), newTestApp(func(app *appsv1.Application) {
		app.Name = "App3"
		app.Spec.Project = "test-project3"
	}))

	t.Run("List all apps", func(t *testing.T) {
		appQuery := application.ApplicationQuery{}
		appList, err := appServer.List(context.Background(), &appQuery)
		assert.NoError(t, err)
		assert.Len(t, appList.Items, 3)
	})

	t.Run("List apps with projects filter set", func(t *testing.T) {
		appQuery := application.ApplicationQuery{Projects: []string{"test-project1"}}
		appList, err := appServer.List(context.Background(), &appQuery)
		assert.NoError(t, err)
		assert.Len(t, appList.Items, 1)
		for _, app := range appList.Items {
			assert.Equal(t, "test-project1", app.Spec.Project)
		}
	})

	t.Run("List apps with project filter set (legacy field)", func(t *testing.T) {
		appQuery := application.ApplicationQuery{Project: []string{"test-project1"}}
		appList, err := appServer.List(context.Background(), &appQuery)
		assert.NoError(t, err)
		assert.Len(t, appList.Items, 1)
		for _, app := range appList.Items {
			assert.Equal(t, "test-project1", app.Spec.Project)
		}
	})

	t.Run("List apps with both projects and project filter set", func(t *testing.T) {
		// If the older field is present, we should use it instead of the newer field.
		appQuery := application.ApplicationQuery{Project: []string{"test-project1"}, Projects: []string{"test-project2"}}
		appList, err := appServer.List(context.Background(), &appQuery)
		assert.NoError(t, err)
		assert.Len(t, appList.Items, 1)
		for _, app := range appList.Items {
			assert.Equal(t, "test-project1", app.Spec.Project)
		}
	})
}

func TestListApps(t *testing.T) {
	appServer := newTestAppServer(t, newTestApp(func(app *appsv1.Application) {
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
	appServer := newTestAppServerWithEnforcerConfigure(f, t, objects...)

	res, err := appServer.List(ctx, &application.ApplicationQuery{})

	assert.NoError(t, err)
	var names []string
	for i := range res.Items {
		names = append(names, res.Items[i].Name)
	}
	assert.Equal(t, 300, len(names))
}

func generateTestApp(num int) []*appsv1.Application {
	apps := []*appsv1.Application{}
	for i := 0; i < num; i++ {
		apps = append(apps, newTestApp(func(app *appsv1.Application) {
			app.Name = fmt.Sprintf("test-app%.6d", i)
		}))
	}

	return apps
}

func BenchmarkListMuchApps(b *testing.B) {
	// 10000 apps
	apps := generateTestApp(10000)
	obj := make([]runtime.Object, len(apps))
	for i, v := range apps {
		obj[i] = v
	}
	appServer := newTestAppServerWithBenchmark(b, obj...)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_, err := appServer.List(context.Background(), &application.ApplicationQuery{})
		if err != nil {
			break
		}
	}
}

func BenchmarkListSomeApps(b *testing.B) {
	// 500 apps
	apps := generateTestApp(500)
	obj := make([]runtime.Object, len(apps))
	for i, v := range apps {
		obj[i] = v
	}
	appServer := newTestAppServerWithBenchmark(b, obj...)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_, err := appServer.List(context.Background(), &application.ApplicationQuery{})
		if err != nil {
			break
		}
	}
}

func BenchmarkListFewApps(b *testing.B) {
	// 10 apps
	apps := generateTestApp(10)
	obj := make([]runtime.Object, len(apps))
	for i, v := range apps {
		obj[i] = v
	}
	appServer := newTestAppServerWithBenchmark(b, obj...)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_, err := appServer.List(context.Background(), &application.ApplicationQuery{})
		if err != nil {
			break
		}
	}
}

func strToPtr(v string) *string {
	return &v
}

func BenchmarkListMuchAppsWithName(b *testing.B) {
	// 10000 apps
	appsMuch := generateTestApp(10000)
	obj := make([]runtime.Object, len(appsMuch))
	for i, v := range appsMuch {
		obj[i] = v
	}
	appServer := newTestAppServerWithBenchmark(b, obj...)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		app := &application.ApplicationQuery{Name: strToPtr("test-app000099")}
		_, err := appServer.List(context.Background(), app)
		if err != nil {
			break
		}
	}
}

func BenchmarkListMuchAppsWithProjects(b *testing.B) {
	// 10000 apps
	appsMuch := generateTestApp(10000)
	appsMuch[999].Spec.Project = "test-project1"
	appsMuch[1999].Spec.Project = "test-project2"
	obj := make([]runtime.Object, len(appsMuch))
	for i, v := range appsMuch {
		obj[i] = v
	}
	appServer := newTestAppServerWithBenchmark(b, obj...)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		app := &application.ApplicationQuery{Project: []string{"test-project1", "test-project2"}}
		_, err := appServer.List(context.Background(), app)
		if err != nil {
			break
		}
	}
}

func BenchmarkListMuchAppsWithRepo(b *testing.B) {
	// 10000 apps
	appsMuch := generateTestApp(10000)
	appsMuch[999].Spec.Source.RepoURL = "https://some-fake-source"
	obj := make([]runtime.Object, len(appsMuch))
	for i, v := range appsMuch {
		obj[i] = v
	}
	appServer := newTestAppServerWithBenchmark(b, obj...)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		app := &application.ApplicationQuery{Repo: strToPtr("https://some-fake-source")}
		_, err := appServer.List(context.Background(), app)
		if err != nil {
			break
		}
	}
}

func TestCreateApp(t *testing.T) {
	testApp := newTestApp()
	appServer := newTestAppServer(t)
	testApp.Spec.Project = ""
	createReq := application.ApplicationCreateRequest{
		Application: testApp,
	}
	app, err := appServer.Create(context.Background(), &createReq)
	assert.NoError(t, err)
	assert.NotNil(t, app)
	assert.NotNil(t, app.Spec)
	assert.Equal(t, app.Spec.Project, "default")
}

func TestCreateAppWithDestName(t *testing.T) {
	appServer := newTestAppServer(t)
	testApp := newTestAppWithDestName()
	createReq := application.ApplicationCreateRequest{
		Application: testApp,
	}
	app, err := appServer.Create(context.Background(), &createReq)
	assert.NoError(t, err)
	assert.NotNil(t, app)
	assert.Equal(t, app.Spec.Destination.Server, "https://cluster-api.com")
}

func TestUpdateApp(t *testing.T) {
	testApp := newTestApp()
	appServer := newTestAppServer(t, testApp)
	testApp.Spec.Project = ""
	app, err := appServer.Update(context.Background(), &application.ApplicationUpdateRequest{
		Application: testApp,
	})
	assert.Nil(t, err)
	assert.Equal(t, app.Spec.Project, "default")
}

func TestUpdateAppSpec(t *testing.T) {
	testApp := newTestApp()
	appServer := newTestAppServer(t, testApp)
	testApp.Spec.Project = ""
	spec, err := appServer.UpdateSpec(context.Background(), &application.ApplicationUpdateSpecRequest{
		Name: &testApp.Name,
		Spec: &testApp.Spec,
	})
	assert.NoError(t, err)
	assert.Equal(t, "default", spec.Project)
	app, err := appServer.Get(context.Background(), &application.ApplicationQuery{Name: &testApp.Name})
	assert.NoError(t, err)
	assert.Equal(t, "default", app.Spec.Project)
}

func TestDeleteApp(t *testing.T) {
	ctx := context.Background()
	appServer := newTestAppServer(t)
	createReq := application.ApplicationCreateRequest{
		Application: newTestApp(),
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
	fakeAppCs.AddReactor("get", "applications", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &appsv1.Application{Spec: appsv1.ApplicationSpec{Source: &appsv1.ApplicationSource{}}}, nil
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

func TestSyncAndTerminate(t *testing.T) {
	ctx := context.Background()
	appServer := newTestAppServer(t)
	testApp := newTestApp()
	testApp.Spec.Source.RepoURL = "https://github.com/argoproj/argo-cd.git"
	createReq := application.ApplicationCreateRequest{
		Application: testApp,
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
	appServer := newTestAppServer(t)
	testApp := newTestApp()
	testApp.Spec.Source.RepoURL = "https://argoproj.github.io/argo-helm"
	testApp.Spec.Source.Path = ""
	testApp.Spec.Source.Chart = "argo-cd"
	testApp.Spec.Source.TargetRevision = "0.7.*"

	appServer.repoClientset = &mocks.Clientset{RepoServerServiceClient: fakeRepoServerClient(true)}

	app, err := appServer.Create(ctx, &application.ApplicationCreateRequest{Application: testApp})
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
	appServer := newTestAppServer(t)
	testApp := newTestApp()
	testApp.Spec.Source.RepoURL = "https://github.com/org/test"
	testApp.Spec.Source.Path = "deploy"
	testApp.Spec.Source.TargetRevision = "0.7.*"
	app, err := appServer.Create(ctx, &application.ApplicationCreateRequest{Application: testApp})
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
	appServer := newTestAppServer(t, testApp)

	updatedApp, err := appServer.Rollback(context.Background(), &application.ApplicationRollbackRequest{
		Name: &testApp.Name,
		Id:   pointer.Int64(1),
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
	appServer := newTestAppServer(t, testApp)
	appServer.enf.SetDefaultRole("")

	t.Run("update without changing project", func(t *testing.T) {
		_ = appServer.enf.SetBuiltinPolicy(`p, admin, applications, update, default/test-app, allow`)
		_, err := appServer.Update(ctx, &application.ApplicationUpdateRequest{Application: testApp})
		assert.NoError(t, err)
	})

	t.Run("cannot update to another project", func(t *testing.T) {
		testApp.Spec.Project = "my-proj"
		_, err := appServer.Update(ctx, &application.ApplicationUpdateRequest{Application: testApp})
		assert.Equal(t, status.Code(err), codes.PermissionDenied)
	})

	t.Run("cannot change projects without create privileges", func(t *testing.T) {
		_ = appServer.enf.SetBuiltinPolicy(`
p, admin, applications, update, default/test-app, allow
p, admin, applications, update, my-proj/test-app, allow
`)
		_, err := appServer.Update(ctx, &application.ApplicationUpdateRequest{Application: testApp})
		statusErr := grpc.UnwrapGRPCStatus(err)
		assert.NotNil(t, statusErr)
		assert.Equal(t, codes.PermissionDenied, statusErr.Code())
	})

	t.Run("cannot change projects without update privileges in new project", func(t *testing.T) {
		_ = appServer.enf.SetBuiltinPolicy(`
p, admin, applications, update, default/test-app, allow
p, admin, applications, create, my-proj/test-app, allow
`)
		_, err := appServer.Update(ctx, &application.ApplicationUpdateRequest{Application: testApp})
		assert.Equal(t, status.Code(err), codes.PermissionDenied)
	})

	t.Run("cannot change projects without update privileges in old project", func(t *testing.T) {
		_ = appServer.enf.SetBuiltinPolicy(`
p, admin, applications, create, my-proj/test-app, allow
p, admin, applications, update, my-proj/test-app, allow
`)
		_, err := appServer.Update(ctx, &application.ApplicationUpdateRequest{Application: testApp})
		statusErr := grpc.UnwrapGRPCStatus(err)
		assert.NotNil(t, statusErr)
		assert.Equal(t, codes.PermissionDenied, statusErr.Code())
	})

	t.Run("can update project with proper permissions", func(t *testing.T) {
		// Verify can update project with proper permissions
		_ = appServer.enf.SetBuiltinPolicy(`
p, admin, applications, update, default/test-app, allow
p, admin, applications, create, my-proj/test-app, allow
p, admin, applications, update, my-proj/test-app, allow
`)
		updatedApp, err := appServer.Update(ctx, &application.ApplicationUpdateRequest{Application: testApp})
		assert.NoError(t, err)
		assert.Equal(t, "my-proj", updatedApp.Spec.Project)
	})
}

func TestAppJsonPatch(t *testing.T) {
	testApp := newTestAppWithAnnotations()
	ctx := context.Background()
	// nolint:staticcheck
	ctx = context.WithValue(ctx, "claims", &jwt.StandardClaims{Subject: "admin"})
	appServer := newTestAppServer(t, testApp)
	appServer.enf.SetDefaultRole("")

	app, err := appServer.Patch(ctx, &application.ApplicationPatchRequest{Name: &testApp.Name, Patch: pointer.String("garbage")})
	assert.Error(t, err)
	assert.Nil(t, app)

	app, err = appServer.Patch(ctx, &application.ApplicationPatchRequest{Name: &testApp.Name, Patch: pointer.String("[]")})
	assert.NoError(t, err)
	assert.NotNil(t, app)

	app, err = appServer.Patch(ctx, &application.ApplicationPatchRequest{Name: &testApp.Name, Patch: pointer.String(`[{"op": "replace", "path": "/spec/source/path", "value": "foo"}]`)})
	assert.NoError(t, err)
	assert.Equal(t, "foo", app.Spec.Source.Path)

	app, err = appServer.Patch(ctx, &application.ApplicationPatchRequest{Name: &testApp.Name, Patch: pointer.String(`[{"op": "remove", "path": "/metadata/annotations/test.annotation"}]`)})
	assert.NoError(t, err)
	assert.NotContains(t, app.Annotations, "test.annotation")
}

func TestAppMergePatch(t *testing.T) {
	testApp := newTestApp()
	ctx := context.Background()
	// nolint:staticcheck
	ctx = context.WithValue(ctx, "claims", &jwt.StandardClaims{Subject: "admin"})
	appServer := newTestAppServer(t, testApp)
	appServer.enf.SetDefaultRole("")

	app, err := appServer.Patch(ctx, &application.ApplicationPatchRequest{
		Name: &testApp.Name, Patch: pointer.String(`{"spec": { "source": { "path": "foo" } }}`), PatchType: pointer.String("merge")})
	assert.NoError(t, err)
	assert.Equal(t, "foo", app.Spec.Source.Path)
}

func TestServer_GetApplicationSyncWindowsState(t *testing.T) {
	t.Run("Active", func(t *testing.T) {
		testApp := newTestApp()
		testApp.Spec.Project = "proj-maint"
		appServer := newTestAppServer(t, testApp)

		active, err := appServer.GetApplicationSyncWindows(context.Background(), &application.ApplicationSyncWindowsQuery{Name: &testApp.Name})
		assert.NoError(t, err)
		assert.Equal(t, 1, len(active.ActiveWindows))
	})
	t.Run("Inactive", func(t *testing.T) {
		testApp := newTestApp()
		testApp.Spec.Project = "default"
		appServer := newTestAppServer(t, testApp)

		active, err := appServer.GetApplicationSyncWindows(context.Background(), &application.ApplicationSyncWindowsQuery{Name: &testApp.Name})
		assert.NoError(t, err)
		assert.Equal(t, 0, len(active.ActiveWindows))
	})
	t.Run("ProjectDoesNotExist", func(t *testing.T) {
		testApp := newTestApp()
		testApp.Spec.Project = "none"
		appServer := newTestAppServer(t, testApp)

		active, err := appServer.GetApplicationSyncWindows(context.Background(), &application.ApplicationSyncWindowsQuery{Name: &testApp.Name})
		assert.Contains(t, err.Error(), "not found")
		assert.Nil(t, active)
	})
}

func TestGetCachedAppState(t *testing.T) {
	testApp := newTestApp()
	testApp.ObjectMeta.ResourceVersion = "1"
	testApp.Spec.Project = "test-proj"
	testProj := &appsv1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-proj",
			Namespace: testNamespace,
		},
	}
	appServer := newTestAppServer(t, testApp, testProj)
	fakeClientSet := appServer.appclientset.(*apps.Clientset)
	fakeClientSet.AddReactor("get", "applications", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &appsv1.Application{Spec: appsv1.ApplicationSpec{Source: &appsv1.ApplicationSource{}}}, nil
	})
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
			fakeClientSet.AddReactor("get", "applications", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, &appsv1.Application{Spec: appsv1.ApplicationSpec{Source: &appsv1.ApplicationSource{}}}, nil
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
		aName, appNs := argo.ParseAppQualifiedName(appName, appServer.ns)
		a, err := appServer.appLister.Applications(appNs).Get(aName)
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
	appServer := newTestAppServer(t, testApp)

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
	appServer := newTestAppServer(t, testApp)

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
	assert.Equal(t, testApp.Spec.Source, getAppDetailsQuery.Source)

	assert.NoError(t, err)
	select {
	case <-ch:
		assert.Equal(t, atomic.LoadInt32(&patched), int32(1))
	case <-time.After(10 * time.Second):
		assert.Fail(t, "Out of time ( 10 seconds )")
	}
}

func TestInferResourcesStatusHealth(t *testing.T) {
	cacheClient := cacheutil.NewCache(cacheutil.NewInMemoryCache(1 * time.Hour))

	testApp := newTestApp()
	testApp.Status.ResourceHealthSource = appsv1.ResourceHealthLocationAppTree
	testApp.Status.Resources = []appsv1.ResourceStatus{{
		Group:     "apps",
		Kind:      "Deployment",
		Name:      "guestbook",
		Namespace: "default",
	}, {
		Group:     "apps",
		Kind:      "StatefulSet",
		Name:      "guestbook-stateful",
		Namespace: "default",
	}}
	appServer := newTestAppServer(t, testApp)
	appStateCache := appstate.NewCache(cacheClient, time.Minute)
	err := appStateCache.SetAppResourcesTree(testApp.Name, &appsv1.ApplicationTree{Nodes: []appsv1.ResourceNode{{
		ResourceRef: appsv1.ResourceRef{
			Group:     "apps",
			Kind:      "Deployment",
			Name:      "guestbook",
			Namespace: "default",
		},
		Health: &appsv1.HealthStatus{
			Status: health.HealthStatusDegraded,
		},
	}}})

	require.NoError(t, err)

	appServer.cache = servercache.NewCache(appStateCache, time.Minute, time.Minute, time.Minute)

	appServer.inferResourcesStatusHealth(testApp)

	assert.Equal(t, health.HealthStatusDegraded, testApp.Status.Resources[0].Health.Status)
	assert.Nil(t, testApp.Status.Resources[1].Health)
}
