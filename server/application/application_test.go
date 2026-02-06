package application

import (
	"context"
	stderrors "errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/argoproj/gitops-engine/pkg/health"
	synccommon "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/gitops-engine/pkg/utils/kube/kubetest"
	"github.com/argoproj/pkg/v2/sync"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	appsv1 "k8s.io/api/apps/v1"
	k8sbatchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	kubetesting "k8s.io/client-go/testing"
	k8scache "k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	apps "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/fake"
	appinformer "github.com/argoproj/argo-cd/v3/pkg/client/informers/externalversions"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient/mocks"
	servercache "github.com/argoproj/argo-cd/v3/server/cache"
	"github.com/argoproj/argo-cd/v3/server/rbacpolicy"
	"github.com/argoproj/argo-cd/v3/test"
	"github.com/argoproj/argo-cd/v3/util/argo"
	"github.com/argoproj/argo-cd/v3/util/assets"
	"github.com/argoproj/argo-cd/v3/util/cache"
	"github.com/argoproj/argo-cd/v3/util/cache/appstate"
	"github.com/argoproj/argo-cd/v3/util/db"
	"github.com/argoproj/argo-cd/v3/util/grpc"
	"github.com/argoproj/argo-cd/v3/util/rbac"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

const (
	testNamespace = "default"
	fakeRepoURL   = "https://git.com/repo.git"
)

var testEnableEventList []string = argo.DefaultEnableEventList()

type broadcasterMock struct {
	objects []runtime.Object
}

func (b broadcasterMock) Subscribe(ch chan *v1alpha1.ApplicationWatchEvent, _ ...func(event *v1alpha1.ApplicationWatchEvent) bool) func() {
	// Simulate the broadcaster notifying the subscriber of an application update.
	// The second parameter to Subscribe is filters. For the purposes of tests, we ignore the filters. Future tests
	// might require implementing those.
	go func() {
		for _, obj := range b.objects {
			app, ok := obj.(*v1alpha1.Application)
			if ok {
				oldVersion, err := strconv.Atoi(app.ResourceVersion)
				if err != nil {
					oldVersion = 0
				}
				clonedApp := app.DeepCopy()
				clonedApp.ResourceVersion = strconv.Itoa(oldVersion + 1)
				ch <- &v1alpha1.ApplicationWatchEvent{Type: watch.Added, Application: *clonedApp}
			}
		}
	}()
	return func() {}
}

func (broadcasterMock) OnAdd(any, bool)   {}
func (broadcasterMock) OnUpdate(any, any) {}
func (broadcasterMock) OnDelete(any)      {}

func fakeRepo() *v1alpha1.Repository {
	return &v1alpha1.Repository{
		Repo: fakeRepoURL,
	}
}

func fakeCluster() *v1alpha1.Cluster {
	return &v1alpha1.Cluster{
		Server: "https://cluster-api.example.com",
		Name:   "fake-cluster",
		Config: v1alpha1.ClusterConfig{},
	}
}

func fakeResolveRevisionResponse() *apiclient.ResolveRevisionResponse {
	return &apiclient.ResolveRevisionResponse{
		Revision:          "f9ba9e98119bf8c1176fbd65dbae26a71d044add",
		AmbiguousRevision: "HEAD (f9ba9e98119bf8c1176fbd65dbae26a71d044add)",
	}
}

func fakeResolveRevisionResponseHelm() *apiclient.ResolveRevisionResponse {
	return &apiclient.ResolveRevisionResponse{
		Revision:          "0.7.*",
		AmbiguousRevision: "0.7.* (0.7.2)",
	}
}

func fakeRepoServerClient(isHelm bool) *mocks.RepoServerServiceClient {
	mockRepoServiceClient := &mocks.RepoServerServiceClient{}
	mockRepoServiceClient.EXPECT().GenerateManifest(mock.Anything, mock.Anything).Return(&apiclient.ManifestResponse{}, nil)
	mockRepoServiceClient.EXPECT().GetAppDetails(mock.Anything, mock.Anything).Return(&apiclient.RepoAppDetailsResponse{}, nil)
	mockRepoServiceClient.EXPECT().TestRepository(mock.Anything, mock.Anything).Return(&apiclient.TestRepositoryResponse{}, nil)
	mockRepoServiceClient.EXPECT().GetRevisionMetadata(mock.Anything, mock.Anything).Return(&v1alpha1.RevisionMetadata{}, nil)
	mockWithFilesClient := &mocks.RepoServerService_GenerateManifestWithFilesClient{}
	mockWithFilesClient.EXPECT().Send(mock.Anything).Return(nil).Maybe()
	mockWithFilesClient.EXPECT().CloseAndRecv().Return(&apiclient.ManifestResponse{}, nil).Maybe()
	mockRepoServiceClient.EXPECT().GenerateManifestWithFiles(mock.Anything, mock.Anything).Return(mockWithFilesClient, nil)
	mockRepoServiceClient.EXPECT().GetRevisionChartDetails(mock.Anything, mock.Anything).Return(&v1alpha1.ChartDetails{}, nil)

	if isHelm {
		mockRepoServiceClient.EXPECT().ResolveRevision(mock.Anything, mock.Anything).Return(fakeResolveRevisionResponseHelm(), nil)
	} else {
		mockRepoServiceClient.EXPECT().ResolveRevision(mock.Anything, mock.Anything).Return(fakeResolveRevisionResponse(), nil)
	}

	return mockRepoServiceClient
}

// return an ApplicationServiceServer which returns fake data
func newTestAppServer(t *testing.T, objects ...runtime.Object) *Server {
	t.Helper()
	f := func(enf *rbac.Enforcer) {
		_ = enf.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
		enf.SetDefaultRole("role:admin")
	}
	return newTestAppServerWithEnforcerConfigure(t, f, map[string]string{}, objects...)
}

func newTestAppServerWithEnforcerConfigure(t *testing.T, f func(*rbac.Enforcer), additionalConfig map[string]string, objects ...runtime.Object) *Server {
	t.Helper()
	kubeclientset := fake.NewClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      "argocd-cm",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: additionalConfig,
	}, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-secret",
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"admin.password":   []byte("test"),
			"server.secretkey": []byte("test"),
		},
	})
	ctx := t.Context()
	db := db.NewDB(testNamespace, settings.NewSettingsManager(ctx, kubeclientset, testNamespace), kubeclientset)
	_, err := db.CreateRepository(ctx, fakeRepo())
	require.NoError(t, err)
	_, err = db.CreateCluster(ctx, fakeCluster())
	require.NoError(t, err)

	mockRepoClient := &mocks.Clientset{RepoServerServiceClient: fakeRepoServerClient(false)}

	defaultProj := &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "default"},
		Spec: v1alpha1.AppProjectSpec{
			SourceRepos:  []string{"*"},
			Destinations: []v1alpha1.ApplicationDestination{{Server: "*", Namespace: "*"}},
		},
	}

	myProj := &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: "my-proj", Namespace: "default"},
		Spec: v1alpha1.AppProjectSpec{
			SourceRepos:  []string{"*"},
			Destinations: []v1alpha1.ApplicationDestination{{Server: "*", Namespace: "*"}},
		},
	}
	projWithSyncWindows := &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: "proj-maint", Namespace: "default"},
		Spec: v1alpha1.AppProjectSpec{
			SourceRepos:  []string{"*"},
			Destinations: []v1alpha1.ApplicationDestination{{Server: "*", Namespace: "*"}},
			SyncWindows:  v1alpha1.SyncWindows{},
		},
	}
	matchingWindow := &v1alpha1.SyncWindow{
		Kind:         "allow",
		Schedule:     "* * * * *",
		Duration:     "1h",
		Applications: []string{"test-app"},
	}
	projWithSyncWindows.Spec.SyncWindows = append(projWithSyncWindows.Spec.SyncWindows, matchingWindow)

	objects = append(objects, defaultProj, myProj, projWithSyncWindows)

	fakeAppsClientset := apps.NewSimpleClientset(objects...)
	factory := appinformer.NewSharedInformerFactoryWithOptions(fakeAppsClientset, 0, appinformer.WithNamespace(""), appinformer.WithTweakListOptions(func(_ *metav1.ListOptions) {}))
	fakeProjLister := factory.Argoproj().V1alpha1().AppProjects().Lister().AppProjects(testNamespace)

	enforcer := rbac.NewEnforcer(kubeclientset, testNamespace, common.ArgoCDRBACConfigMapName, nil)
	f(enforcer)
	enforcer.SetClaimsEnforcerFunc(rbacpolicy.NewRBACPolicyEnforcer(enforcer, fakeProjLister).EnforceClaims)

	settingsMgr := settings.NewSettingsManager(ctx, kubeclientset, testNamespace)

	// populate the app informer with the fake objects
	appInformer := factory.Argoproj().V1alpha1().Applications().Informer()
	// TODO(jessesuen): probably should return cancel function so tests can stop background informer
	// ctx, cancel := context.WithCancel(t.Context())
	go appInformer.Run(ctx.Done())
	if !k8scache.WaitForCacheSync(ctx.Done(), appInformer.HasSynced) {
		panic("Timed out waiting for caches to sync")
	}

	projInformer := factory.Argoproj().V1alpha1().AppProjects().Informer()
	go projInformer.Run(ctx.Done())
	if !k8scache.WaitForCacheSync(ctx.Done(), projInformer.HasSynced) {
		panic("Timed out waiting for caches to sync")
	}

	broadcaster := broadcasterMock{
		objects: objects,
	}

	appStateCache := appstate.NewCache(cache.NewCache(cache.NewInMemoryCache(time.Hour)), time.Hour)
	// pre-populate the app cache
	for _, obj := range objects {
		app, ok := obj.(*v1alpha1.Application)
		if ok {
			err := appStateCache.SetAppManagedResources(app.Name, []*v1alpha1.ResourceDiff{})
			require.NoError(t, err)

			// Pre-populate the resource tree based on the app's resources.
			nodes := make([]v1alpha1.ResourceNode, len(app.Status.Resources))
			for i, res := range app.Status.Resources {
				nodes[i] = v1alpha1.ResourceNode{
					ResourceRef: v1alpha1.ResourceRef{
						Group:     res.Group,
						Kind:      res.Kind,
						Version:   res.Version,
						Name:      res.Name,
						Namespace: res.Namespace,
						UID:       "fake",
					},
				}
			}
			err = appStateCache.SetAppResourcesTree(app.Name, &v1alpha1.ApplicationTree{
				Nodes: nodes,
			})
			require.NoError(t, err)
		}
	}
	appCache := servercache.NewCache(appStateCache, time.Hour, time.Hour)

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
		testEnableEventList,
		true,
	)
	return server.(*Server)
}

// return an ApplicationServiceServer which returns fake data
func newTestAppServerWithBenchmark(b *testing.B, objects ...runtime.Object) *Server {
	b.Helper()
	f := func(enf *rbac.Enforcer) {
		_ = enf.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
		enf.SetDefaultRole("role:admin")
	}
	return newTestAppServerWithEnforcerConfigureWithBenchmark(b, f, objects...)
}

func newTestAppServerWithEnforcerConfigureWithBenchmark(b *testing.B, f func(*rbac.Enforcer), objects ...runtime.Object) *Server {
	b.Helper()
	kubeclientset := fake.NewClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      "argocd-cm",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
	}, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-secret",
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"admin.password":   []byte("test"),
			"server.secretkey": []byte("test"),
		},
	})
	ctx := b.Context()
	db := db.NewDB(testNamespace, settings.NewSettingsManager(ctx, kubeclientset, testNamespace), kubeclientset)
	_, err := db.CreateRepository(ctx, fakeRepo())
	require.NoError(b, err)
	_, err = db.CreateCluster(ctx, fakeCluster())
	require.NoError(b, err)

	mockRepoClient := &mocks.Clientset{RepoServerServiceClient: fakeRepoServerClient(false)}

	defaultProj := &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "default"},
		Spec: v1alpha1.AppProjectSpec{
			SourceRepos:  []string{"*"},
			Destinations: []v1alpha1.ApplicationDestination{{Server: "*", Namespace: "*"}},
		},
	}
	myProj := &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: "my-proj", Namespace: "default"},
		Spec: v1alpha1.AppProjectSpec{
			SourceRepos:  []string{"*"},
			Destinations: []v1alpha1.ApplicationDestination{{Server: "*", Namespace: "*"}},
		},
	}
	projWithSyncWindows := &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: "proj-maint", Namespace: "default"},
		Spec: v1alpha1.AppProjectSpec{
			SourceRepos:  []string{"*"},
			Destinations: []v1alpha1.ApplicationDestination{{Server: "*", Namespace: "*"}},
			SyncWindows:  v1alpha1.SyncWindows{},
		},
	}
	matchingWindow := &v1alpha1.SyncWindow{
		Kind:         "allow",
		Schedule:     "* * * * *",
		Duration:     "1h",
		Applications: []string{"test-app"},
	}
	projWithSyncWindows.Spec.SyncWindows = append(projWithSyncWindows.Spec.SyncWindows, matchingWindow)

	objects = append(objects, defaultProj, myProj, projWithSyncWindows)

	fakeAppsClientset := apps.NewSimpleClientset(objects...)
	factory := appinformer.NewSharedInformerFactoryWithOptions(fakeAppsClientset, 0, appinformer.WithNamespace(""), appinformer.WithTweakListOptions(func(_ *metav1.ListOptions) {}))
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

	broadcaster := broadcasterMock{
		objects: objects,
	}

	appStateCache := appstate.NewCache(cache.NewCache(cache.NewInMemoryCache(time.Hour)), time.Hour)
	// pre-populate the app cache
	for _, obj := range objects {
		app, ok := obj.(*v1alpha1.Application)
		if ok {
			err := appStateCache.SetAppManagedResources(app.Name, []*v1alpha1.ResourceDiff{})
			require.NoError(b, err)

			// Pre-populate the resource tree based on the app's resources.
			nodes := make([]v1alpha1.ResourceNode, len(app.Status.Resources))
			for i, res := range app.Status.Resources {
				nodes[i] = v1alpha1.ResourceNode{
					ResourceRef: v1alpha1.ResourceRef{
						Group:     res.Group,
						Kind:      res.Kind,
						Version:   res.Version,
						Name:      res.Name,
						Namespace: res.Namespace,
						UID:       "fake",
					},
				}
			}
			err = appStateCache.SetAppResourcesTree(app.Name, &v1alpha1.ApplicationTree{
				Nodes: nodes,
			})
			require.NoError(b, err)
		}
	}
	appCache := servercache.NewCache(appStateCache, time.Hour, time.Hour)

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
		testEnableEventList,
		true,
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
    server: https://cluster-api.example.com
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
    server: https://cluster-api.example.com
`

func newTestAppWithDestName(opts ...func(app *v1alpha1.Application)) *v1alpha1.Application {
	return createTestApp(fakeAppWithDestName, opts...)
}

func newTestApp(opts ...func(app *v1alpha1.Application)) *v1alpha1.Application {
	return createTestApp(fakeApp, opts...)
}

func newMultiSourceTestApp(opts ...func(app *v1alpha1.Application)) *v1alpha1.Application {
	multiSourceApp := newTestApp(opts...)
	multiSourceApp.Name = "multi-source-app"
	multiSourceApp.Spec = v1alpha1.ApplicationSpec{
		Sources: []v1alpha1.ApplicationSource{
			{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				Path:           "helm-guestbook",
				TargetRevision: "appbranch1",
			},
			{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				Path:           "kustomize-guestbook",
				TargetRevision: "appbranch2",
			},
		},
		Destination: v1alpha1.ApplicationDestination{
			Server:    "https://cluster-api.example.com",
			Namespace: test.FakeDestNamespace,
		},
	}
	return multiSourceApp
}

func newTestAppWithAnnotations(opts ...func(app *v1alpha1.Application)) *v1alpha1.Application {
	return createTestApp(fakeAppWithAnnotations, opts...)
}

func createTestApp(testApp string, opts ...func(app *v1alpha1.Application)) *v1alpha1.Application {
	var app v1alpha1.Application
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
	project    string
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

func (t *TestServerStream) SendMsg(_ any) error {
	return nil
}

func (t *TestServerStream) RecvMsg(_ any) error {
	return nil
}

func (t *TestServerStream) SendAndClose(_ *apiclient.ManifestResponse) error {
	return nil
}

func (t *TestServerStream) Recv() (*application.ApplicationManifestQueryWithFilesWrapper, error) {
	if !t.headerSent {
		t.headerSent = true
		return &application.ApplicationManifestQueryWithFilesWrapper{Part: &application.ApplicationManifestQueryWithFilesWrapper_Query{
			Query: &application.ApplicationManifestQueryWithFiles{
				Name:     ptr.To(t.appName),
				Project:  ptr.To(t.project),
				Checksum: ptr.To(""),
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

func (t *TestResourceTreeServer) Send(_ *v1alpha1.ApplicationTree) error {
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

func (t *TestResourceTreeServer) SendMsg(_ any) error {
	return nil
}

func (t *TestResourceTreeServer) RecvMsg(_ any) error {
	return nil
}

type TestPodLogsServer struct {
	ctx context.Context
}

func (t *TestPodLogsServer) Send(_ *application.LogEntry) error {
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

func (t *TestPodLogsServer) SendMsg(_ any) error {
	return nil
}

func (t *TestPodLogsServer) RecvMsg(_ any) error {
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
	deployment := appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
	}
	testApp := newTestApp(func(app *v1alpha1.Application) {
		app.Name = "test"
		app.Status.Resources = []v1alpha1.ResourceStatus{
			{
				Group:     deployment.GroupVersionKind().Group,
				Kind:      deployment.GroupVersionKind().Kind,
				Version:   deployment.GroupVersionKind().Version,
				Name:      deployment.Name,
				Namespace: deployment.Namespace,
				Status:    "Synced",
			},
		}
		app.Status.History = []v1alpha1.RevisionHistory{
			{
				ID: 0,
				Source: v1alpha1.ApplicationSource{
					TargetRevision: "something-old",
				},
			},
		}
	})
	testHelmApp := newTestApp(func(app *v1alpha1.Application) {
		app.Name = "test-helm"
		app.Spec.Source.Path = ""
		app.Spec.Source.Chart = "test"
		app.Status.Resources = []v1alpha1.ResourceStatus{
			{
				Group:     deployment.GroupVersionKind().Group,
				Kind:      deployment.GroupVersionKind().Kind,
				Version:   deployment.GroupVersionKind().Version,
				Name:      deployment.Name,
				Namespace: deployment.Namespace,
				Status:    "Synced",
			},
		}
		app.Status.History = []v1alpha1.RevisionHistory{
			{
				ID: 0,
				Source: v1alpha1.ApplicationSource{
					TargetRevision: "something-old",
				},
			},
		}
	})
	testAppMulti := newTestApp(func(app *v1alpha1.Application) {
		app.Name = "test-multi"
		app.Spec.Sources = v1alpha1.ApplicationSources{
			v1alpha1.ApplicationSource{
				TargetRevision: "something-old",
			},
			v1alpha1.ApplicationSource{
				TargetRevision: "something-old",
			},
		}
		app.Status.Resources = []v1alpha1.ResourceStatus{
			{
				Group:     deployment.GroupVersionKind().Group,
				Kind:      deployment.GroupVersionKind().Kind,
				Version:   deployment.GroupVersionKind().Version,
				Name:      deployment.Name,
				Namespace: deployment.Namespace,
				Status:    "Synced",
			},
		}
		app.Status.History = []v1alpha1.RevisionHistory{
			{
				ID: 1,
				Sources: v1alpha1.ApplicationSources{
					v1alpha1.ApplicationSource{
						TargetRevision: "something-old",
					},
					v1alpha1.ApplicationSource{
						TargetRevision: "something-old",
					},
				},
			},
		}
	})
	testDeployment := kube.MustToUnstructured(&deployment)
	appServer := newTestAppServerWithEnforcerConfigure(t, f, map[string]string{}, testApp, testHelmApp, testAppMulti, testDeployment)

	noRoleCtx := t.Context()
	//nolint:staticcheck
	adminCtx := context.WithValue(noRoleCtx, "claims", &jwt.MapClaims{"groups": []string{"admin"}})

	t.Run("Get", func(t *testing.T) {
		_, err := appServer.Get(adminCtx, &application.ApplicationQuery{Name: ptr.To("test")})
		require.NoError(t, err)
		_, err = appServer.Get(noRoleCtx, &application.ApplicationQuery{Name: ptr.To("test")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.Get(adminCtx, &application.ApplicationQuery{Name: ptr.To("doest-not-exist")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.Get(adminCtx, &application.ApplicationQuery{Name: ptr.To("doest-not-exist"), Project: []string{"test"}})
		assert.EqualError(t, err, "rpc error: code = NotFound desc = applications.argoproj.io \"doest-not-exist\" not found", "when the request specifies a project, we can return the standard k8s error message")
	})

	t.Run("GetManifests", func(t *testing.T) {
		_, err := appServer.GetManifests(adminCtx, &application.ApplicationManifestQuery{Name: ptr.To("test")})
		require.NoError(t, err)
		_, err = appServer.GetManifests(noRoleCtx, &application.ApplicationManifestQuery{Name: ptr.To("test")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.GetManifests(adminCtx, &application.ApplicationManifestQuery{Name: ptr.To("doest-not-exist")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.GetManifests(adminCtx, &application.ApplicationManifestQuery{Name: ptr.To("doest-not-exist"), Project: ptr.To("test")})
		assert.EqualError(t, err, "rpc error: code = NotFound desc = applications.argoproj.io \"doest-not-exist\" not found", "when the request specifies a project, we can return the standard k8s error message")
	})

	t.Run("ListResourceEvents", func(t *testing.T) {
		_, err := appServer.ListResourceEvents(adminCtx, &application.ApplicationResourceEventsQuery{Name: ptr.To("test")})
		require.NoError(t, err)
		_, err = appServer.ListResourceEvents(noRoleCtx, &application.ApplicationResourceEventsQuery{Name: ptr.To("test")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.ListResourceEvents(adminCtx, &application.ApplicationResourceEventsQuery{Name: ptr.To("doest-not-exist")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.ListResourceEvents(adminCtx, &application.ApplicationResourceEventsQuery{Name: ptr.To("doest-not-exist"), Project: ptr.To("test")})
		assert.EqualError(t, err, "rpc error: code = NotFound desc = applications.argoproj.io \"doest-not-exist\" not found", "when the request specifies a project, we can return the standard k8s error message")
	})

	t.Run("UpdateSpec", func(t *testing.T) {
		_, err := appServer.UpdateSpec(adminCtx, &application.ApplicationUpdateSpecRequest{Name: ptr.To("test"), Spec: &v1alpha1.ApplicationSpec{
			Destination: v1alpha1.ApplicationDestination{Namespace: "default", Server: "https://cluster-api.example.com"},
			Source:      &v1alpha1.ApplicationSource{RepoURL: "https://some-fake-source", Path: "."},
		}})
		require.NoError(t, err)
		_, err = appServer.UpdateSpec(noRoleCtx, &application.ApplicationUpdateSpecRequest{Name: ptr.To("test"), Spec: &v1alpha1.ApplicationSpec{
			Destination: v1alpha1.ApplicationDestination{Namespace: "default", Server: "https://cluster-api.example.com"},
			Source:      &v1alpha1.ApplicationSource{RepoURL: "https://some-fake-source", Path: "."},
		}})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.UpdateSpec(adminCtx, &application.ApplicationUpdateSpecRequest{Name: ptr.To("doest-not-exist"), Spec: &v1alpha1.ApplicationSpec{
			Destination: v1alpha1.ApplicationDestination{Namespace: "default", Server: "https://cluster-api.example.com"},
			Source:      &v1alpha1.ApplicationSource{RepoURL: "https://some-fake-source", Path: "."},
		}})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.UpdateSpec(adminCtx, &application.ApplicationUpdateSpecRequest{Name: ptr.To("doest-not-exist"), Project: ptr.To("test"), Spec: &v1alpha1.ApplicationSpec{
			Destination: v1alpha1.ApplicationDestination{Namespace: "default", Server: "https://cluster-api.example.com"},
			Source:      &v1alpha1.ApplicationSource{RepoURL: "https://some-fake-source", Path: "."},
		}})
		assert.EqualError(t, err, "rpc error: code = NotFound desc = applications.argoproj.io \"doest-not-exist\" not found", "when the request specifies a project, we can return the standard k8s error message")
	})

	t.Run("Patch", func(t *testing.T) {
		_, err := appServer.Patch(adminCtx, &application.ApplicationPatchRequest{Name: ptr.To("test"), Patch: ptr.To(`[{"op": "replace", "path": "/spec/source/path", "value": "foo"}]`)})
		require.NoError(t, err)
		_, err = appServer.Patch(noRoleCtx, &application.ApplicationPatchRequest{Name: ptr.To("test"), Patch: ptr.To(`[{"op": "replace", "path": "/spec/source/path", "value": "foo"}]`)})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.Patch(adminCtx, &application.ApplicationPatchRequest{Name: ptr.To("doest-not-exist")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.Patch(adminCtx, &application.ApplicationPatchRequest{Name: ptr.To("doest-not-exist"), Project: ptr.To("test")})
		assert.EqualError(t, err, "rpc error: code = NotFound desc = applications.argoproj.io \"doest-not-exist\" not found", "when the request specifies a project, we can return the standard k8s error message")
	})

	t.Run("GetResource", func(t *testing.T) {
		_, err := appServer.GetResource(adminCtx, &application.ApplicationResourceRequest{Name: ptr.To("test"), ResourceName: ptr.To("test"), Group: ptr.To("apps"), Kind: ptr.To("Deployment"), Namespace: ptr.To("test")})
		require.NoError(t, err)
		_, err = appServer.GetResource(noRoleCtx, &application.ApplicationResourceRequest{Name: ptr.To("test"), ResourceName: ptr.To("test"), Group: ptr.To("apps"), Kind: ptr.To("Deployment"), Namespace: ptr.To("test")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.GetResource(adminCtx, &application.ApplicationResourceRequest{Name: ptr.To("doest-not-exist"), ResourceName: ptr.To("test"), Group: ptr.To("apps"), Kind: ptr.To("Deployment"), Namespace: ptr.To("test")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.GetResource(adminCtx, &application.ApplicationResourceRequest{Name: ptr.To("doest-not-exist"), Project: ptr.To("test"), ResourceName: ptr.To("test"), Group: ptr.To("apps"), Kind: ptr.To("Deployment"), Namespace: ptr.To("test")})
		assert.EqualError(t, err, "rpc error: code = NotFound desc = applications.argoproj.io \"doest-not-exist\" not found", "when the request specifies a project, we can return the standard k8s error message")
	})

	t.Run("PatchResource", func(t *testing.T) {
		_, err := appServer.PatchResource(adminCtx, &application.ApplicationResourcePatchRequest{Name: ptr.To("test"), ResourceName: ptr.To("test"), Group: ptr.To("apps"), Kind: ptr.To("Deployment"), Namespace: ptr.To("test"), Patch: ptr.To(`[{"op": "replace", "path": "/spec/replicas", "value": 3}]`)})
		// This will always throw an error, because the kubectl mock for PatchResource is hard-coded to return nil.
		// The best we can do is to confirm we get past the permission check.
		assert.NotEqual(t, common.PermissionDeniedAPIError.Error(), err.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.PatchResource(noRoleCtx, &application.ApplicationResourcePatchRequest{Name: ptr.To("test"), ResourceName: ptr.To("test"), Group: ptr.To("apps"), Kind: ptr.To("Deployment"), Namespace: ptr.To("test"), Patch: ptr.To(`[{"op": "replace", "path": "/spec/replicas", "value": 3}]`)})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.PatchResource(adminCtx, &application.ApplicationResourcePatchRequest{Name: ptr.To("doest-not-exist"), ResourceName: ptr.To("test"), Group: ptr.To("apps"), Kind: ptr.To("Deployment"), Namespace: ptr.To("test"), Patch: ptr.To(`[{"op": "replace", "path": "/spec/replicas", "value": 3}]`)})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.PatchResource(adminCtx, &application.ApplicationResourcePatchRequest{Name: ptr.To("doest-not-exist"), Project: ptr.To("test"), ResourceName: ptr.To("test"), Group: ptr.To("apps"), Kind: ptr.To("Deployment"), Namespace: ptr.To("test"), Patch: ptr.To(`[{"op": "replace", "path": "/spec/replicas", "value": 3}]`)})
		assert.EqualError(t, err, "rpc error: code = NotFound desc = applications.argoproj.io \"doest-not-exist\" not found", "when the request specifies a project, we can return the standard k8s error message")
	})

	t.Run("DeleteResource", func(t *testing.T) {
		_, err := appServer.DeleteResource(adminCtx, &application.ApplicationResourceDeleteRequest{Name: ptr.To("test"), ResourceName: ptr.To("test"), Group: ptr.To("apps"), Kind: ptr.To("Deployment"), Namespace: ptr.To("test")})
		require.NoError(t, err)
		_, err = appServer.DeleteResource(noRoleCtx, &application.ApplicationResourceDeleteRequest{Name: ptr.To("test"), ResourceName: ptr.To("test"), Group: ptr.To("apps"), Kind: ptr.To("Deployment"), Namespace: ptr.To("test")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.DeleteResource(adminCtx, &application.ApplicationResourceDeleteRequest{Name: ptr.To("doest-not-exist"), ResourceName: ptr.To("test"), Group: ptr.To("apps"), Kind: ptr.To("Deployment"), Namespace: ptr.To("test")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.DeleteResource(adminCtx, &application.ApplicationResourceDeleteRequest{Name: ptr.To("doest-not-exist"), Project: ptr.To("test"), ResourceName: ptr.To("test"), Group: ptr.To("apps"), Kind: ptr.To("Deployment"), Namespace: ptr.To("test")})
		assert.EqualError(t, err, "rpc error: code = NotFound desc = applications.argoproj.io \"doest-not-exist\" not found", "when the request specifies a project, we can return the standard k8s error message")
	})

	t.Run("ResourceTree", func(t *testing.T) {
		_, err := appServer.ResourceTree(adminCtx, &application.ResourcesQuery{ApplicationName: ptr.To("test")})
		require.NoError(t, err)
		_, err = appServer.ResourceTree(noRoleCtx, &application.ResourcesQuery{ApplicationName: ptr.To("test")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.ResourceTree(adminCtx, &application.ResourcesQuery{ApplicationName: ptr.To("doest-not-exist")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.ResourceTree(adminCtx, &application.ResourcesQuery{ApplicationName: ptr.To("doest-not-exist"), Project: ptr.To("test")})
		assert.EqualError(t, err, "rpc error: code = NotFound desc = applications.argoproj.io \"doest-not-exist\" not found", "when the request specifies a project, we can return the standard k8s error message")
	})

	t.Run("RevisionMetadata", func(t *testing.T) {
		_, err := appServer.RevisionMetadata(adminCtx, &application.RevisionMetadataQuery{Name: ptr.To("test")})
		require.NoError(t, err)
		_, err = appServer.RevisionMetadata(adminCtx, &application.RevisionMetadataQuery{Name: ptr.To("test-multi"), SourceIndex: ptr.To(int32(0)), VersionId: ptr.To(int32(1))})
		require.NoError(t, err)
		_, err = appServer.RevisionMetadata(noRoleCtx, &application.RevisionMetadataQuery{Name: ptr.To("test")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.RevisionMetadata(adminCtx, &application.RevisionMetadataQuery{Name: ptr.To("doest-not-exist")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.RevisionMetadata(adminCtx, &application.RevisionMetadataQuery{Name: ptr.To("doest-not-exist"), Project: ptr.To("test")})
		assert.EqualError(t, err, "rpc error: code = NotFound desc = applications.argoproj.io \"doest-not-exist\" not found", "when the request specifies a project, we can return the standard k8s error message")
	})

	t.Run("RevisionChartDetails", func(t *testing.T) {
		_, err := appServer.RevisionChartDetails(adminCtx, &application.RevisionMetadataQuery{Name: ptr.To("test-helm")})
		require.NoError(t, err)
		_, err = appServer.RevisionChartDetails(noRoleCtx, &application.RevisionMetadataQuery{Name: ptr.To("test-helm")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.RevisionChartDetails(adminCtx, &application.RevisionMetadataQuery{Name: ptr.To("doest-not-exist")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.RevisionChartDetails(adminCtx, &application.RevisionMetadataQuery{Name: ptr.To("doest-not-exist"), Project: ptr.To("test")})
		assert.EqualError(t, err, "rpc error: code = NotFound desc = applications.argoproj.io \"doest-not-exist\" not found", "when the request specifies a project, we can return the standard k8s error message")
	})

	t.Run("ManagedResources", func(t *testing.T) {
		_, err := appServer.ManagedResources(adminCtx, &application.ResourcesQuery{ApplicationName: ptr.To("test")})
		require.NoError(t, err)
		_, err = appServer.ManagedResources(noRoleCtx, &application.ResourcesQuery{ApplicationName: ptr.To("test")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.ManagedResources(adminCtx, &application.ResourcesQuery{ApplicationName: ptr.To("doest-not-exist")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.ManagedResources(adminCtx, &application.ResourcesQuery{ApplicationName: ptr.To("doest-not-exist"), Project: ptr.To("test")})
		assert.EqualError(t, err, "rpc error: code = NotFound desc = applications.argoproj.io \"doest-not-exist\" not found", "when the request specifies a project, we can return the standard k8s error message")
	})

	t.Run("Sync", func(t *testing.T) {
		_, err := appServer.Sync(adminCtx, &application.ApplicationSyncRequest{Name: ptr.To("test")})
		require.NoError(t, err)
		_, err = appServer.Sync(noRoleCtx, &application.ApplicationSyncRequest{Name: ptr.To("test")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.Sync(adminCtx, &application.ApplicationSyncRequest{Name: ptr.To("doest-not-exist")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.Sync(adminCtx, &application.ApplicationSyncRequest{Name: ptr.To("doest-not-exist"), Project: ptr.To("test")})
		assert.EqualError(t, err, "rpc error: code = NotFound desc = applications.argoproj.io \"doest-not-exist\" not found", "when the request specifies a project, we can return the standard k8s error message")
	})

	t.Run("TerminateOperation", func(t *testing.T) {
		// The sync operation is already started from the previous test. We just need to set the field that the
		// controller would set if this were an actual Argo CD environment.
		setSyncRunningOperationState(t, appServer)
		_, err := appServer.TerminateOperation(adminCtx, &application.OperationTerminateRequest{Name: ptr.To("test")})
		require.NoError(t, err)
		_, err = appServer.TerminateOperation(noRoleCtx, &application.OperationTerminateRequest{Name: ptr.To("test")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.TerminateOperation(adminCtx, &application.OperationTerminateRequest{Name: ptr.To("doest-not-exist")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.TerminateOperation(adminCtx, &application.OperationTerminateRequest{Name: ptr.To("doest-not-exist"), Project: ptr.To("test")})
		assert.EqualError(t, err, "rpc error: code = NotFound desc = applications.argoproj.io \"doest-not-exist\" not found", "when the request specifies a project, we can return the standard k8s error message")
	})

	t.Run("Rollback", func(t *testing.T) {
		unsetSyncRunningOperationState(t, appServer)
		_, err := appServer.Rollback(adminCtx, &application.ApplicationRollbackRequest{Name: ptr.To("test")})
		require.NoError(t, err)
		_, err = appServer.Rollback(adminCtx, &application.ApplicationRollbackRequest{Name: ptr.To("test-multi"), Id: ptr.To(int64(1))})
		require.NoError(t, err)
		_, err = appServer.Rollback(noRoleCtx, &application.ApplicationRollbackRequest{Name: ptr.To("test")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.Rollback(adminCtx, &application.ApplicationRollbackRequest{Name: ptr.To("doest-not-exist")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.Rollback(adminCtx, &application.ApplicationRollbackRequest{Name: ptr.To("doest-not-exist"), Project: ptr.To("test")})
		assert.EqualError(t, err, "rpc error: code = NotFound desc = applications.argoproj.io \"doest-not-exist\" not found", "when the request specifies a project, we can return the standard k8s error message")
	})

	t.Run("ListResourceActions", func(t *testing.T) {
		_, err := appServer.ListResourceActions(adminCtx, &application.ApplicationResourceRequest{Name: ptr.To("test"), ResourceName: ptr.To("test"), Group: ptr.To("apps"), Kind: ptr.To("Deployment"), Namespace: ptr.To("test")})
		require.NoError(t, err)
		_, err = appServer.ListResourceActions(noRoleCtx, &application.ApplicationResourceRequest{Name: ptr.To("test")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.ListResourceActions(noRoleCtx, &application.ApplicationResourceRequest{Group: ptr.To("argoproj.io"), Kind: ptr.To("Application"), Name: ptr.To("test")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.ListResourceActions(adminCtx, &application.ApplicationResourceRequest{Name: ptr.To("doest-not-exist")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.ListResourceActions(adminCtx, &application.ApplicationResourceRequest{Name: ptr.To("doest-not-exist"), Project: ptr.To("test")})
		assert.EqualError(t, err, "rpc error: code = NotFound desc = applications.argoproj.io \"doest-not-exist\" not found", "when the request specifies a project, we can return the standard k8s error message")
	})

	//nolint:staticcheck,SA1019 // RunResourceAction is deprecated, but we still need to support it for backward compatibility.
	t.Run("RunResourceAction", func(t *testing.T) {
		_, err := appServer.RunResourceAction(adminCtx, &application.ResourceActionRunRequest{Name: ptr.To("test"), ResourceName: ptr.To("test"), Group: ptr.To("apps"), Kind: ptr.To("Deployment"), Namespace: ptr.To("test"), Action: ptr.To("restart")})
		require.NoError(t, err)
		_, err = appServer.RunResourceAction(noRoleCtx, &application.ResourceActionRunRequest{Name: ptr.To("test")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.RunResourceAction(noRoleCtx, &application.ResourceActionRunRequest{Group: ptr.To("argoproj.io"), Kind: ptr.To("Application"), Name: ptr.To("test")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.RunResourceAction(adminCtx, &application.ResourceActionRunRequest{Name: ptr.To("doest-not-exist")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.RunResourceAction(adminCtx, &application.ResourceActionRunRequest{Name: ptr.To("doest-not-exist"), Project: ptr.To("test")})
		assert.EqualError(t, err, "rpc error: code = NotFound desc = applications.argoproj.io \"doest-not-exist\" not found", "when the request specifies a project, we can return the standard k8s error message")
	})

	t.Run("RunResourceActionV2", func(t *testing.T) {
		_, err := appServer.RunResourceActionV2(adminCtx, &application.ResourceActionRunRequestV2{Name: ptr.To("test"), ResourceName: ptr.To("test"), Group: ptr.To("apps"), Kind: ptr.To("Deployment"), Namespace: ptr.To("test"), Action: ptr.To("restart")})
		require.NoError(t, err)
		_, err = appServer.RunResourceActionV2(noRoleCtx, &application.ResourceActionRunRequestV2{Name: ptr.To("test")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.RunResourceActionV2(noRoleCtx, &application.ResourceActionRunRequestV2{Group: ptr.To("argoproj.io"), Kind: ptr.To("Application"), Name: ptr.To("test")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.RunResourceActionV2(adminCtx, &application.ResourceActionRunRequestV2{Name: ptr.To("doest-not-exist")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.RunResourceActionV2(adminCtx, &application.ResourceActionRunRequestV2{Name: ptr.To("doest-not-exist"), Project: ptr.To("test")})
		assert.EqualError(t, err, "rpc error: code = NotFound desc = applications.argoproj.io \"doest-not-exist\" not found", "when the request specifies a project, we can return the standard k8s error message")
	})

	t.Run("GetApplicationSyncWindows", func(t *testing.T) {
		_, err := appServer.GetApplicationSyncWindows(adminCtx, &application.ApplicationSyncWindowsQuery{Name: ptr.To("test")})
		require.NoError(t, err)
		_, err = appServer.GetApplicationSyncWindows(noRoleCtx, &application.ApplicationSyncWindowsQuery{Name: ptr.To("test")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.GetApplicationSyncWindows(adminCtx, &application.ApplicationSyncWindowsQuery{Name: ptr.To("doest-not-exist")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.GetApplicationSyncWindows(adminCtx, &application.ApplicationSyncWindowsQuery{Name: ptr.To("doest-not-exist"), Project: ptr.To("test")})
		assert.EqualError(t, err, "rpc error: code = NotFound desc = applications.argoproj.io \"doest-not-exist\" not found", "when the request specifies a project, we can return the standard k8s error message")
	})

	t.Run("GetManifestsWithFiles", func(t *testing.T) {
		err := appServer.GetManifestsWithFiles(&TestServerStream{ctx: adminCtx, appName: "test"})
		require.NoError(t, err)
		err = appServer.GetManifestsWithFiles(&TestServerStream{ctx: noRoleCtx, appName: "test"})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		err = appServer.GetManifestsWithFiles(&TestServerStream{ctx: adminCtx, appName: "does-not-exist"})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		err = appServer.GetManifestsWithFiles(&TestServerStream{ctx: adminCtx, appName: "does-not-exist", project: "test"})
		assert.EqualError(t, err, "rpc error: code = NotFound desc = applications.argoproj.io \"does-not-exist\" not found", "when the request specifies a project, we can return the standard k8s error message")
	})

	t.Run("WatchResourceTree", func(t *testing.T) {
		err := appServer.WatchResourceTree(&application.ResourcesQuery{ApplicationName: ptr.To("test")}, &TestResourceTreeServer{ctx: adminCtx})
		require.NoError(t, err)
		err = appServer.WatchResourceTree(&application.ResourcesQuery{ApplicationName: ptr.To("test")}, &TestResourceTreeServer{ctx: noRoleCtx})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		err = appServer.WatchResourceTree(&application.ResourcesQuery{ApplicationName: ptr.To("does-not-exist")}, &TestResourceTreeServer{ctx: adminCtx})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		err = appServer.WatchResourceTree(&application.ResourcesQuery{ApplicationName: ptr.To("does-not-exist"), Project: ptr.To("test")}, &TestResourceTreeServer{ctx: adminCtx})
		assert.EqualError(t, err, "rpc error: code = NotFound desc = applications.argoproj.io \"does-not-exist\" not found", "when the request specifies a project, we can return the standard k8s error message")
	})

	t.Run("PodLogs", func(t *testing.T) {
		err := appServer.PodLogs(&application.ApplicationPodLogsQuery{Name: ptr.To("test")}, &TestPodLogsServer{ctx: adminCtx})
		require.NoError(t, err)
		err = appServer.PodLogs(&application.ApplicationPodLogsQuery{Name: ptr.To("test")}, &TestPodLogsServer{ctx: noRoleCtx})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		err = appServer.PodLogs(&application.ApplicationPodLogsQuery{Name: ptr.To("does-not-exist")}, &TestPodLogsServer{ctx: adminCtx})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		err = appServer.PodLogs(&application.ApplicationPodLogsQuery{Name: ptr.To("does-not-exist"), Project: ptr.To("test")}, &TestPodLogsServer{ctx: adminCtx})
		assert.EqualError(t, err, "rpc error: code = NotFound desc = applications.argoproj.io \"does-not-exist\" not found", "when the request specifies a project, we can return the standard k8s error message")
	})

	t.Run("ListLinks", func(t *testing.T) {
		_, err := appServer.ListLinks(adminCtx, &application.ListAppLinksRequest{Name: ptr.To("test")})
		require.NoError(t, err)
		_, err = appServer.ListLinks(noRoleCtx, &application.ListAppLinksRequest{Name: ptr.To("test")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.ListLinks(adminCtx, &application.ListAppLinksRequest{Name: ptr.To("does-not-exist")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.ListLinks(adminCtx, &application.ListAppLinksRequest{Name: ptr.To("does-not-exist"), Project: ptr.To("test")})
		assert.EqualError(t, err, "rpc error: code = NotFound desc = applications.argoproj.io \"does-not-exist\" not found", "when the request specifies a project, we can return the standard k8s error message")
	})

	t.Run("ListResourceLinks", func(t *testing.T) {
		_, err := appServer.ListResourceLinks(adminCtx, &application.ApplicationResourceRequest{Name: ptr.To("test"), ResourceName: ptr.To("test"), Group: ptr.To("apps"), Kind: ptr.To("Deployment"), Namespace: ptr.To("test")})
		require.NoError(t, err)
		_, err = appServer.ListResourceLinks(noRoleCtx, &application.ApplicationResourceRequest{Name: ptr.To("test"), ResourceName: ptr.To("test"), Group: ptr.To("apps"), Kind: ptr.To("Deployment"), Namespace: ptr.To("test")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.ListResourceLinks(adminCtx, &application.ApplicationResourceRequest{Name: ptr.To("does-not-exist"), ResourceName: ptr.To("test"), Group: ptr.To("apps"), Kind: ptr.To("Deployment"), Namespace: ptr.To("test")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.ListResourceLinks(adminCtx, &application.ApplicationResourceRequest{Name: ptr.To("does-not-exist"), ResourceName: ptr.To("test"), Group: ptr.To("apps"), Kind: ptr.To("Deployment"), Namespace: ptr.To("test"), Project: ptr.To("test")})
		assert.EqualError(t, err, "rpc error: code = NotFound desc = applications.argoproj.io \"does-not-exist\" not found", "when the request specifies a project, we can return the standard k8s error message")
	})

	// Do this last so other stuff doesn't fail.
	t.Run("Delete", func(t *testing.T) {
		_, err := appServer.Delete(adminCtx, &application.ApplicationDeleteRequest{Name: ptr.To("test")})
		require.NoError(t, err)
		_, err = appServer.Delete(noRoleCtx, &application.ApplicationDeleteRequest{Name: ptr.To("test")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.Delete(adminCtx, &application.ApplicationDeleteRequest{Name: ptr.To("doest-not-exist")})
		require.EqualError(t, err, common.PermissionDeniedAPIError.Error(), "error message must be _only_ the permission error, to avoid leaking information about app existence")
		_, err = appServer.Delete(adminCtx, &application.ApplicationDeleteRequest{Name: ptr.To("doest-not-exist"), Project: ptr.To("test")})
		assert.EqualError(t, err, "rpc error: code = NotFound desc = applications.argoproj.io \"doest-not-exist\" not found", "when the request specifies a project, we can return the standard k8s error message")
	})
}

// setSyncRunningOperationState simulates starting a sync operation on the given app.
func setSyncRunningOperationState(t *testing.T, appServer *Server) {
	t.Helper()
	appIf := appServer.appclientset.ArgoprojV1alpha1().Applications("default")
	app, err := appIf.Get(t.Context(), "test", metav1.GetOptions{})
	require.NoError(t, err)
	// This sets the status that would be set by the controller usually.
	app.Status.OperationState = &v1alpha1.OperationState{Phase: synccommon.OperationRunning, Operation: v1alpha1.Operation{Sync: &v1alpha1.SyncOperation{}}}
	_, err = appIf.Update(t.Context(), app, metav1.UpdateOptions{})
	require.NoError(t, err)
}

// unsetSyncRunningOperationState simulates finishing a sync operation on the given app.
func unsetSyncRunningOperationState(t *testing.T, appServer *Server) {
	t.Helper()
	appIf := appServer.appclientset.ArgoprojV1alpha1().Applications("default")
	app, err := appIf.Get(t.Context(), "test", metav1.GetOptions{})
	require.NoError(t, err)
	app.Operation = nil
	app.Status.OperationState = nil
	_, err = appIf.Update(t.Context(), app, metav1.UpdateOptions{})
	require.NoError(t, err)
}

func TestListAppsInNamespaceWithLabels(t *testing.T) {
	appServer := newTestAppServer(t, newTestApp(func(app *v1alpha1.Application) {
		app.Name = "App1"
		app.Namespace = "test-namespace"
		app.SetLabels(map[string]string{"key1": "value1", "key2": "value1"})
	}), newTestApp(func(app *v1alpha1.Application) {
		app.Name = "App2"
		app.Namespace = "test-namespace"
		app.SetLabels(map[string]string{"key1": "value2"})
	}), newTestApp(func(app *v1alpha1.Application) {
		app.Name = "App3"
		app.Namespace = "test-namespace"
		app.SetLabels(map[string]string{"key1": "value3"})
	}))
	appServer.ns = "test-namespace"
	appQuery := application.ApplicationQuery{}
	namespace := "test-namespace"
	appQuery.AppNamespace = &namespace
	testListAppsWithLabels(t, appQuery, appServer)
}

func TestListAppsInDefaultNSWithLabels(t *testing.T) {
	appServer := newTestAppServer(t, newTestApp(func(app *v1alpha1.Application) {
		app.Name = "App1"
		app.SetLabels(map[string]string{"key1": "value1", "key2": "value1"})
	}), newTestApp(func(app *v1alpha1.Application) {
		app.Name = "App2"
		app.SetLabels(map[string]string{"key1": "value2"})
	}), newTestApp(func(app *v1alpha1.Application) {
		app.Name = "App3"
		app.SetLabels(map[string]string{"key1": "value3"})
	}))
	appQuery := application.ApplicationQuery{}
	testListAppsWithLabels(t, appQuery, appServer)
}

func testListAppsWithLabels(t *testing.T, appQuery application.ApplicationQuery, appServer *Server) {
	t.Helper()
	validTests := []struct {
		testName       string
		label          string
		expectedResult []string
	}{
		{
			testName:       "Equality based filtering using '=' operator",
			label:          "key1=value1",
			expectedResult: []string{"App1"},
		},
		{
			testName:       "Equality based filtering using '==' operator",
			label:          "key1==value1",
			expectedResult: []string{"App1"},
		},
		{
			testName:       "Equality based filtering using '!=' operator",
			label:          "key1!=value1",
			expectedResult: []string{"App2", "App3"},
		},
		{
			testName:       "Set based filtering using 'in' operator",
			label:          "key1 in (value1, value3)",
			expectedResult: []string{"App1", "App3"},
		},
		{
			testName:       "Set based filtering using 'notin' operator",
			label:          "key1 notin (value1, value3)",
			expectedResult: []string{"App2"},
		},
		{
			testName:       "Set based filtering using 'exists' operator",
			label:          "key1",
			expectedResult: []string{"App1", "App2", "App3"},
		},
		{
			testName:       "Set based filtering using 'not exists' operator",
			label:          "!key2",
			expectedResult: []string{"App2", "App3"},
		},
	}
	// test valid scenarios
	for _, validTest := range validTests {
		t.Run(validTest.testName, func(t *testing.T) {
			appQuery.Selector = &validTest.label
			res, err := appServer.List(t.Context(), &appQuery)
			require.NoError(t, err)
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
		{
			testName:    "Set based filtering using '>' operator",
			label:       "key1>value1",
			errorMesage: "error parsing the selector",
		},
		{
			testName:    "Set based filtering using '<' operator",
			label:       "key1<value1",
			errorMesage: "error parsing the selector",
		},
	}
	// test invalid scenarios
	for _, invalidTest := range invalidTests {
		t.Run(invalidTest.testName, func(t *testing.T) {
			appQuery.Selector = &invalidTest.label
			_, err := appServer.List(t.Context(), &appQuery)
			assert.ErrorContains(t, err, invalidTest.errorMesage)
		})
	}
}

func TestListAppWithProjects(t *testing.T) {
	appServer := newTestAppServer(t, newTestApp(func(app *v1alpha1.Application) {
		app.Name = "App1"
		app.Spec.Project = "test-project1"
	}), newTestApp(func(app *v1alpha1.Application) {
		app.Name = "App2"
		app.Spec.Project = "test-project2"
	}), newTestApp(func(app *v1alpha1.Application) {
		app.Name = "App3"
		app.Spec.Project = "test-project3"
	}))

	t.Run("List all apps", func(t *testing.T) {
		appQuery := application.ApplicationQuery{}
		appList, err := appServer.List(t.Context(), &appQuery)
		require.NoError(t, err)
		assert.Len(t, appList.Items, 3)
	})

	t.Run("List apps with projects filter set", func(t *testing.T) {
		appQuery := application.ApplicationQuery{Projects: []string{"test-project1"}}
		appList, err := appServer.List(t.Context(), &appQuery)
		require.NoError(t, err)
		assert.Len(t, appList.Items, 1)
		for _, app := range appList.Items {
			assert.Equal(t, "test-project1", app.Spec.Project)
		}
	})

	t.Run("List apps with project filter set (legacy field)", func(t *testing.T) {
		appQuery := application.ApplicationQuery{Project: []string{"test-project1"}}
		appList, err := appServer.List(t.Context(), &appQuery)
		require.NoError(t, err)
		assert.Len(t, appList.Items, 1)
		for _, app := range appList.Items {
			assert.Equal(t, "test-project1", app.Spec.Project)
		}
	})

	t.Run("List apps with both projects and project filter set", func(t *testing.T) {
		// If the older field is present, we should use it instead of the newer field.
		appQuery := application.ApplicationQuery{Project: []string{"test-project1"}, Projects: []string{"test-project2"}}
		appList, err := appServer.List(t.Context(), &appQuery)
		require.NoError(t, err)
		assert.Len(t, appList.Items, 1)
		for _, app := range appList.Items {
			assert.Equal(t, "test-project1", app.Spec.Project)
		}
	})
}

func TestListApps(t *testing.T) {
	appServer := newTestAppServer(t, newTestApp(func(app *v1alpha1.Application) {
		app.Name = "bcd"
	}), newTestApp(func(app *v1alpha1.Application) {
		app.Name = "abc"
	}), newTestApp(func(app *v1alpha1.Application) {
		app.Name = "def"
	}))

	res, err := appServer.List(t.Context(), &application.ApplicationQuery{})
	require.NoError(t, err)
	var names []string
	for i := range res.Items {
		names = append(names, res.Items[i].Name)
	}
	assert.Equal(t, []string{"abc", "bcd", "def"}, names)
}

func TestCoupleAppsListApps(t *testing.T) {
	var objects []runtime.Object
	ctx := t.Context()

	var groups []string
	for i := range 50 {
		groups = append(groups, fmt.Sprintf("group-%d", i))
	}
	//nolint:staticcheck
	ctx = context.WithValue(ctx, "claims", &jwt.MapClaims{"groups": groups})
	for projectId := range 100 {
		projectName := fmt.Sprintf("proj-%d", projectId)
		for appId := range 100 {
			objects = append(objects, newTestApp(func(app *v1alpha1.Application) {
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
	appServer := newTestAppServerWithEnforcerConfigure(t, f, map[string]string{}, objects...)

	res, err := appServer.List(ctx, &application.ApplicationQuery{})

	require.NoError(t, err)
	var names []string
	for i := range res.Items {
		names = append(names, res.Items[i].Name)
	}
	assert.Len(t, names, 300)
}

func generateTestApp(num int) []*v1alpha1.Application {
	apps := []*v1alpha1.Application{}
	for i := range num {
		apps = append(apps, newTestApp(func(app *v1alpha1.Application) {
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

	for b.Loop() {
		_, err := appServer.List(b.Context(), &application.ApplicationQuery{})
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

	for b.Loop() {
		_, err := appServer.List(b.Context(), &application.ApplicationQuery{})
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

	for b.Loop() {
		_, err := appServer.List(b.Context(), &application.ApplicationQuery{})
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

	for b.Loop() {
		app := &application.ApplicationQuery{Name: strToPtr("test-app000099")}
		_, err := appServer.List(b.Context(), app)
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

	for b.Loop() {
		app := &application.ApplicationQuery{Project: []string{"test-project1", "test-project2"}}
		_, err := appServer.List(b.Context(), app)
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

	for b.Loop() {
		app := &application.ApplicationQuery{Repo: strToPtr("https://some-fake-source")}
		_, err := appServer.List(b.Context(), app)
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
	app, err := appServer.Create(t.Context(), &createReq)
	require.NoError(t, err)
	assert.NotNil(t, app)
	assert.NotNil(t, app.Spec)
	assert.Equal(t, "default", app.Spec.Project)
}

func TestCreateAppWithDestName(t *testing.T) {
	appServer := newTestAppServer(t)
	testApp := newTestAppWithDestName()
	createReq := application.ApplicationCreateRequest{
		Application: testApp,
	}
	app, err := appServer.Create(t.Context(), &createReq)
	require.NoError(t, err)
	assert.NotNil(t, app)
}

// TestCreateAppWithOperation tests that an application created with an operation is created with the operation removed.
// Avoids regressions of https://github.com/argoproj/argo-cd/security/advisories/GHSA-g623-jcgg-mhmm
func TestCreateAppWithOperation(t *testing.T) {
	appServer := newTestAppServer(t)
	testApp := newTestAppWithDestName()
	testApp.Operation = &v1alpha1.Operation{
		Sync: &v1alpha1.SyncOperation{
			Manifests: []string{
				"test",
			},
		},
	}
	createReq := application.ApplicationCreateRequest{
		Application: testApp,
	}
	app, err := appServer.Create(t.Context(), &createReq)
	require.NoError(t, err)
	require.NotNil(t, app)
	assert.Nil(t, app.Operation)
}

func TestCreateAppUpsert(t *testing.T) {
	t.Parallel()
	t.Run("No error when spec equals", func(t *testing.T) {
		t.Parallel()
		appServer := newTestAppServer(t)
		testApp := newTestApp()

		createReq := application.ApplicationCreateRequest{
			Application: testApp,
		}
		// Call Create() instead of adding the object to the tesst server to make sure the app is correctly normalized.
		_, err := appServer.Create(t.Context(), &createReq)
		require.NoError(t, err)

		app, err := appServer.Create(t.Context(), &createReq)
		require.NoError(t, err)
		require.NotNil(t, app)
	})
	t.Run("Error on update without upsert", func(t *testing.T) {
		t.Parallel()
		appServer := newTestAppServer(t)
		testApp := newTestApp()

		// Call Create() instead of adding the object to the tesst server to make sure the app is correctly normalized.
		_, err := appServer.Create(t.Context(), &application.ApplicationCreateRequest{
			Application: testApp,
		})
		require.NoError(t, err)

		newApp := newTestApp()
		newApp.Spec.Source.Name = "updated"
		createReq := application.ApplicationCreateRequest{
			Application: newApp,
		}
		_, err = appServer.Create(t.Context(), &createReq)
		require.EqualError(t, err, "rpc error: code = InvalidArgument desc = existing application spec is different, use upsert flag to force update")
	})
	t.Run("Invalid existing app can be updated", func(t *testing.T) {
		t.Parallel()
		testApp := newTestApp()
		testApp.Spec.Destination.Server = "https://invalid-cluster"
		appServer := newTestAppServer(t, testApp)

		newApp := newTestAppWithDestName()
		newApp.TypeMeta = testApp.TypeMeta
		newApp.Spec.Source.Name = "updated"
		createReq := application.ApplicationCreateRequest{
			Application: newApp,
			Upsert:      ptr.To(true),
		}
		app, err := appServer.Create(t.Context(), &createReq)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.Equal(t, "updated", app.Spec.Source.Name)
	})
	t.Run("Can update application project", func(t *testing.T) {
		t.Parallel()
		testApp := newTestApp()
		appServer := newTestAppServer(t, testApp)

		newApp := newTestAppWithDestName()
		newApp.TypeMeta = testApp.TypeMeta
		newApp.Spec.Project = "my-proj"
		createReq := application.ApplicationCreateRequest{
			Application: newApp,
			Upsert:      ptr.To(true),
		}
		app, err := appServer.Create(t.Context(), &createReq)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.Equal(t, "my-proj", app.Spec.Project)
	})
	t.Run("Existing label and annotations are preserved", func(t *testing.T) {
		t.Parallel()
		testApp := newTestApp()
		testApp.Annotations = map[string]string{"test": "test-value", "update": "old"}
		testApp.Labels = map[string]string{"test": "test-value", "update": "old"}
		appServer := newTestAppServer(t, testApp)

		newApp := newTestAppWithDestName()
		newApp.TypeMeta = testApp.TypeMeta
		newApp.Annotations = map[string]string{"update": "new"}
		newApp.Labels = map[string]string{"update": "new"}
		createReq := application.ApplicationCreateRequest{
			Application: newApp,
			Upsert:      ptr.To(true),
		}
		app, err := appServer.Create(t.Context(), &createReq)
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.Len(t, app.Annotations, 2)
		assert.Equal(t, "new", app.GetAnnotations()["update"])
		assert.Len(t, app.Labels, 2)
		assert.Equal(t, "new", app.GetLabels()["update"])
	})
}

func TestUpdateApp(t *testing.T) {
	t.Parallel()
	t.Run("Same spec", func(t *testing.T) {
		t.Parallel()
		testApp := newTestApp()
		appServer := newTestAppServer(t, testApp)
		testApp.Spec.Project = ""
		app, err := appServer.Update(t.Context(), &application.ApplicationUpdateRequest{
			Application: testApp,
		})
		require.NoError(t, err)
		assert.Equal(t, "default", app.Spec.Project)
	})
	t.Run("Invalid existing app can be updated", func(t *testing.T) {
		t.Parallel()
		testApp := newTestApp()
		testApp.Spec.Destination.Server = "https://invalid-cluster"
		appServer := newTestAppServer(t, testApp)

		updateApp := newTestAppWithDestName()
		updateApp.TypeMeta = testApp.TypeMeta
		updateApp.Spec.Source.Name = "updated"
		app, err := appServer.Update(t.Context(), &application.ApplicationUpdateRequest{
			Application: updateApp,
		})
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.Equal(t, "updated", app.Spec.Source.Name)
	})
	t.Run("Can update application project from invalid", func(t *testing.T) {
		t.Parallel()
		testApp := newTestApp()
		restrictedProj := &v1alpha1.AppProject{
			ObjectMeta: metav1.ObjectMeta{Name: "restricted-proj", Namespace: "default"},
			Spec: v1alpha1.AppProjectSpec{
				SourceRepos:  []string{"not-your-repo"},
				Destinations: []v1alpha1.ApplicationDestination{{Server: "*", Namespace: "not-your-namespace"}},
			},
		}
		testApp.Spec.Project = restrictedProj.Name
		appServer := newTestAppServer(t, testApp, restrictedProj)

		updateApp := newTestAppWithDestName()
		updateApp.TypeMeta = testApp.TypeMeta
		updateApp.Spec.Project = "my-proj"
		app, err := appServer.Update(t.Context(), &application.ApplicationUpdateRequest{
			Application: updateApp,
		})
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.Equal(t, "my-proj", app.Spec.Project)
	})
	t.Run("Cannot update application project to invalid", func(t *testing.T) {
		t.Parallel()
		testApp := newTestApp()
		restrictedProj := &v1alpha1.AppProject{
			ObjectMeta: metav1.ObjectMeta{Name: "restricted-proj", Namespace: "default"},
			Spec: v1alpha1.AppProjectSpec{
				SourceRepos:  []string{"not-your-repo"},
				Destinations: []v1alpha1.ApplicationDestination{{Server: "*", Namespace: "not-your-namespace"}},
			},
		}
		appServer := newTestAppServer(t, testApp, restrictedProj)

		updateApp := newTestAppWithDestName()
		updateApp.TypeMeta = testApp.TypeMeta
		updateApp.Spec.Project = restrictedProj.Name
		_, err := appServer.Update(t.Context(), &application.ApplicationUpdateRequest{
			Application: updateApp,
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "application repo https://github.com/argoproj/argocd-example-apps.git is not permitted in project 'restricted-proj'")
		require.ErrorContains(t, err, "application destination server 'fake-cluster' and namespace 'fake-dest-ns' do not match any of the allowed destinations in project 'restricted-proj'")
	})
	t.Run("Cannot update application project to inexisting", func(t *testing.T) {
		t.Parallel()
		testApp := newTestApp()
		appServer := newTestAppServer(t, testApp)

		updateApp := newTestAppWithDestName()
		updateApp.TypeMeta = testApp.TypeMeta
		updateApp.Spec.Project = "i-do-not-exist"
		_, err := appServer.Update(t.Context(), &application.ApplicationUpdateRequest{
			Application: updateApp,
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "app is not allowed in project \"i-do-not-exist\", or the project does not exist")
	})
	t.Run("Can update application project with project argument", func(t *testing.T) {
		t.Parallel()
		testApp := newTestApp()
		appServer := newTestAppServer(t, testApp)

		updateApp := newTestAppWithDestName()
		updateApp.TypeMeta = testApp.TypeMeta
		updateApp.Spec.Project = "my-proj"
		app, err := appServer.Update(t.Context(), &application.ApplicationUpdateRequest{
			Application: updateApp,
			Project:     ptr.To("default"),
		})
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.Equal(t, "my-proj", app.Spec.Project)
	})
	t.Run("Existing label and annotations are replaced", func(t *testing.T) {
		t.Parallel()
		testApp := newTestApp()
		testApp.Annotations = map[string]string{"test": "test-value", "update": "old"}
		testApp.Labels = map[string]string{"test": "test-value", "update": "old"}
		appServer := newTestAppServer(t, testApp)

		updateApp := newTestAppWithDestName()
		updateApp.TypeMeta = testApp.TypeMeta
		updateApp.Annotations = map[string]string{"update": "new"}
		updateApp.Labels = map[string]string{"update": "new"}
		app, err := appServer.Update(t.Context(), &application.ApplicationUpdateRequest{
			Application: updateApp,
		})
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.Len(t, app.Annotations, 1)
		assert.Equal(t, "new", app.GetAnnotations()["update"])
		assert.Len(t, app.Labels, 1)
		assert.Equal(t, "new", app.GetLabels()["update"])
	})
}

func TestUpdateAppSpec(t *testing.T) {
	testApp := newTestApp()
	appServer := newTestAppServer(t, testApp)
	testApp.Spec.Project = ""
	spec, err := appServer.UpdateSpec(t.Context(), &application.ApplicationUpdateSpecRequest{
		Name: &testApp.Name,
		Spec: &testApp.Spec,
	})
	require.NoError(t, err)
	assert.Equal(t, "default", spec.Project)
	app, err := appServer.Get(t.Context(), &application.ApplicationQuery{Name: &testApp.Name})
	require.NoError(t, err)
	assert.Equal(t, "default", app.Spec.Project)
}

func TestDeleteApp(t *testing.T) {
	ctx := t.Context()
	appServer := newTestAppServer(t)
	createReq := application.ApplicationCreateRequest{
		Application: newTestApp(),
	}
	app, err := appServer.Create(ctx, &createReq)
	require.NoError(t, err)

	app, err = appServer.Get(ctx, &application.ApplicationQuery{Name: &app.Name})
	require.NoError(t, err)
	assert.NotNil(t, app)

	fakeAppCs := appServer.appclientset.(*deepCopyAppClientset).GetUnderlyingClientSet().(*apps.Clientset)
	// this removes the default */* reactor so we can set our own patch/delete reactor
	fakeAppCs.ReactionChain = nil
	patched := false
	deleted := false
	fakeAppCs.AddReactor("patch", "applications", func(_ kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		patched = true
		return true, nil, nil
	})
	fakeAppCs.AddReactor("delete", "applications", func(_ kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		deleted = true
		return true, nil, nil
	})
	fakeAppCs.AddReactor("get", "applications", func(_ kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &v1alpha1.Application{Spec: v1alpha1.ApplicationSpec{Source: &v1alpha1.ApplicationSource{}}}, nil
	})
	appServer.appclientset = fakeAppCs

	trueVar := true
	_, err = appServer.Delete(ctx, &application.ApplicationDeleteRequest{Name: &app.Name, Cascade: &trueVar})
	require.NoError(t, err)
	assert.True(t, patched)
	assert.True(t, deleted)

	// now call delete with cascade=false. patch should not be called
	falseVar := false
	patched = false
	deleted = false
	_, err = appServer.Delete(ctx, &application.ApplicationDeleteRequest{Name: &app.Name, Cascade: &falseVar})
	require.NoError(t, err)
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
		require.NoError(t, err)
		assert.True(t, patched)
		assert.True(t, deleted)
		t.Cleanup(revertValues)
	})

	t.Run("Delete with cascade disabled and background propagation policy", func(t *testing.T) {
		policy := backgroundPropagationPolicy
		_, err = appServer.Delete(ctx, &application.ApplicationDeleteRequest{Name: &app.Name, Cascade: &falseVar, PropagationPolicy: &policy})
		require.EqualError(t, err, "rpc error: code = InvalidArgument desc = cannot set propagation policy when cascading is disabled")
		assert.False(t, patched)
		assert.False(t, deleted)
		t.Cleanup(revertValues)
	})

	t.Run("Delete with invalid propagation policy", func(t *testing.T) {
		invalidPolicy := "invalid"
		_, err = appServer.Delete(ctx, &application.ApplicationDeleteRequest{Name: &app.Name, Cascade: &trueVar, PropagationPolicy: &invalidPolicy})
		require.EqualError(t, err, "rpc error: code = InvalidArgument desc = invalid propagation policy: invalid")
		assert.False(t, patched)
		assert.False(t, deleted)
		t.Cleanup(revertValues)
	})

	t.Run("Delete with foreground propagation policy", func(t *testing.T) {
		policy := foregroundPropagationPolicy
		_, err = appServer.Delete(ctx, &application.ApplicationDeleteRequest{Name: &app.Name, Cascade: &trueVar, PropagationPolicy: &policy})
		require.NoError(t, err)
		assert.True(t, patched)
		assert.True(t, deleted)
		t.Cleanup(revertValues)
	})
}

func TestDeleteResourcesRBAC(t *testing.T) {
	ctx := t.Context()
	//nolint:staticcheck
	ctx = context.WithValue(ctx, "claims", &jwt.RegisteredClaims{Subject: "test-user"})
	testApp := newTestApp()
	appServer := newTestAppServer(t, testApp)
	appServer.enf.SetDefaultRole("")

	argoCM := map[string]string{"server.rbac.disableApplicationFineGrainedRBACInheritance": "false"}
	appServerWithRBACInheritance := newTestAppServerWithEnforcerConfigure(t, func(_ *rbac.Enforcer) {}, argoCM, testApp)
	appServerWithRBACInheritance.enf.SetDefaultRole("")

	req := application.ApplicationResourceDeleteRequest{
		Name:         &testApp.Name,
		AppNamespace: &testApp.Namespace,
		Group:        strToPtr("fake.io"),
		Kind:         strToPtr("PodTest"),
		Namespace:    strToPtr("fake-ns"),
		ResourceName: strToPtr("my-pod-test"),
	}

	expectedErrorWhenDeleteAllowed := "rpc error: code = InvalidArgument desc = PodTest fake.io my-pod-test not found as part of application test-app"

	t.Run("delete with application permission", func(t *testing.T) {
		_ = appServer.enf.SetBuiltinPolicy(`
p, test-user, applications, delete, default/test-app, allow
`)
		_, err := appServer.DeleteResource(ctx, &req)
		assert.Equal(t, codes.PermissionDenied.String(), status.Code(err).String())
	})

	t.Run("delete with application permission with inheritance", func(t *testing.T) {
		_ = appServerWithRBACInheritance.enf.SetBuiltinPolicy(`
p, test-user, applications, delete, default/test-app, allow
`)
		_, err := appServerWithRBACInheritance.DeleteResource(ctx, &req)
		assert.EqualError(t, err, expectedErrorWhenDeleteAllowed)
	})

	t.Run("delete with application permission but deny subresource", func(t *testing.T) {
		_ = appServer.enf.SetBuiltinPolicy(`
p, test-user, applications, delete, default/test-app, allow
p, test-user, applications, delete/*, default/test-app, deny
`)
		_, err := appServer.DeleteResource(ctx, &req)
		assert.Equal(t, codes.PermissionDenied.String(), status.Code(err).String())
	})

	t.Run("delete with application permission but deny subresource with inheritance", func(t *testing.T) {
		_ = appServerWithRBACInheritance.enf.SetBuiltinPolicy(`
p, test-user, applications, delete, default/test-app, allow
p, test-user, applications, delete/*, default/test-app, deny
`)
		_, err := appServerWithRBACInheritance.DeleteResource(ctx, &req)
		assert.EqualError(t, err, expectedErrorWhenDeleteAllowed)
	})

	t.Run("delete with subresource", func(t *testing.T) {
		_ = appServer.enf.SetBuiltinPolicy(`
p, test-user, applications, delete/*, default/test-app, allow
`)
		_, err := appServer.DeleteResource(ctx, &req)
		assert.EqualError(t, err, expectedErrorWhenDeleteAllowed)
	})

	t.Run("delete with subresource but deny applications", func(t *testing.T) {
		_ = appServer.enf.SetBuiltinPolicy(`
p, test-user, applications, delete, default/test-app, deny
p, test-user, applications, delete/*, default/test-app, allow
`)
		_, err := appServer.DeleteResource(ctx, &req)
		assert.EqualError(t, err, expectedErrorWhenDeleteAllowed)
	})

	t.Run("delete with subresource but deny applications with inheritance", func(t *testing.T) {
		_ = appServerWithRBACInheritance.enf.SetBuiltinPolicy(`
p, test-user, applications, delete, default/test-app, deny
p, test-user, applications, delete/*, default/test-app, allow
`)
		_, err := appServerWithRBACInheritance.DeleteResource(ctx, &req)
		assert.EqualError(t, err, expectedErrorWhenDeleteAllowed)
	})

	t.Run("delete with specific subresource denied", func(t *testing.T) {
		_ = appServer.enf.SetBuiltinPolicy(`
p, test-user, applications, delete/*, default/test-app, allow
p, test-user, applications, delete/fake.io/PodTest/*, default/test-app, deny
`)
		_, err := appServer.DeleteResource(ctx, &req)
		assert.Equal(t, codes.PermissionDenied.String(), status.Code(err).String())
	})
}

func TestPatchResourcesRBAC(t *testing.T) {
	ctx := t.Context()
	//nolint:staticcheck
	ctx = context.WithValue(ctx, "claims", &jwt.RegisteredClaims{Subject: "test-user"})
	testApp := newTestApp()
	appServer := newTestAppServer(t, testApp)
	appServer.enf.SetDefaultRole("")

	argoCM := map[string]string{"server.rbac.disableApplicationFineGrainedRBACInheritance": "false"}
	appServerWithRBACInheritance := newTestAppServerWithEnforcerConfigure(t, func(_ *rbac.Enforcer) {}, argoCM, testApp)
	appServerWithRBACInheritance.enf.SetDefaultRole("")

	req := application.ApplicationResourcePatchRequest{
		Name:         &testApp.Name,
		AppNamespace: &testApp.Namespace,
		Group:        strToPtr("fake.io"),
		Kind:         strToPtr("PodTest"),
		Namespace:    strToPtr("fake-ns"),
		ResourceName: strToPtr("my-pod-test"),
	}

	expectedErrorWhenUpdateAllowed := "rpc error: code = InvalidArgument desc = PodTest fake.io my-pod-test not found as part of application test-app"

	t.Run("patch with application permission", func(t *testing.T) {
		_ = appServer.enf.SetBuiltinPolicy(`
p, test-user, applications, update, default/test-app, allow
`)
		_, err := appServer.PatchResource(ctx, &req)
		assert.Equal(t, codes.PermissionDenied.String(), status.Code(err).String())
	})

	t.Run("patch with application permission with inheritance", func(t *testing.T) {
		_ = appServerWithRBACInheritance.enf.SetBuiltinPolicy(`
p, test-user, applications, update, default/test-app, allow
`)
		_, err := appServerWithRBACInheritance.PatchResource(ctx, &req)
		assert.EqualError(t, err, expectedErrorWhenUpdateAllowed)
	})

	t.Run("patch with application permission but deny subresource", func(t *testing.T) {
		_ = appServer.enf.SetBuiltinPolicy(`
p, test-user, applications, update, default/test-app, allow
p, test-user, applications, update/*, default/test-app, deny
`)
		_, err := appServer.PatchResource(ctx, &req)
		assert.Equal(t, codes.PermissionDenied.String(), status.Code(err).String())
	})

	t.Run("patch with application permission but deny subresource with inheritance", func(t *testing.T) {
		_ = appServerWithRBACInheritance.enf.SetBuiltinPolicy(`
p, test-user, applications, update, default/test-app, allow
p, test-user, applications, update/*, default/test-app, deny
`)
		_, err := appServerWithRBACInheritance.PatchResource(ctx, &req)
		assert.EqualError(t, err, expectedErrorWhenUpdateAllowed)
	})

	t.Run("patch with subresource", func(t *testing.T) {
		_ = appServer.enf.SetBuiltinPolicy(`
p, test-user, applications, update/*, default/test-app, allow
`)
		_, err := appServer.PatchResource(ctx, &req)
		assert.EqualError(t, err, expectedErrorWhenUpdateAllowed)
	})

	t.Run("patch with subresource but deny applications", func(t *testing.T) {
		_ = appServer.enf.SetBuiltinPolicy(`
p, test-user, applications, update, default/test-app, deny
p, test-user, applications, update/*, default/test-app, allow
`)
		_, err := appServer.PatchResource(ctx, &req)
		assert.EqualError(t, err, expectedErrorWhenUpdateAllowed)
	})

	t.Run("patch with subresource but deny applications with inheritance", func(t *testing.T) {
		_ = appServerWithRBACInheritance.enf.SetBuiltinPolicy(`
p, test-user, applications, update, default/test-app, deny
p, test-user, applications, update/*, default/test-app, allow
`)
		_, err := appServerWithRBACInheritance.PatchResource(ctx, &req)
		assert.EqualError(t, err, expectedErrorWhenUpdateAllowed)
	})

	t.Run("patch with specific subresource denied", func(t *testing.T) {
		_ = appServer.enf.SetBuiltinPolicy(`
p, test-user, applications, update/*, default/test-app, allow
p, test-user, applications, update/fake.io/PodTest/*, default/test-app, deny
`)
		_, err := appServer.PatchResource(ctx, &req)
		assert.Equal(t, codes.PermissionDenied.String(), status.Code(err).String())
	})
}

func TestSyncRBACOverrideRequired_DiffRevDenied(t *testing.T) {
	ctx := t.Context()
	//nolint:staticcheck
	ctx = context.WithValue(ctx, "claims", &jwt.RegisteredClaims{Subject: "test-user"})

	f := func(enf *rbac.Enforcer) {
		_ = enf.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
		enf.SetDefaultRole("role:admin")
	}
	appServer := newTestAppServerWithEnforcerConfigure(t, f,
		map[string]string{"application.sync.requireOverridePrivilegeForRevisionSync": "true"})

	_ = appServer.enf.SetBuiltinPolicy(`
        p, test-user, applications, get, default/*, allow
        p, test-user, applications, create, default/*, allow
        p, test-user, applications, sync, default/*, allow
        p, test-user, applications, override, default/*, deny
    `)

	testApp := newTestApp()
	app, err := appServer.Create(ctx,
		&application.ApplicationCreateRequest{Application: testApp})
	require.NoError(t, err)
	syncReq := &application.ApplicationSyncRequest{
		Name:     &app.Name,
		Revision: ptr.To("revisionbranch"),
	}
	_, err = appServer.Sync(ctx, syncReq)
	assert.Equal(t, codes.PermissionDenied.String(), status.Code(err).String(),
		"should not be able to sync to different revision without override permission")

	// same for multi-source app
	multiSourceApp := newMultiSourceTestApp()
	app, err = appServer.Create(ctx,
		&application.ApplicationCreateRequest{Application: multiSourceApp})
	require.NoError(t, err)
	syncReq = &application.ApplicationSyncRequest{
		Name:            &app.Name,
		SourcePositions: []int64{1},
		Revisions:       []string{"revisionbranch1"},
	}
	_, err = appServer.Sync(ctx, syncReq)
	assert.Equal(t, codes.PermissionDenied.String(), status.Code(err).String(),
		"should not be able to sync to different revision without override permission, multi-source app")
}

func TestSyncRBACOverrideRequired_SameRevisionAllowed(t *testing.T) {
	ctx := t.Context()
	//nolint:staticcheck
	ctx = context.WithValue(ctx, "claims", &jwt.RegisteredClaims{Subject: "test-user"})

	f := func(enf *rbac.Enforcer) {
		_ = enf.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
		enf.SetDefaultRole("role:admin")
	}
	appServer := newTestAppServerWithEnforcerConfigure(t, f,
		map[string]string{"application.sync.requireOverridePrivilegeForRevisionSync": "true"})
	_ = appServer.enf.SetBuiltinPolicy(`
        p, test-user, applications, get, default/*, allow
        p, test-user, applications, create, default/*, allow
        p, test-user, applications, sync, default/*, allow
        p, test-user, applications, override, default/*, deny
    `)

	// create an app and sync to the same revision
	testApp := newTestApp()
	app, err := appServer.Create(ctx,
		&application.ApplicationCreateRequest{Application: testApp})
	require.NoError(t, err)
	syncReq := &application.ApplicationSyncRequest{
		Name:     &app.Name,
		Revision: ptr.To("HEAD"),
	}
	syncedApp, err := appServer.Sync(ctx, syncReq)
	require.NoError(t, err,
		"should be able to sync to the same revision without override permission")
	assert.NotNil(t, syncedApp)

	// same for multi-source app
	multiSourceApp := newMultiSourceTestApp()
	app, err = appServer.Create(ctx,
		&application.ApplicationCreateRequest{Application: multiSourceApp})
	require.NoError(t, err)
	syncReq = &application.ApplicationSyncRequest{
		Name:            &app.Name,
		SourcePositions: []int64{1, 2},
		Revisions:       []string{"appbranch1", "appbranch2"},
	}
	syncedApp, err = appServer.Sync(ctx, syncReq)
	require.NoError(t, err,
		"should be able to sync to the same revision without override permission, multi-source app")
	assert.NotNil(t, syncedApp)
}

func TestSyncRBACOverrideRequired_WithoutRevisionAllowed(t *testing.T) {
	ctx := t.Context()
	//nolint:staticcheck
	ctx = context.WithValue(ctx, "claims", &jwt.RegisteredClaims{Subject: "test-user"})

	f := func(enf *rbac.Enforcer) {
		_ = enf.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
		enf.SetDefaultRole("role:admin")
	}
	appServer := newTestAppServerWithEnforcerConfigure(t, f,
		map[string]string{"application.sync.requireOverridePrivilegeForRevisionSync": "true"})

	_ = appServer.enf.SetBuiltinPolicy(`
        p, test-user, applications, get, default/*, allow
        p, test-user, applications, create, default/*, allow
        p, test-user, applications, sync, default/*, allow
        p, test-user, applications, override, default/*, deny
    `)

	testApp := newTestApp()
	app, err := appServer.Create(ctx,
		&application.ApplicationCreateRequest{Application: testApp})
	require.NoError(t, err)
	syncReq := &application.ApplicationSyncRequest{
		Name: &app.Name,
	}
	syncedApp, err := appServer.Sync(ctx, syncReq)
	require.NoError(t, err)
	assert.NotNil(t, syncedApp)
	assert.Equal(t, app.Spec, testApp.Spec)

	// same for multi-source app
	multiSourceApp := newMultiSourceTestApp()
	app, err = appServer.Create(ctx,
		&application.ApplicationCreateRequest{Application: multiSourceApp})
	require.NoError(t, err)
	syncReq = &application.ApplicationSyncRequest{
		Name: &app.Name,
	}
	syncedApp, err = appServer.Sync(ctx, syncReq)
	require.NoError(t, err)
	assert.NotNil(t, syncedApp)
}

func TestSyncRBACOverrideGranted_DiffRevisionAllowed(t *testing.T) {
	ctx := t.Context()
	//nolint:staticcheck
	ctx = context.WithValue(ctx, "claims", &jwt.RegisteredClaims{Subject: "test-user"})

	f := func(enf *rbac.Enforcer) {
		_ = enf.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
		enf.SetDefaultRole("role:admin")
	}
	appServer := newTestAppServerWithEnforcerConfigure(t, f,
		map[string]string{"application.sync.requireOverridePrivilegeForRevisionSync": "true"})

	_ = appServer.enf.SetBuiltinPolicy(`
        p, test-user, applications, get, default/*, allow
        p, test-user, applications, create, default/*, allow
        p, test-user, applications, sync, default/*, allow
        p, test-user, applications, override, default/*, allow
    `)

	testApp := newTestApp()

	app, err := appServer.Create(ctx,
		&application.ApplicationCreateRequest{Application: testApp})
	require.NoError(t, err)
	syncReq := &application.ApplicationSyncRequest{
		Name:     &app.Name,
		Revision: ptr.To("revisionbranch"),
	}
	syncedApp, err := appServer.Sync(ctx, syncReq)
	require.NoError(t, err,
		"should be able to sync to different revision with override permission")
	assert.NotNil(t, syncedApp)
	assert.Equal(t, "HEAD", syncedApp.Spec.Source.TargetRevision)

	// same for multi-source app
	multiSourceApp := newMultiSourceTestApp()
	app, err = appServer.Create(ctx,
		&application.ApplicationCreateRequest{Application: multiSourceApp})
	require.NoError(t, err)
	syncReq = &application.ApplicationSyncRequest{
		Name:            &app.Name,
		SourcePositions: []int64{1, 2},
		Revisions:       []string{"appbranch1", "appbranch2"},
	}
	syncedApp, err = appServer.Sync(ctx, syncReq)
	require.NoError(t, err,
		"should be able to sync to different revision with override permission, multi-source app")
	assert.NotNil(t, syncedApp)
}

func TestSyncMultiSource_PosTooLarge(t *testing.T) {
	ctx := t.Context()
	//nolint:staticcheck
	ctx = context.WithValue(ctx, "claims", &jwt.RegisteredClaims{Subject: "test-user"})
	appServer := newTestAppServer(t)

	_ = appServer.enf.SetBuiltinPolicy(`
    p, test-user, applications, get, default/*, allow
    p, test-user, applications, create, default/*, allow
    p, test-user, applications, sync, default/*, allow
    `)
	// create a new app
	multiSourceApp := newMultiSourceTestApp()
	app, err := appServer.Create(ctx,
		&application.ApplicationCreateRequest{Application: multiSourceApp})
	require.NoError(t, err)
	// sync with source position greater than the number of sources
	syncReq := &application.ApplicationSyncRequest{
		Name:            &app.Name,
		SourcePositions: []int64{3, 2},
		Revisions:       []string{"appbranch1", "appbranch2"},
	}
	_, err = appServer.Sync(ctx, syncReq)
	require.EqualError(t, err, "source position is out of range",
		"should fail because source position is greater than the number of sources")
}

func TestSyncMultiSource_PosTooSmall(t *testing.T) {
	ctx := t.Context()
	//nolint:staticcheck
	ctx = context.WithValue(ctx, "claims", &jwt.RegisteredClaims{Subject: "test-user"})
	appServer := newTestAppServer(t)

	_ = appServer.enf.SetBuiltinPolicy(`
    p, test-user, applications, get, default/*, allow
    p, test-user, applications, create, default/*, allow
    p, test-user, applications, sync, default/*, allow
    `)
	// create a new app
	multiSourceApp := newMultiSourceTestApp()
	app, err := appServer.Create(ctx,
		&application.ApplicationCreateRequest{Application: multiSourceApp})
	require.NoError(t, err)
	// sync with source position less than 1
	// this should fail because source positions are 1-based
	syncReq := &application.ApplicationSyncRequest{
		Name:            &app.Name,
		SourcePositions: []int64{0, 2},
		Revisions:       []string{"appbranch1", "appbranch2"},
	}
	_, err = appServer.Sync(ctx, syncReq)
	require.EqualError(t, err, "source position is out of range",
		"should fail because source position is less than 1")
}

func TestSync_SyncWithoutSyncPermissionShouldFail(t *testing.T) {
	ctx := t.Context()
	//nolint:staticcheck
	ctx = context.WithValue(ctx, "claims", &jwt.RegisteredClaims{Subject: "test-user"})
	appServer := newTestAppServer(t)
	_ = appServer.enf.SetBuiltinPolicy(`
		p, test-user, applications, get, default/*, allow
		p, test-user, applications, create, default/*, allow
		p, test-user, applications, sync, default/*, deny`)

	testApp := newTestApp()
	app, err := appServer.Create(ctx,
		&application.ApplicationCreateRequest{Application: testApp})
	require.NoError(t, err)
	syncReq := &application.ApplicationSyncRequest{
		Name: &app.Name,
	}
	_, err = appServer.Sync(ctx, syncReq)
	assert.Equal(t, codes.PermissionDenied.String(), status.Code(err).String())

	// same for multi-source app
	multiSourceApp := newMultiSourceTestApp()
	app, err = appServer.Create(ctx,
		&application.ApplicationCreateRequest{Application: multiSourceApp})
	require.NoError(t, err)
	syncReq = &application.ApplicationSyncRequest{
		Name: &app.Name,
	}
	_, err = appServer.Sync(ctx, syncReq)
	assert.Equal(t, codes.PermissionDenied.String(), status.Code(err).String())
}

func TestSyncRBACSettingsError(t *testing.T) {
	ctx := t.Context()
	//nolint:staticcheck
	ctx = context.WithValue(ctx, "claims", &jwt.RegisteredClaims{Subject: "test-user"})
	appServer := newTestAppServer(t)
	_ = appServer.enf.SetBuiltinPolicy(`
		p, test-user, applications, get, default/*, allow
		p, test-user, applications, create, default/*, allow
		p, test-user, applications, sync, default/*, allow
		p, test-user, applications, override, default/*, deny
	`)

	// create a new app
	testApp := newTestApp()
	app, err := appServer.Create(ctx,
		&application.ApplicationCreateRequest{Application: testApp})
	require.NoError(t, err)

	// override settings manager to return error
	brokenclientset := fake.NewClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-secret",
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"admin.password":   []byte("test"),
			"server.secretkey": []byte("test"),
		},
	})
	appServer.settingsMgr = settings.NewSettingsManager(ctx, brokenclientset, testNamespace)
	// and sync to different revision
	syncReq := &application.ApplicationSyncRequest{
		Name:     &app.Name,
		Revision: ptr.To("revisionbranch"),
	}

	_, err2 := appServer.Sync(ctx, syncReq)
	require.Error(t, err2)
	require.ErrorContains(t, err2, "error getting setting")
}

func TestSyncRBACOverrideFalse_DiffRevNoOverrideAllowed(t *testing.T) {
	ctx := t.Context()
	//nolint:staticcheck
	ctx = context.WithValue(ctx, "claims", &jwt.RegisteredClaims{Subject: "test-user"})
	appServer := newTestAppServer(t)
	_ = appServer.enf.SetBuiltinPolicy(`
        p, test-user, applications, get, default/*, allow
        p, test-user, applications, create, default/*, allow
        p, test-user, applications, sync, default/*, allow
        p, test-user, applications, override, default/*, deny
    `)

	// create a new app
	testApp := newTestApp()
	app, err := appServer.Create(ctx,
		&application.ApplicationCreateRequest{Application: testApp})
	require.NoError(t, err)
	// and sync to different revision
	syncReq := &application.ApplicationSyncRequest{
		Name:     &app.Name,
		Revision: ptr.To("revisionbranch"),
	}

	syncedApp, err := appServer.Sync(ctx, syncReq)
	require.NoError(t, err)
	assert.NotNil(t, syncedApp)
	assert.Equal(t, "HEAD", syncedApp.Spec.Source.TargetRevision)

	// same for multi-source app

	multiSourceApp := newMultiSourceTestApp()
	app, err = appServer.Create(ctx,
		&application.ApplicationCreateRequest{Application: multiSourceApp})
	require.NoError(t, err)
	syncReq = &application.ApplicationSyncRequest{
		Name:            &app.Name,
		SourcePositions: []int64{1},
		Revisions:       []string{"revisionbranch1"},
	}
	syncedApp, err = appServer.Sync(ctx, syncReq)
	require.NoError(t, err)
	assert.NotNil(t, syncedApp)
	// Sync to different revision must not change app spec
	assert.Equal(t, "appbranch1", syncedApp.Spec.Sources[0].TargetRevision)
	assert.Equal(t, "appbranch2", syncedApp.Spec.Sources[1].TargetRevision)
}

func TestSyncRBACOverrideNotRequired_SameRevisionAllowed(t *testing.T) {
	ctx := t.Context()
	//nolint:staticcheck
	ctx = context.WithValue(ctx, "claims", &jwt.RegisteredClaims{Subject: "test-user"})
	appServer := newTestAppServer(t)
	_ = appServer.enf.SetBuiltinPolicy(`
        p, test-user, applications, get, default/*, allow
        p, test-user, applications, create, default/*, allow
        p, test-user, applications, sync, default/*, allow
        p, test-user, applications, override, default/*, deny`)

	// create a new app
	testApp := newTestApp()
	app, err := appServer.Create(ctx, &application.ApplicationCreateRequest{Application: testApp})
	require.NoError(t, err)
	syncReq := &application.ApplicationSyncRequest{
		Name:     &app.Name,
		Revision: ptr.To("HEAD"),
	}

	syncedApp, err := appServer.Sync(ctx, syncReq)
	require.NoError(t, err,
		"should be able to sync to the same revision without override permission")
	assert.NotNil(t, syncedApp)

	// same for multi-source app
	multiSourceApp := newMultiSourceTestApp()
	app, err = appServer.Create(ctx, &application.ApplicationCreateRequest{Application: multiSourceApp})
	require.NoError(t, err)
	syncReq = &application.ApplicationSyncRequest{
		Name:            &app.Name,
		SourcePositions: []int64{1, 2},
		Revisions:       []string{"appbranch1", "appbranch2"},
	}
	syncedApp, err = appServer.Sync(ctx, syncReq)
	require.NoError(t, err,
		"should be able to sync to the same revision without override permission, multi-source app")
	assert.NotNil(t, syncedApp)
}

func TestSyncRBACOverrideNotRequired_EmptyRevisionAllowed(t *testing.T) {
	ctx := t.Context()
	//nolint:staticcheck
	ctx = context.WithValue(ctx, "claims", &jwt.RegisteredClaims{Subject: "test-user"})
	appServer := newTestAppServer(t)
	_ = appServer.enf.SetBuiltinPolicy(`
        p, test-user, applications, get, default/*, allow
        p, test-user, applications, create, default/*, allow
        p, test-user, applications, sync, default/*, allow
        p, test-user, applications, override, default/*, deny
    `)

	// create a new app
	testApp := newTestApp()
	app, err := appServer.Create(ctx, &application.ApplicationCreateRequest{Application: testApp})
	require.NoError(t, err)
	syncReq := &application.ApplicationSyncRequest{
		Name: &app.Name,
	}

	syncedApp, err := appServer.Sync(ctx, syncReq)
	require.NoError(t, err,
		"should be able to sync with empty revision without override permission")
	assert.NotNil(t, app)
	assert.Equal(t, "HEAD", syncedApp.Spec.Source.TargetRevision)

	// same for multi-source app
	multiSourceApp := newMultiSourceTestApp()
	app, err = appServer.Create(ctx, &application.ApplicationCreateRequest{Application: multiSourceApp})
	require.NoError(t, err)
	syncReq = &application.ApplicationSyncRequest{
		Name: &app.Name,
	}
	syncedApp, err = appServer.Sync(ctx, syncReq)
	require.NoError(t, err,
		"should be able to sync with empty revision without override permission, multi-source app")
	assert.NotNil(t, syncedApp)
	assert.Equal(t, "appbranch1", syncedApp.Spec.Sources[0].TargetRevision)
	assert.Equal(t, "appbranch2", syncedApp.Spec.Sources[1].TargetRevision)
}

func TestSyncRBACOverrideNotRequired_DiffRevisionAllowed(t *testing.T) {
	ctx := t.Context()
	//nolint:staticcheck
	ctx = context.WithValue(ctx, "claims", &jwt.RegisteredClaims{Subject: "test-user"})
	appServer := newTestAppServer(t)
	_ = appServer.enf.SetBuiltinPolicy(`
        p, test-user, applications, get, default/*, allow
        p, test-user, applications, create, default/*, allow
        p, test-user, applications, sync, default/*, allow
        p, test-user, applications, override, default/*, allow
    `)
	// create a new app
	testApp := newTestApp()
	app, err := appServer.Create(ctx, &application.ApplicationCreateRequest{Application: testApp})
	require.NoError(t, err)

	syncReq := &application.ApplicationSyncRequest{
		Name:     &app.Name,
		Revision: ptr.To("revisionbranch"),
	}

	syncedApp, err := appServer.Sync(ctx, syncReq)
	require.NoError(t, err,
		"should be able to sync to different revision with override permission")
	assert.NotNil(t, syncedApp)
	// Sync must not change app spec
	assert.Equal(t, "HEAD", syncedApp.Spec.Source.TargetRevision)

	// same for multi-source app
	multiSourceApp := newMultiSourceTestApp()
	app, err = appServer.Create(ctx, &application.ApplicationCreateRequest{Application: multiSourceApp})
	require.NoError(t, err)
	syncReq = &application.ApplicationSyncRequest{
		Name: &app.Name,
	}
	syncedApp, err = appServer.Sync(ctx, syncReq)
	require.NoError(t, err,
		"should be able to sync to different revision with override permission, multi-source app")
	assert.NotNil(t, syncedApp)
	assert.Equal(t, "appbranch1", syncedApp.Spec.Sources[0].TargetRevision)
	assert.Equal(t, "appbranch2", syncedApp.Spec.Sources[1].TargetRevision)
}

func TestSyncRBACOverrideNotRequired_DiffRevisionWithAutosyncPrevented(t *testing.T) {
	ctx := t.Context()
	//nolint:staticcheck
	ctx = context.WithValue(ctx, "claims", &jwt.RegisteredClaims{Subject: "test-user"})
	appServer := newTestAppServer(t)
	_ = appServer.enf.SetBuiltinPolicy(`
        p, test-user, applications, get, default/*, allow
        p, test-user, applications, create, default/*, allow
        p, test-user, applications, sync, default/*, allow
        p, test-user, applications, override, default/*, allow
    `)

	// create a new app
	testApp := newTestApp()
	testApp.Spec.SyncPolicy = &v1alpha1.SyncPolicy{
		Automated: &v1alpha1.SyncPolicyAutomated{
			Prune:    true,
			SelfHeal: true,
		},
	}
	app, err := appServer.Create(ctx, &application.ApplicationCreateRequest{Application: testApp})
	require.NoError(t, err)
	syncReq := &application.ApplicationSyncRequest{
		Name:     &app.Name,
		Revision: ptr.To("revisionbranch"),
	}

	_, err = appServer.Sync(ctx, syncReq)
	require.EqualError(t, err, "rpc error: code = FailedPrecondition desc = Cannot sync to revisionbranch: auto-sync currently set to HEAD",
		"should not be able to sync to different revision with auto-sync enabled")

	// same for multi-source app
	multiSourceApp := newMultiSourceTestApp()
	multiSourceApp.Spec.SyncPolicy = &v1alpha1.SyncPolicy{
		Automated: &v1alpha1.SyncPolicyAutomated{
			Prune:    true,
			SelfHeal: true,
		},
	}

	app, err = appServer.Create(ctx, &application.ApplicationCreateRequest{Application: multiSourceApp})
	require.NoError(t, err)
	syncReq = &application.ApplicationSyncRequest{
		Name:            &app.Name,
		SourcePositions: []int64{1},
		Revisions:       []string{"revisionbranch1"},
	}
	_, err = appServer.Sync(ctx, syncReq)
	assert.EqualError(t, err, "rpc error: code = FailedPrecondition desc = Cannot sync source https://github.com/argoproj/argocd-example-apps.git to revisionbranch1: auto-sync currently set to appbranch1",
		"should not be able to sync to different revision with auto-sync enabled, multi-source app")
}

func TestSyncAndTerminate(t *testing.T) {
	ctx := t.Context()
	appServer := newTestAppServer(t)
	testApp := newTestApp()
	testApp.Spec.Source.RepoURL = "https://github.com/argoproj/argo-cd.git"
	createReq := application.ApplicationCreateRequest{
		Application: testApp,
	}
	app, err := appServer.Create(ctx, &createReq)
	require.NoError(t, err)
	app, err = appServer.Sync(ctx, &application.ApplicationSyncRequest{Name: &app.Name})
	require.NoError(t, err)
	assert.NotNil(t, app)
	assert.NotNil(t, app.Operation)
	assert.Equal(t, testApp.Spec.GetSource(), *app.Operation.Sync.Source)

	events, err := appServer.kubeclientset.CoreV1().Events(appServer.ns).List(t.Context(), metav1.ListOptions{})
	require.NoError(t, err)
	event := events.Items[1]

	assert.Regexp(t, ".*initiated sync to HEAD \\([0-9A-Fa-f]{40}\\).*", event.Message)

	// set status.operationState to pretend that an operation has started by controller
	app.Status.OperationState = &v1alpha1.OperationState{
		Operation: *app.Operation,
		Phase:     synccommon.OperationRunning,
		StartedAt: metav1.NewTime(time.Now()),
	}
	_, err = appServer.appclientset.ArgoprojV1alpha1().Applications(appServer.ns).Update(t.Context(), app, metav1.UpdateOptions{})
	require.NoError(t, err)

	resp, err := appServer.TerminateOperation(ctx, &application.OperationTerminateRequest{Name: &app.Name})
	require.NoError(t, err)
	assert.NotNil(t, resp)

	app, err = appServer.Get(ctx, &application.ApplicationQuery{Name: &app.Name})
	require.NoError(t, err)
	assert.NotNil(t, app)
	assert.Equal(t, synccommon.OperationTerminating, app.Status.OperationState.Phase)
}

func TestSyncHelm(t *testing.T) {
	ctx := t.Context()
	appServer := newTestAppServer(t)
	testApp := newTestApp()
	testApp.Spec.Source.RepoURL = "https://argoproj.github.io/argo-helm"
	testApp.Spec.Source.Path = ""
	testApp.Spec.Source.Chart = "argo-cd"
	testApp.Spec.Source.TargetRevision = "0.7.*"

	appServer.repoClientset = &mocks.Clientset{RepoServerServiceClient: fakeRepoServerClient(true)}

	app, err := appServer.Create(ctx, &application.ApplicationCreateRequest{Application: testApp})
	require.NoError(t, err)

	app, err = appServer.Sync(ctx, &application.ApplicationSyncRequest{Name: &app.Name})
	require.NoError(t, err)
	assert.NotNil(t, app)
	assert.NotNil(t, app.Operation)

	events, err := appServer.kubeclientset.CoreV1().Events(appServer.ns).List(t.Context(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.Equal(t, "Unknown user initiated sync to 0.7.* (0.7.2)", events.Items[1].Message)
}

func TestSyncGit(t *testing.T) {
	ctx := t.Context()
	appServer := newTestAppServer(t)
	testApp := newTestApp()
	testApp.Spec.Source.RepoURL = "https://github.com/org/test"
	testApp.Spec.Source.Path = "deploy"
	testApp.Spec.Source.TargetRevision = "0.7.*"
	app, err := appServer.Create(ctx, &application.ApplicationCreateRequest{Application: testApp})
	require.NoError(t, err)
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
	require.NoError(t, err)
	assert.NotNil(t, app)
	assert.NotNil(t, app.Operation)
	events, err := appServer.kubeclientset.CoreV1().Events(appServer.ns).List(t.Context(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.Equal(t, "Unknown user initiated sync locally", events.Items[1].Message)
}

func TestSync_WithRefresh(t *testing.T) {
	ctx := t.Context()
	appServer := newTestAppServer(t)
	testApp := newTestApp()
	testApp.Spec.SyncPolicy = &v1alpha1.SyncPolicy{
		Retry: &v1alpha1.RetryStrategy{
			Refresh: true,
		},
	}
	testApp.Spec.Source.RepoURL = "https://github.com/argoproj/argo-cd.git"
	createReq := application.ApplicationCreateRequest{
		Application: testApp,
	}
	app, err := appServer.Create(ctx, &createReq)
	require.NoError(t, err)
	app, err = appServer.Sync(ctx, &application.ApplicationSyncRequest{Name: &app.Name})
	require.NoError(t, err)
	assert.NotNil(t, app)
	assert.NotNil(t, app.Operation)
	assert.True(t, app.Operation.Retry.Refresh)
}

func TestGetManifests_WithNoCache(t *testing.T) {
	testApp := newTestApp()
	appServer := newTestAppServer(t, testApp)

	mockRepoServiceClient := mocks.NewRepoServerServiceClient(t)
	mockRepoServiceClient.EXPECT().GenerateManifest(mock.Anything, mock.MatchedBy(func(mr *apiclient.ManifestRequest) bool {
		return mr.NoCache
	})).Return(&apiclient.ManifestResponse{}, nil)

	appServer.repoClientset = &mocks.Clientset{RepoServerServiceClient: mockRepoServiceClient}

	_, err := appServer.GetManifests(t.Context(), &application.ApplicationManifestQuery{
		Name:    &testApp.Name,
		NoCache: ptr.To(true),
	})
	require.NoError(t, err)
}

func TestRollbackApp(t *testing.T) {
	testApp := newTestApp()
	testApp.Status.History = []v1alpha1.RevisionHistory{{
		ID:        1,
		Revision:  "abc",
		Revisions: []string{"abc"},
		Source:    *testApp.Spec.Source.DeepCopy(),
		Sources:   []v1alpha1.ApplicationSource{*testApp.Spec.Source.DeepCopy()},
	}}
	appServer := newTestAppServer(t, testApp)

	updatedApp, err := appServer.Rollback(t.Context(), &application.ApplicationRollbackRequest{
		Name: &testApp.Name,
		Id:   ptr.To(int64(1)),
	})

	require.NoError(t, err)

	assert.NotNil(t, updatedApp.Operation)
	assert.NotNil(t, updatedApp.Operation.Sync)
	assert.NotNil(t, updatedApp.Operation.Sync.Source)
	assert.Equal(t, testApp.Status.History[0].Source, *updatedApp.Operation.Sync.Source)
	assert.Equal(t, testApp.Status.History[0].Sources, updatedApp.Operation.Sync.Sources)
	assert.Equal(t, testApp.Status.History[0].Revision, updatedApp.Operation.Sync.Revision)
	assert.Equal(t, testApp.Status.History[0].Revisions, updatedApp.Operation.Sync.Revisions)
}

func TestRollbackApp_WithRefresh(t *testing.T) {
	testApp := newTestApp()
	testApp.Spec.SyncPolicy = &v1alpha1.SyncPolicy{
		Retry: &v1alpha1.RetryStrategy{
			Refresh: true,
		},
	}

	testApp.Status.History = []v1alpha1.RevisionHistory{{
		ID:       1,
		Revision: "abc",
		Source:   *testApp.Spec.Source.DeepCopy(),
	}}
	appServer := newTestAppServer(t, testApp)

	updatedApp, err := appServer.Rollback(t.Context(), &application.ApplicationRollbackRequest{
		Name: &testApp.Name,
		Id:   ptr.To(int64(1)),
	})

	require.NoError(t, err)

	assert.NotNil(t, updatedApp.Operation)
	assert.NotNil(t, updatedApp.Operation.Retry)
	assert.False(t, updatedApp.Operation.Retry.Refresh, "refresh should never be set on rollback")
}

func TestUpdateAppProject(t *testing.T) {
	testApp := newTestApp()
	ctx := t.Context()
	//nolint:staticcheck
	ctx = context.WithValue(ctx, "claims", &jwt.RegisteredClaims{Subject: "admin"})
	appServer := newTestAppServer(t, testApp)
	appServer.enf.SetDefaultRole("")

	t.Run("update without changing project", func(t *testing.T) {
		_ = appServer.enf.SetBuiltinPolicy(`p, admin, applications, update, default/test-app, allow`)
		_, err := appServer.Update(ctx, &application.ApplicationUpdateRequest{Application: testApp})
		require.NoError(t, err)
	})

	t.Run("cannot update to another project", func(t *testing.T) {
		testApp.Spec.Project = "my-proj"
		_, err := appServer.Update(ctx, &application.ApplicationUpdateRequest{Application: testApp})
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
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
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
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
		require.NoError(t, err)
		assert.Equal(t, "my-proj", updatedApp.Spec.Project)
	})
}

func TestAppJsonPatch(t *testing.T) {
	testApp := newTestAppWithAnnotations()
	ctx := t.Context()
	//nolint:staticcheck
	ctx = context.WithValue(ctx, "claims", &jwt.RegisteredClaims{Subject: "admin"})
	appServer := newTestAppServer(t, testApp)
	appServer.enf.SetDefaultRole("")

	app, err := appServer.Patch(ctx, &application.ApplicationPatchRequest{Name: &testApp.Name, Patch: ptr.To("garbage")})
	require.Error(t, err)
	assert.Nil(t, app)

	app, err = appServer.Patch(ctx, &application.ApplicationPatchRequest{Name: &testApp.Name, Patch: ptr.To("[]")})
	require.NoError(t, err)
	assert.NotNil(t, app)

	app, err = appServer.Patch(ctx, &application.ApplicationPatchRequest{Name: &testApp.Name, Patch: ptr.To(`[{"op": "replace", "path": "/spec/source/path", "value": "foo"}]`)})
	require.NoError(t, err)
	assert.Equal(t, "foo", app.Spec.Source.Path)

	app, err = appServer.Patch(ctx, &application.ApplicationPatchRequest{Name: &testApp.Name, Patch: ptr.To(`[{"op": "remove", "path": "/metadata/annotations/test.annotation"}]`)})
	require.NoError(t, err)
	assert.NotContains(t, app.Annotations, "test.annotation")
}

func TestAppMergePatch(t *testing.T) {
	testApp := newTestApp()
	ctx := t.Context()
	//nolint:staticcheck
	ctx = context.WithValue(ctx, "claims", &jwt.RegisteredClaims{Subject: "admin"})
	appServer := newTestAppServer(t, testApp)
	appServer.enf.SetDefaultRole("")

	app, err := appServer.Patch(ctx, &application.ApplicationPatchRequest{
		Name: &testApp.Name, Patch: ptr.To(`{"spec": { "source": { "path": "foo" } }}`), PatchType: ptr.To("merge"),
	})
	require.NoError(t, err)
	assert.Equal(t, "foo", app.Spec.Source.Path)
}

func TestServer_GetApplicationSyncWindowsState(t *testing.T) {
	t.Run("Active", func(t *testing.T) {
		testApp := newTestApp()
		testApp.Spec.Project = "proj-maint"
		appServer := newTestAppServer(t, testApp)

		active, err := appServer.GetApplicationSyncWindows(t.Context(), &application.ApplicationSyncWindowsQuery{Name: &testApp.Name})
		require.NoError(t, err)
		assert.Len(t, active.ActiveWindows, 1)
	})
	t.Run("Inactive", func(t *testing.T) {
		testApp := newTestApp()
		testApp.Spec.Project = "default"
		appServer := newTestAppServer(t, testApp)

		active, err := appServer.GetApplicationSyncWindows(t.Context(), &application.ApplicationSyncWindowsQuery{Name: &testApp.Name})
		require.NoError(t, err)
		assert.Empty(t, active.ActiveWindows)
	})
	t.Run("ProjectDoesNotExist", func(t *testing.T) {
		testApp := newTestApp()
		testApp.Spec.Project = "none"
		appServer := newTestAppServer(t, testApp)

		active, err := appServer.GetApplicationSyncWindows(t.Context(), &application.ApplicationSyncWindowsQuery{Name: &testApp.Name})
		require.ErrorContains(t, err, "not exist")
		assert.Nil(t, active)
	})
}

func TestGetCachedAppState(t *testing.T) {
	testApp := newTestApp()
	testApp.ResourceVersion = "1"
	testApp.Spec.Project = "test-proj"
	testProj := &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-proj",
			Namespace: testNamespace,
		},
	}
	appServer := newTestAppServer(t, testApp, testProj)
	fakeClientSet := appServer.appclientset.(*deepCopyAppClientset).GetUnderlyingClientSet().(*apps.Clientset)
	fakeClientSet.AddReactor("get", "applications", func(_ kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &v1alpha1.Application{Spec: v1alpha1.ApplicationSpec{Source: &v1alpha1.ApplicationSource{}}}, nil
	})
	t.Run("NoError", func(t *testing.T) {
		err := appServer.getCachedAppState(t.Context(), testApp, func() error {
			return nil
		})
		require.NoError(t, err)
	})
	t.Run("CacheMissErrorTriggersRefresh", func(t *testing.T) {
		retryCount := 0
		patched := false
		watcher := watch.NewFakeWithChanSize(1, true)

		// Configure fakeClientSet within lock, before requesting cached app state, to avoid data race
		fakeClientSet.Lock()
		fakeClientSet.ReactionChain = nil
		fakeClientSet.WatchReactionChain = nil
		fakeClientSet.AddReactor("patch", "applications", func(_ kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			patched = true
			updated := testApp.DeepCopy()
			updated.ResourceVersion = "2"
			appServer.appBroadcaster.OnUpdate(testApp, updated)
			return true, testApp, nil
		})
		fakeClientSet.AddReactor("get", "applications", func(_ kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, &v1alpha1.Application{Spec: v1alpha1.ApplicationSpec{Source: &v1alpha1.ApplicationSource{}}}, nil
		})
		fakeClientSet.Unlock()
		fakeClientSet.AddWatchReactor("applications", func(_ kubetesting.Action) (handled bool, ret watch.Interface, err error) {
			return true, watcher, nil
		})

		err := appServer.getCachedAppState(t.Context(), testApp, func() error {
			res := cache.ErrCacheMiss
			if retryCount == 1 {
				res = nil
			}
			retryCount++
			return res
		})
		require.NoError(t, err)
		assert.Equal(t, 2, retryCount)
		assert.True(t, patched)
	})

	t.Run("NonCacheErrorDoesNotTriggerRefresh", func(t *testing.T) {
		randomError := stderrors.New("random error")
		err := appServer.getCachedAppState(t.Context(), testApp, func() error {
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
		require.NoError(t, err)
		assert.Equal(t, specPatch, string(nonStatus))
		assert.Nil(t, status)
	}
	{
		nonStatus, status, err := splitStatusPatch([]byte(statusPatch))
		require.NoError(t, err)
		assert.Nil(t, nonStatus)
		assert.Equal(t, statusPatch, string(status))
	}
	{
		bothPatch := `{"spec":{"aaa":"bbb"},"status":{"ccc":"ddd"}}`
		nonStatus, status, err := splitStatusPatch([]byte(bothPatch))
		require.NoError(t, err)
		assert.Equal(t, specPatch, string(nonStatus))
		assert.Equal(t, statusPatch, string(status))
	}
	{
		otherFields := `{"operation":{"eee":"fff"},"spec":{"aaa":"bbb"},"status":{"ccc":"ddd"}}`
		nonStatus, status, err := splitStatusPatch([]byte(otherFields))
		require.NoError(t, err)
		assert.JSONEq(t, `{"operation":{"eee":"fff"},"spec":{"aaa":"bbb"}}`, string(nonStatus))
		assert.Equal(t, statusPatch, string(status))
	}
}

func TestLogsGetSelectedPod(t *testing.T) {
	deployment := v1alpha1.ResourceRef{Group: "", Version: "v1", Kind: "Deployment", Name: "deployment", UID: "1"}
	rs := v1alpha1.ResourceRef{Group: "", Version: "v1", Kind: "ReplicaSet", Name: "rs", UID: "2"}
	podRS := v1alpha1.ResourceRef{Group: "", Version: "v1", Kind: "Pod", Name: "podrs", UID: "3"}
	pod := v1alpha1.ResourceRef{Group: "", Version: "v1", Kind: "Pod", Name: "pod", UID: "4"}
	treeNodes := []v1alpha1.ResourceNode{
		{ResourceRef: deployment, ParentRefs: nil},
		{ResourceRef: rs, ParentRefs: []v1alpha1.ResourceRef{deployment}},
		{ResourceRef: podRS, ParentRefs: []v1alpha1.ResourceRef{rs}},
		{ResourceRef: pod, ParentRefs: nil},
	}
	appName := "appName"

	t.Run("GetAllPods", func(t *testing.T) {
		podQuery := application.ApplicationPodLogsQuery{
			Name: &appName,
		}
		pods := getSelectedPods(treeNodes, &podQuery)
		assert.Len(t, pods, 2)
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
		assert.Len(t, pods, 1)
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
		assert.Len(t, pods, 1)
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
		assert.Empty(t, pods)
	})
}

func TestMaxPodLogsRender(t *testing.T) {
	defaultMaxPodLogsToRender, _ := newTestAppServer(t).settingsMgr.GetMaxPodLogsToRender()

	// Case: number of pods to view logs is less than defaultMaxPodLogsToRender
	podNumber := int(defaultMaxPodLogsToRender - 1)
	appServer, adminCtx := createAppServerWithMaxLodLogs(t, podNumber)

	t.Run("PodLogs", func(t *testing.T) {
		err := appServer.PodLogs(&application.ApplicationPodLogsQuery{Name: ptr.To("test")}, &TestPodLogsServer{ctx: adminCtx})
		statusCode, _ := status.FromError(err)
		assert.Equal(t, codes.OK, statusCode.Code())
	})

	// Case: number of pods higher than defaultMaxPodLogsToRender
	podNumber = int(defaultMaxPodLogsToRender + 1)
	appServer, adminCtx = createAppServerWithMaxLodLogs(t, podNumber)

	t.Run("PodLogs", func(t *testing.T) {
		err := appServer.PodLogs(&application.ApplicationPodLogsQuery{Name: ptr.To("test")}, &TestPodLogsServer{ctx: adminCtx})
		require.Error(t, err)
		statusCode, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, statusCode.Code())
		assert.EqualError(t, err, "rpc error: code = InvalidArgument desc = max pods to view logs are reached. Please provide more granular query")
	})

	// Case: number of pods to view logs is less than customMaxPodLogsToRender
	customMaxPodLogsToRender := int64(15)
	podNumber = int(customMaxPodLogsToRender - 1)
	appServer, adminCtx = createAppServerWithMaxLodLogs(t, podNumber, customMaxPodLogsToRender)

	t.Run("PodLogs", func(t *testing.T) {
		err := appServer.PodLogs(&application.ApplicationPodLogsQuery{Name: ptr.To("test")}, &TestPodLogsServer{ctx: adminCtx})
		statusCode, _ := status.FromError(err)
		assert.Equal(t, codes.OK, statusCode.Code())
	})

	// Case: number of pods higher than customMaxPodLogsToRender
	customMaxPodLogsToRender = int64(15)
	podNumber = int(customMaxPodLogsToRender + 1)
	appServer, adminCtx = createAppServerWithMaxLodLogs(t, podNumber, customMaxPodLogsToRender)

	t.Run("PodLogs", func(t *testing.T) {
		err := appServer.PodLogs(&application.ApplicationPodLogsQuery{Name: ptr.To("test")}, &TestPodLogsServer{ctx: adminCtx})
		require.Error(t, err)
		statusCode, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, statusCode.Code())
		assert.EqualError(t, err, "rpc error: code = InvalidArgument desc = max pods to view logs are reached. Please provide more granular query")
	})
}

// createAppServerWithMaxLodLogs creates a new app server with given number of pods and resources
func createAppServerWithMaxLodLogs(t *testing.T, podNumber int, maxPodLogsToRender ...int64) (*Server, context.Context) {
	t.Helper()
	runtimeObjects := make([]runtime.Object, podNumber+1)
	resources := make([]v1alpha1.ResourceStatus, podNumber)

	for i := range podNumber {
		pod := corev1.Pod{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Pod",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("pod-%d", i),
				Namespace: "test",
			},
		}
		resources[i] = v1alpha1.ResourceStatus{
			Group:     pod.GroupVersionKind().Group,
			Kind:      pod.GroupVersionKind().Kind,
			Version:   pod.GroupVersionKind().Version,
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Status:    "Synced",
		}
		runtimeObjects[i] = kube.MustToUnstructured(&pod)
	}

	testApp := newTestApp(func(app *v1alpha1.Application) {
		app.Name = "test"
		app.Status.Resources = resources
	})
	runtimeObjects[podNumber] = testApp

	noRoleCtx := t.Context()
	//nolint:staticcheck
	adminCtx := context.WithValue(noRoleCtx, "claims", &jwt.MapClaims{"groups": []string{"admin"}})

	if len(maxPodLogsToRender) > 0 {
		f := func(enf *rbac.Enforcer) {
			_ = enf.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
			enf.SetDefaultRole("role:admin")
		}
		formatInt := strconv.FormatInt(maxPodLogsToRender[0], 10)
		appServer := newTestAppServerWithEnforcerConfigure(t, f, map[string]string{"server.maxPodLogsToRender": formatInt}, runtimeObjects...)
		return appServer, adminCtx
	}
	appServer := newTestAppServer(t, runtimeObjects...)
	return appServer, adminCtx
}

// refreshAnnotationRemover runs an infinite loop until it detects and removes refresh annotation or given context is done
func refreshAnnotationRemover(t *testing.T, ctx context.Context, patched *int32, appServer *Server, appName string, ch chan string) {
	t.Helper()
	for ctx.Err() == nil {
		aName, appNs := argo.ParseFromQualifiedName(appName, appServer.ns)
		a, err := appServer.appLister.Applications(appNs).Get(aName)
		require.NoError(t, err)
		if a.GetAnnotations() != nil && a.GetAnnotations()[v1alpha1.AnnotationKeyRefresh] != "" {
			a.SetAnnotations(map[string]string{})
			a.SetResourceVersion("999")
			_, err = appServer.appclientset.ArgoprojV1alpha1().Applications(a.Namespace).Update(
				t.Context(), a, metav1.UpdateOptions{})
			require.NoError(t, err)
			atomic.AddInt32(patched, 1)
			ch <- ""
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestGetAppRefresh_NormalRefresh(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	testApp := newTestApp()
	testApp.ResourceVersion = "1"
	appServer := newTestAppServer(t, testApp)

	var patched int32

	ch := make(chan string, 1)

	go refreshAnnotationRemover(t, ctx, &patched, appServer, testApp.Name, ch)

	_, err := appServer.Get(t.Context(), &application.ApplicationQuery{
		Name:    &testApp.Name,
		Refresh: ptr.To(string(v1alpha1.RefreshTypeNormal)),
	})
	require.NoError(t, err)

	select {
	case <-ch:
		assert.Equal(t, int32(1), atomic.LoadInt32(&patched))
	case <-time.After(10 * time.Second):
		assert.Fail(t, "Out of time ( 10 seconds )")
	}
}

func TestGetAppRefresh_HardRefresh(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	testApp := newTestApp()
	testApp.ResourceVersion = "1"
	appServer := newTestAppServer(t, testApp)

	var getAppDetailsQuery *apiclient.RepoServerAppDetailsQuery
	mockRepoServiceClient := &mocks.RepoServerServiceClient{}
	mockRepoServiceClient.EXPECT().GetAppDetails(mock.Anything, mock.MatchedBy(func(q *apiclient.RepoServerAppDetailsQuery) bool {
		getAppDetailsQuery = q
		return true
	})).Return(&apiclient.RepoAppDetailsResponse{}, nil)
	appServer.repoClientset = &mocks.Clientset{RepoServerServiceClient: mockRepoServiceClient}

	var patched int32

	ch := make(chan string, 1)

	go refreshAnnotationRemover(t, ctx, &patched, appServer, testApp.Name, ch)

	_, err := appServer.Get(t.Context(), &application.ApplicationQuery{
		Name:    &testApp.Name,
		Refresh: ptr.To(string(v1alpha1.RefreshTypeHard)),
	})
	require.NoError(t, err)
	require.NotNil(t, getAppDetailsQuery)
	assert.True(t, getAppDetailsQuery.NoCache)
	assert.Equal(t, testApp.Spec.Source, getAppDetailsQuery.Source)

	require.NoError(t, err)
	select {
	case <-ch:
		assert.Equal(t, int32(1), atomic.LoadInt32(&patched))
	case <-time.After(10 * time.Second):
		assert.Fail(t, "Out of time ( 10 seconds )")
	}
}

func TestGetApp_HealthStatusPropagation(t *testing.T) {
	newServerWithTree := func(t *testing.T) (*Server, *v1alpha1.Application) {
		t.Helper()
		cacheClient := cache.NewCache(cache.NewInMemoryCache(1 * time.Hour))

		testApp := newTestApp()
		testApp.Status.ResourceHealthSource = v1alpha1.ResourceHealthLocationAppTree
		testApp.Status.Resources = []v1alpha1.ResourceStatus{
			{
				Group:     "apps",
				Kind:      "Deployment",
				Name:      "guestbook",
				Namespace: "default",
			},
		}

		appServer := newTestAppServer(t, testApp)

		appStateCache := appstate.NewCache(cacheClient, time.Minute)
		appInstanceName := testApp.InstanceName(appServer.appNamespaceOrDefault(testApp.Namespace))
		err := appStateCache.SetAppResourcesTree(appInstanceName, &v1alpha1.ApplicationTree{
			Nodes: []v1alpha1.ResourceNode{{
				ResourceRef: v1alpha1.ResourceRef{
					Group:     "apps",
					Kind:      "Deployment",
					Name:      "guestbook",
					Namespace: "default",
				},
				Health: &v1alpha1.HealthStatus{Status: health.HealthStatusDegraded},
			}},
		})
		require.NoError(t, err)
		appServer.cache = servercache.NewCache(appStateCache, time.Minute, time.Minute)

		return appServer, testApp
	}

	t.Run("propagated health status on get with no refresh", func(t *testing.T) {
		appServer, testApp := newServerWithTree(t)
		fetchedApp, err := appServer.Get(t.Context(), &application.ApplicationQuery{
			Name: &testApp.Name,
		})
		require.NoError(t, err)
		assert.Equal(t, health.HealthStatusDegraded, fetchedApp.Status.Resources[0].Health.Status)
	})

	t.Run("propagated health status on normal refresh", func(t *testing.T) {
		appServer, testApp := newServerWithTree(t)
		var patched int32
		ch := make(chan string, 1)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		go refreshAnnotationRemover(t, ctx, &patched, appServer, testApp.Name, ch)

		fetchedApp, err := appServer.Get(t.Context(), &application.ApplicationQuery{
			Name:    &testApp.Name,
			Refresh: ptr.To(string(v1alpha1.RefreshTypeNormal)),
		})
		require.NoError(t, err)

		select {
		case <-ch:
			assert.Equal(t, int32(1), atomic.LoadInt32(&patched))
		case <-time.After(10 * time.Second):
			assert.Fail(t, "Out of time ( 10 seconds )")
		}
		assert.Equal(t, health.HealthStatusDegraded, fetchedApp.Status.Resources[0].Health.Status)
	})

	t.Run("propagated health status on hard refresh", func(t *testing.T) {
		appServer, testApp := newServerWithTree(t)
		var patched int32
		ch := make(chan string, 1)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		go refreshAnnotationRemover(t, ctx, &patched, appServer, testApp.Name, ch)

		fetchedApp, err := appServer.Get(t.Context(), &application.ApplicationQuery{
			Name:    &testApp.Name,
			Refresh: ptr.To(string(v1alpha1.RefreshTypeHard)),
		})
		require.NoError(t, err)

		select {
		case <-ch:
			assert.Equal(t, int32(1), atomic.LoadInt32(&patched))
		case <-time.After(10 * time.Second):
			assert.Fail(t, "Out of time ( 10 seconds )")
		}
		assert.Equal(t, health.HealthStatusDegraded, fetchedApp.Status.Resources[0].Health.Status)
	})
}

func TestInferResourcesStatusHealth(t *testing.T) {
	cacheClient := cache.NewCache(cache.NewInMemoryCache(1 * time.Hour))

	testApp := newTestApp()
	testApp.Status.ResourceHealthSource = v1alpha1.ResourceHealthLocationAppTree
	testApp.Status.Resources = []v1alpha1.ResourceStatus{{
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
	err := appStateCache.SetAppResourcesTree(testApp.Name, &v1alpha1.ApplicationTree{Nodes: []v1alpha1.ResourceNode{{
		ResourceRef: v1alpha1.ResourceRef{
			Group:     "apps",
			Kind:      "Deployment",
			Name:      "guestbook",
			Namespace: "default",
		},
		Health: &v1alpha1.HealthStatus{
			Status: health.HealthStatusDegraded,
		},
	}}})

	require.NoError(t, err)

	appServer.cache = servercache.NewCache(appStateCache, time.Minute, time.Minute)

	appServer.inferResourcesStatusHealth(testApp)

	assert.Equal(t, health.HealthStatusDegraded, testApp.Status.Resources[0].Health.Status)
	assert.Nil(t, testApp.Status.Resources[1].Health)
}

func TestInferResourcesStatusHealthWithAppInAnyNamespace(t *testing.T) {
	cacheClient := cache.NewCache(cache.NewInMemoryCache(1 * time.Hour))

	testApp := newTestApp()
	testApp.Namespace = "otherNamespace"
	testApp.Status.ResourceHealthSource = v1alpha1.ResourceHealthLocationAppTree
	testApp.Status.Resources = []v1alpha1.ResourceStatus{{
		Group:     "apps",
		Kind:      "Deployment",
		Name:      "guestbook",
		Namespace: "otherNamespace",
	}, {
		Group:     "apps",
		Kind:      "StatefulSet",
		Name:      "guestbook-stateful",
		Namespace: "otherNamespace",
	}}
	appServer := newTestAppServer(t, testApp)
	appStateCache := appstate.NewCache(cacheClient, time.Minute)
	err := appStateCache.SetAppResourcesTree("otherNamespace"+"_"+testApp.Name, &v1alpha1.ApplicationTree{Nodes: []v1alpha1.ResourceNode{{
		ResourceRef: v1alpha1.ResourceRef{
			Group:     "apps",
			Kind:      "Deployment",
			Name:      "guestbook",
			Namespace: "otherNamespace",
		},
		Health: &v1alpha1.HealthStatus{
			Status: health.HealthStatusDegraded,
		},
	}}})

	require.NoError(t, err)

	appServer.cache = servercache.NewCache(appStateCache, time.Minute, time.Minute)

	appServer.inferResourcesStatusHealth(testApp)

	assert.Equal(t, health.HealthStatusDegraded, testApp.Status.Resources[0].Health.Status)
	assert.Nil(t, testApp.Status.Resources[1].Health)
}

func TestRunNewStyleResourceAction(t *testing.T) {
	cacheClient := cache.NewCache(cache.NewInMemoryCache(1 * time.Hour))

	group := "batch"
	kind := "CronJob"
	version := "v1"
	resourceName := "my-cron-job"
	namespace := testNamespace
	action := "create-job"
	uid := "1"

	resources := []v1alpha1.ResourceStatus{{
		Group:     group,
		Kind:      kind,
		Name:      resourceName,
		Namespace: testNamespace,
		Version:   version,
	}}

	appStateCache := appstate.NewCache(cacheClient, time.Minute)

	nodes := []v1alpha1.ResourceNode{{
		ResourceRef: v1alpha1.ResourceRef{
			Group:     group,
			Kind:      kind,
			Version:   version,
			Name:      resourceName,
			Namespace: testNamespace,
			UID:       uid,
		},
	}}

	createJobDenyingProj := &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: "createJobDenyingProj", Namespace: "default"},
		Spec: v1alpha1.AppProjectSpec{
			SourceRepos:                []string{"*"},
			Destinations:               []v1alpha1.ApplicationDestination{{Server: "*", Namespace: "*"}},
			NamespaceResourceWhitelist: []metav1.GroupKind{{Group: "never", Kind: "mind"}},
		},
	}

	cronJob := k8sbatchv1.CronJob{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "CronJob",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-cron-job",
			Namespace: testNamespace,
			Labels: map[string]string{
				"some": "label",
			},
		},
		Spec: k8sbatchv1.CronJobSpec{
			Schedule: "* * * * *",
			JobTemplate: k8sbatchv1.JobTemplateSpec{
				Spec: k8sbatchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:            "hello",
									Image:           "busybox:1.28",
									ImagePullPolicy: "IfNotPresent",
									Command:         []string{"/bin/sh", "-c", "date; echo Hello from the Kubernetes cluster"},
								},
							},
							RestartPolicy: corev1.RestartPolicyOnFailure,
						},
					},
				},
			},
		},
	}

	t.Run("CreateOperationNotPermitted", func(t *testing.T) {
		testApp := newTestApp()
		testApp.Spec.Project = "createJobDenyingProj"
		testApp.Status.ResourceHealthSource = v1alpha1.ResourceHealthLocationAppTree
		testApp.Status.Resources = resources

		appServer := newTestAppServer(t, testApp, createJobDenyingProj, kube.MustToUnstructured(&cronJob))
		appServer.cache = servercache.NewCache(appStateCache, time.Minute, time.Minute)

		err := appStateCache.SetAppResourcesTree(testApp.Name, &v1alpha1.ApplicationTree{Nodes: nodes})
		require.NoError(t, err)

		appResponse, runErr := appServer.RunResourceActionV2(t.Context(), &application.ResourceActionRunRequestV2{
			Name:         &testApp.Name,
			Namespace:    &namespace,
			Action:       &action,
			AppNamespace: &testApp.Namespace,
			ResourceName: &resourceName,
			Version:      &version,
			Group:        &group,
			Kind:         &kind,
		})

		require.ErrorContains(t, runErr, "is not permitted to manage")
		assert.Nil(t, appResponse)
	})

	t.Run("CreateOperationPermitted", func(t *testing.T) {
		testApp := newTestApp()
		testApp.Status.ResourceHealthSource = v1alpha1.ResourceHealthLocationAppTree
		testApp.Status.Resources = resources

		appServer := newTestAppServer(t, testApp, kube.MustToUnstructured(&cronJob))
		appServer.cache = servercache.NewCache(appStateCache, time.Minute, time.Minute)

		err := appStateCache.SetAppResourcesTree(testApp.Name, &v1alpha1.ApplicationTree{Nodes: nodes})
		require.NoError(t, err)

		appResponse, runErr := appServer.RunResourceActionV2(t.Context(), &application.ResourceActionRunRequestV2{
			Name:         &testApp.Name,
			Namespace:    &namespace,
			Action:       &action,
			AppNamespace: &testApp.Namespace,
			ResourceName: &resourceName,
			Version:      &version,
			Group:        &group,
			Kind:         &kind,
		})

		require.NoError(t, runErr)
		assert.NotNil(t, appResponse)
	})
}

func TestRunOldStyleResourceAction(t *testing.T) {
	cacheClient := cache.NewCache(cache.NewInMemoryCache(1 * time.Hour))

	group := "apps"
	kind := "Deployment"
	version := "v1"
	resourceName := "nginx-deploy"
	namespace := testNamespace
	action := "pause"
	uid := "2"

	resources := []v1alpha1.ResourceStatus{{
		Group:     group,
		Kind:      kind,
		Name:      resourceName,
		Namespace: testNamespace,
		Version:   version,
	}}

	appStateCache := appstate.NewCache(cacheClient, time.Minute)

	nodes := []v1alpha1.ResourceNode{{
		ResourceRef: v1alpha1.ResourceRef{
			Group:     group,
			Kind:      kind,
			Version:   version,
			Name:      resourceName,
			Namespace: testNamespace,
			UID:       uid,
		},
	}}

	deployment := appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx-deploy",
			Namespace: testNamespace,
		},
	}

	t.Run("DefaultPatchOperation", func(t *testing.T) {
		testApp := newTestApp()
		testApp.Status.ResourceHealthSource = v1alpha1.ResourceHealthLocationAppTree
		testApp.Status.Resources = resources

		// appServer := newTestAppServer(t, testApp, returnDeployment())
		appServer := newTestAppServer(t, testApp, kube.MustToUnstructured(&deployment))
		appServer.cache = servercache.NewCache(appStateCache, time.Minute, time.Minute)

		err := appStateCache.SetAppResourcesTree(testApp.Name, &v1alpha1.ApplicationTree{Nodes: nodes})
		require.NoError(t, err)

		appResponse, runErr := appServer.RunResourceActionV2(t.Context(), &application.ResourceActionRunRequestV2{
			Name:         &testApp.Name,
			Namespace:    &namespace,
			Action:       &action,
			AppNamespace: &testApp.Namespace,
			ResourceName: &resourceName,
			Version:      &version,
			Group:        &group,
			Kind:         &kind,
		})

		require.NoError(t, runErr)
		assert.NotNil(t, appResponse)
	})
}

func TestIsApplicationPermitted(t *testing.T) {
	t.Run("Incorrect project", func(t *testing.T) {
		testApp := newTestApp()
		appServer := newTestAppServer(t, testApp)
		projects := map[string]bool{"test-app": false}
		permitted := appServer.isApplicationPermitted(labels.Everything(), 0, nil, "test", "default", projects, *testApp)
		assert.False(t, permitted)
	})

	t.Run("Version is incorrect", func(t *testing.T) {
		testApp := newTestApp()
		appServer := newTestAppServer(t, testApp)
		minVersion := 100000
		testApp.ResourceVersion = strconv.Itoa(minVersion - 1)
		permitted := appServer.isApplicationPermitted(labels.Everything(), minVersion, nil, "test", "default", nil, *testApp)
		assert.False(t, permitted)
	})

	t.Run("Application name is incorrect", func(t *testing.T) {
		testApp := newTestApp()
		appServer := newTestAppServer(t, testApp)
		appName := "test"
		permitted := appServer.isApplicationPermitted(labels.Everything(), 0, nil, appName, "default", nil, *testApp)
		assert.False(t, permitted)
	})

	t.Run("Application namespace is incorrect", func(t *testing.T) {
		testApp := newTestApp()
		appServer := newTestAppServer(t, testApp)
		permitted := appServer.isApplicationPermitted(labels.Everything(), 0, nil, testApp.Name, "demo", nil, *testApp)
		assert.False(t, permitted)
	})

	t.Run("Application is not part of enabled namespace", func(t *testing.T) {
		testApp := newTestApp()
		appServer := newTestAppServer(t, testApp)
		appServer.ns = "server-ns"
		appServer.enabledNamespaces = []string{"demo"}
		permitted := appServer.isApplicationPermitted(labels.Everything(), 0, nil, testApp.Name, testApp.Namespace, nil, *testApp)
		assert.False(t, permitted)
	})

	t.Run("Application is part of enabled namespace", func(t *testing.T) {
		testApp := newTestApp()
		appServer := newTestAppServer(t, testApp)
		appServer.ns = "server-ns"
		appServer.enabledNamespaces = []string{testApp.Namespace}
		permitted := appServer.isApplicationPermitted(labels.Everything(), 0, nil, testApp.Name, testApp.Namespace, nil, *testApp)
		assert.True(t, permitted)
	})
}

func TestAppNamespaceRestrictions(t *testing.T) {
	t.Parallel()

	t.Run("List applications in controller namespace", func(t *testing.T) {
		t.Parallel()
		testApp := newTestApp()
		appServer := newTestAppServer(t, testApp)
		apps, err := appServer.List(t.Context(), &application.ApplicationQuery{})
		require.NoError(t, err)
		require.Len(t, apps.Items, 1)
	})

	t.Run("List applications with non-allowed apps existing", func(t *testing.T) {
		t.Parallel()
		testApp1 := newTestApp()
		testApp1.Namespace = "argocd-1"
		appServer := newTestAppServer(t, testApp1)
		apps, err := appServer.List(t.Context(), &application.ApplicationQuery{})
		require.NoError(t, err)
		require.Empty(t, apps.Items)
	})

	t.Run("List applications with non-allowed apps existing and explicit ns request", func(t *testing.T) {
		t.Parallel()
		testApp1 := newTestApp()
		testApp2 := newTestApp()
		testApp2.Namespace = "argocd-1"
		appServer := newTestAppServer(t, testApp1, testApp2)
		apps, err := appServer.List(t.Context(), &application.ApplicationQuery{AppNamespace: ptr.To("argocd-1")})
		require.NoError(t, err)
		require.Empty(t, apps.Items)
	})

	t.Run("List applications with allowed apps in other namespaces", func(t *testing.T) {
		t.Parallel()
		testApp1 := newTestApp()
		testApp1.Namespace = "argocd-1"
		appServer := newTestAppServer(t, testApp1)
		appServer.enabledNamespaces = []string{"argocd-1"}
		apps, err := appServer.List(t.Context(), &application.ApplicationQuery{})
		require.NoError(t, err)
		require.Len(t, apps.Items, 1)
	})

	t.Run("Get application in control plane namespace", func(t *testing.T) {
		t.Parallel()
		testApp := newTestApp()
		appServer := newTestAppServer(t, testApp)
		app, err := appServer.Get(t.Context(), &application.ApplicationQuery{
			Name: ptr.To("test-app"),
		})
		require.NoError(t, err)
		assert.Equal(t, "test-app", app.GetName())
	})
	t.Run("Get application in other namespace when forbidden", func(t *testing.T) {
		t.Parallel()
		testApp := newTestApp()
		testApp.Namespace = "argocd-1"
		appServer := newTestAppServer(t, testApp)
		app, err := appServer.Get(t.Context(), &application.ApplicationQuery{
			Name:         ptr.To("test-app"),
			AppNamespace: ptr.To("argocd-1"),
		})
		require.ErrorContains(t, err, "permission denied")
		require.Nil(t, app)
	})
	t.Run("Get application in other namespace when allowed", func(t *testing.T) {
		t.Parallel()
		testApp := newTestApp()
		testApp.Namespace = "argocd-1"
		testApp.Spec.Project = "other-ns"
		otherNsProj := &v1alpha1.AppProject{
			ObjectMeta: metav1.ObjectMeta{Name: "other-ns", Namespace: "default"},
			Spec: v1alpha1.AppProjectSpec{
				SourceRepos:      []string{"*"},
				Destinations:     []v1alpha1.ApplicationDestination{{Server: "*", Namespace: "*"}},
				SourceNamespaces: []string{"argocd-1"},
			},
		}
		appServer := newTestAppServer(t, testApp, otherNsProj)
		appServer.enabledNamespaces = []string{"argocd-1"}
		app, err := appServer.Get(t.Context(), &application.ApplicationQuery{
			Name:         ptr.To("test-app"),
			AppNamespace: ptr.To("argocd-1"),
		})
		require.NoError(t, err)
		require.NotNil(t, app)
		require.Equal(t, "argocd-1", app.Namespace)
		require.Equal(t, "test-app", app.Name)
	})
	t.Run("Get application in other namespace when project is not allowed", func(t *testing.T) {
		t.Parallel()
		testApp := newTestApp()
		testApp.Namespace = "argocd-1"
		testApp.Spec.Project = "other-ns"
		otherNsProj := &v1alpha1.AppProject{
			ObjectMeta: metav1.ObjectMeta{Name: "other-ns", Namespace: "default"},
			Spec: v1alpha1.AppProjectSpec{
				SourceRepos:      []string{"*"},
				Destinations:     []v1alpha1.ApplicationDestination{{Server: "*", Namespace: "*"}},
				SourceNamespaces: []string{"argocd-2"},
			},
		}
		appServer := newTestAppServer(t, testApp, otherNsProj)
		appServer.enabledNamespaces = []string{"argocd-1"}
		app, err := appServer.Get(t.Context(), &application.ApplicationQuery{
			Name:         ptr.To("test-app"),
			AppNamespace: ptr.To("argocd-1"),
		})
		require.Error(t, err)
		require.Nil(t, app)
		require.ErrorContains(t, err, "app is not allowed in project")
	})
	t.Run("Create application in other namespace when allowed", func(t *testing.T) {
		t.Parallel()
		testApp := newTestApp()
		testApp.Namespace = "argocd-1"
		testApp.Spec.Project = "other-ns"
		otherNsProj := &v1alpha1.AppProject{
			ObjectMeta: metav1.ObjectMeta{Name: "other-ns", Namespace: "default"},
			Spec: v1alpha1.AppProjectSpec{
				SourceRepos:      []string{"*"},
				Destinations:     []v1alpha1.ApplicationDestination{{Server: "*", Namespace: "*"}},
				SourceNamespaces: []string{"argocd-1"},
			},
		}
		appServer := newTestAppServer(t, otherNsProj)
		appServer.enabledNamespaces = []string{"argocd-1"}
		app, err := appServer.Create(t.Context(), &application.ApplicationCreateRequest{
			Application: testApp,
		})
		require.NoError(t, err)
		require.NotNil(t, app)
		assert.Equal(t, "test-app", app.Name)
		assert.Equal(t, "argocd-1", app.Namespace)
	})

	t.Run("Create application in other namespace when not allowed by project", func(t *testing.T) {
		t.Parallel()
		testApp := newTestApp()
		testApp.Namespace = "argocd-1"
		testApp.Spec.Project = "other-ns"
		otherNsProj := &v1alpha1.AppProject{
			ObjectMeta: metav1.ObjectMeta{Name: "other-ns", Namespace: "default"},
			Spec: v1alpha1.AppProjectSpec{
				SourceRepos:      []string{"*"},
				Destinations:     []v1alpha1.ApplicationDestination{{Server: "*", Namespace: "*"}},
				SourceNamespaces: []string{},
			},
		}
		appServer := newTestAppServer(t, otherNsProj)
		appServer.enabledNamespaces = []string{"argocd-1"}
		app, err := appServer.Create(t.Context(), &application.ApplicationCreateRequest{
			Application: testApp,
		})
		require.Error(t, err)
		require.Nil(t, app)
		require.ErrorContains(t, err, "app is not allowed in project")
	})

	t.Run("Create application in other namespace when not allowed by configuration", func(t *testing.T) {
		t.Parallel()
		testApp := newTestApp()
		testApp.Namespace = "argocd-1"
		testApp.Spec.Project = "other-ns"
		otherNsProj := &v1alpha1.AppProject{
			ObjectMeta: metav1.ObjectMeta{Name: "other-ns", Namespace: "default"},
			Spec: v1alpha1.AppProjectSpec{
				SourceRepos:      []string{"*"},
				Destinations:     []v1alpha1.ApplicationDestination{{Server: "*", Namespace: "*"}},
				SourceNamespaces: []string{"argocd-1"},
			},
		}
		appServer := newTestAppServer(t, otherNsProj)
		appServer.enabledNamespaces = []string{"argocd-2"}
		app, err := appServer.Create(t.Context(), &application.ApplicationCreateRequest{
			Application: testApp,
		})
		require.Error(t, err)
		require.Nil(t, app)
		require.ErrorContains(t, err, "namespace 'argocd-1' is not permitted")
	})
	t.Run("Get application sync window in other namespace when project is allowed", func(t *testing.T) {
		t.Parallel()
		testApp := newTestApp()
		testApp.Namespace = "argocd-1"
		testApp.Spec.Project = "other-ns"
		otherNsProj := &v1alpha1.AppProject{
			ObjectMeta: metav1.ObjectMeta{Name: "other-ns", Namespace: "default"},
			Spec: v1alpha1.AppProjectSpec{
				SourceRepos:      []string{"*"},
				Destinations:     []v1alpha1.ApplicationDestination{{Server: "*", Namespace: "*"}},
				SourceNamespaces: []string{"argocd-1"},
			},
		}
		appServer := newTestAppServer(t, testApp, otherNsProj)
		appServer.enabledNamespaces = []string{"argocd-1"}
		active, err := appServer.GetApplicationSyncWindows(t.Context(), &application.ApplicationSyncWindowsQuery{Name: &testApp.Name, AppNamespace: &testApp.Namespace})
		require.NoError(t, err)
		assert.Empty(t, active.ActiveWindows)
	})
	t.Run("Get application sync window in other namespace when project is not allowed", func(t *testing.T) {
		t.Parallel()
		testApp := newTestApp()
		testApp.Namespace = "argocd-1"
		testApp.Spec.Project = "other-ns"
		otherNsProj := &v1alpha1.AppProject{
			ObjectMeta: metav1.ObjectMeta{Name: "other-ns", Namespace: "default"},
			Spec: v1alpha1.AppProjectSpec{
				SourceRepos:      []string{"*"},
				Destinations:     []v1alpha1.ApplicationDestination{{Server: "*", Namespace: "*"}},
				SourceNamespaces: []string{"argocd-2"},
			},
		}
		appServer := newTestAppServer(t, testApp, otherNsProj)
		appServer.enabledNamespaces = []string{"argocd-1"}
		active, err := appServer.GetApplicationSyncWindows(t.Context(), &application.ApplicationSyncWindowsQuery{Name: &testApp.Name, AppNamespace: &testApp.Namespace})
		require.Error(t, err)
		require.Nil(t, active)
		require.ErrorContains(t, err, "app is not allowed in project")
	})
	t.Run("Get list of links in other namespace when project is not allowed", func(t *testing.T) {
		t.Parallel()
		testApp := newTestApp()
		testApp.Namespace = "argocd-1"
		testApp.Spec.Project = "other-ns"
		otherNsProj := &v1alpha1.AppProject{
			ObjectMeta: metav1.ObjectMeta{Name: "other-ns", Namespace: "default"},
			Spec: v1alpha1.AppProjectSpec{
				SourceRepos:      []string{"*"},
				Destinations:     []v1alpha1.ApplicationDestination{{Server: "*", Namespace: "*"}},
				SourceNamespaces: []string{"argocd-2"},
			},
		}
		appServer := newTestAppServer(t, testApp, otherNsProj)
		appServer.enabledNamespaces = []string{"argocd-1"}
		links, err := appServer.ListLinks(t.Context(), &application.ListAppLinksRequest{
			Name:      ptr.To("test-app"),
			Namespace: ptr.To("argocd-1"),
		})
		require.Error(t, err)
		require.Nil(t, links)
		require.ErrorContains(t, err, "app is not allowed in project")
	})
	t.Run("Get list of links in other namespace when project is allowed", func(t *testing.T) {
		t.Parallel()
		testApp := newTestApp()
		testApp.Namespace = "argocd-1"
		testApp.Spec.Project = "other-ns"
		otherNsProj := &v1alpha1.AppProject{
			ObjectMeta: metav1.ObjectMeta{Name: "other-ns", Namespace: "default"},
			Spec: v1alpha1.AppProjectSpec{
				SourceRepos:      []string{"*"},
				Destinations:     []v1alpha1.ApplicationDestination{{Server: "*", Namespace: "*"}},
				SourceNamespaces: []string{"argocd-1"},
			},
		}
		appServer := newTestAppServer(t, testApp, otherNsProj)
		appServer.enabledNamespaces = []string{"argocd-1"}
		links, err := appServer.ListLinks(t.Context(), &application.ListAppLinksRequest{
			Name:      ptr.To("test-app"),
			Namespace: ptr.To("argocd-1"),
		})
		require.NoError(t, err)
		assert.Empty(t, links.Items)
	})
}

func TestGetAmbiguousRevision_MultiSource(t *testing.T) {
	app := &v1alpha1.Application{
		Spec: v1alpha1.ApplicationSpec{
			Sources: []v1alpha1.ApplicationSource{
				{
					TargetRevision: "revision1",
				},
				{
					TargetRevision: "revision2",
				},
			},
		},
	}
	syncReq := &application.ApplicationSyncRequest{
		SourcePositions: []int64{1, 2},
		Revisions:       []string{"rev1", "rev2"},
	}

	sourceIndex := 0
	expected := "rev1"
	result := getAmbiguousRevision(app, syncReq, sourceIndex)
	assert.Equalf(t, expected, result, "Expected ambiguous revision to be %s, but got %s", expected, result)

	sourceIndex = 1
	expected = "rev2"
	result = getAmbiguousRevision(app, syncReq, sourceIndex)
	assert.Equal(t, expected, result, "Expected ambiguous revision to be %s, but got %s", expected, result)

	// Test when app.Spec.HasMultipleSources() is false
	app.Spec = v1alpha1.ApplicationSpec{
		Source: &v1alpha1.ApplicationSource{
			TargetRevision: "revision3",
		},
		Sources: nil,
	}
	syncReq = &application.ApplicationSyncRequest{
		Revision: strToPtr("revision3"),
	}
	expected = "revision3"
	result = getAmbiguousRevision(app, syncReq, sourceIndex)
	assert.Equal(t, expected, result, "Expected ambiguous revision to be %s, but got %s", expected, result)
}

func TestGetAmbiguousRevision_SingleSource(t *testing.T) {
	app := &v1alpha1.Application{
		Spec: v1alpha1.ApplicationSpec{
			Source: &v1alpha1.ApplicationSource{
				TargetRevision: "revision1",
			},
		},
	}
	syncReq := &application.ApplicationSyncRequest{
		Revision: strToPtr("rev1"),
	}

	// Test when app.Spec.HasMultipleSources() is true
	sourceIndex := 1
	expected := "rev1"
	result := getAmbiguousRevision(app, syncReq, sourceIndex)
	assert.Equalf(t, expected, result, "Expected ambiguous revision to be %s, but got %s", expected, result)
}

func TestServer_ResolveSourceRevisions_MultiSource(t *testing.T) {
	s := newTestAppServer(t)

	ctx := t.Context()
	a := &v1alpha1.Application{
		Spec: v1alpha1.ApplicationSpec{
			Sources: []v1alpha1.ApplicationSource{
				{
					RepoURL: "https://github.com/example/repo.git",
				},
			},
		},
	}

	syncReq := &application.ApplicationSyncRequest{
		SourcePositions: []int64{1},
		Revisions:       []string{"HEAD"},
	}

	revision, displayRevision, sourceRevisions, displayRevisions, err := s.resolveSourceRevisions(ctx, a, syncReq)

	require.NoError(t, err)
	assert.Empty(t, revision)
	assert.Empty(t, displayRevision)
	assert.Equal(t, []string{fakeResolveRevisionResponse().Revision}, sourceRevisions)
	assert.Equal(t, []string{fakeResolveRevisionResponse().AmbiguousRevision}, displayRevisions)
}

func TestServer_ResolveSourceRevisions_SingleSource(t *testing.T) {
	s := newTestAppServer(t)

	ctx := t.Context()
	a := &v1alpha1.Application{
		Spec: v1alpha1.ApplicationSpec{
			Source: &v1alpha1.ApplicationSource{
				RepoURL: "https://github.com/example/repo.git",
			},
		},
	}

	syncReq := &application.ApplicationSyncRequest{
		Revision: strToPtr("HEAD"),
	}

	revision, displayRevision, sourceRevisions, displayRevisions, err := s.resolveSourceRevisions(ctx, a, syncReq)

	require.NoError(t, err)
	assert.Equal(t, fakeResolveRevisionResponse().Revision, revision)
	assert.Equal(t, fakeResolveRevisionResponse().AmbiguousRevision, displayRevision)
	assert.Equal(t, []string(nil), sourceRevisions)
	assert.Equal(t, []string(nil), displayRevisions)
}

func Test_RevisionMetadata(t *testing.T) {
	t.Parallel()

	singleSourceApp := newTestApp()
	singleSourceApp.Name = "single-source-app"
	singleSourceApp.Spec = v1alpha1.ApplicationSpec{
		Source: &v1alpha1.ApplicationSource{
			RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
			Path:           "helm-guestbook",
			TargetRevision: "HEAD",
		},
	}

	multiSourceApp := newTestApp()
	multiSourceApp.Name = "multi-source-app"
	multiSourceApp.Spec = v1alpha1.ApplicationSpec{
		Sources: []v1alpha1.ApplicationSource{
			{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				Path:           "helm-guestbook",
				TargetRevision: "HEAD",
			},
			{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
				Path:           "kustomize-guestbook",
				TargetRevision: "HEAD",
			},
		},
	}

	singleSourceHistory := []v1alpha1.RevisionHistory{
		{
			ID:       1,
			Source:   singleSourceApp.Spec.GetSource(),
			Revision: "a",
		},
	}
	multiSourceHistory := []v1alpha1.RevisionHistory{
		{
			ID:        1,
			Sources:   multiSourceApp.Spec.GetSources(),
			Revisions: []string{"a", "b"},
		},
	}

	testCases := []struct {
		name        string
		multiSource bool
		history     *struct {
			matchesSourceType bool
		}
		sourceIndex         *int32
		versionId           *int32
		expectErrorContains *string
	}{
		{
			name:        "single-source app without history, no source index, no version ID",
			multiSource: false,
		},
		{
			name:                "single-source app without history, no source index, missing version ID",
			multiSource:         false,
			versionId:           ptr.To(int32(999)),
			expectErrorContains: ptr.To("the app has no history"),
		},
		{
			name:        "single source app without history, present source index, no version ID",
			multiSource: false,
			sourceIndex: ptr.To(int32(0)),
		},
		{
			name:                "single source app without history, invalid source index, no version ID",
			multiSource:         false,
			sourceIndex:         ptr.To(int32(999)),
			expectErrorContains: ptr.To("source index 999 not found"),
		},
		{
			name:        "single source app with matching history, no source index, no version ID",
			multiSource: false,
			history:     &struct{ matchesSourceType bool }{true},
		},
		{
			name:                "single source app with matching history, no source index, missing version ID",
			multiSource:         false,
			history:             &struct{ matchesSourceType bool }{true},
			versionId:           ptr.To(int32(999)),
			expectErrorContains: ptr.To("history not found for version ID 999"),
		},
		{
			name:        "single source app with matching history, no source index, present version ID",
			multiSource: false,
			history:     &struct{ matchesSourceType bool }{true},
			versionId:   ptr.To(int32(1)),
		},
		{
			name:        "single source app with multi-source history, no source index, no version ID",
			multiSource: false,
			history:     &struct{ matchesSourceType bool }{false},
		},
		{
			name:                "single source app with multi-source history, no source index, missing version ID",
			multiSource:         false,
			history:             &struct{ matchesSourceType bool }{false},
			versionId:           ptr.To(int32(999)),
			expectErrorContains: ptr.To("history not found for version ID 999"),
		},
		{
			name:        "single source app with multi-source history, no source index, present version ID",
			multiSource: false,
			history:     &struct{ matchesSourceType bool }{false},
			versionId:   ptr.To(int32(1)),
		},
		{
			name:        "single-source app with multi-source history, source index 1, no version ID",
			multiSource: false,
			sourceIndex: ptr.To(int32(1)),
			history:     &struct{ matchesSourceType bool }{false},
			// Since the user requested source index 1, but no version ID, we'll get an error when looking at the live
			// source, because the live source is single-source.
			expectErrorContains: ptr.To("there is only 1 source"),
		},
		{
			name:                "single-source app with multi-source history, invalid source index, no version ID",
			multiSource:         false,
			sourceIndex:         ptr.To(int32(999)),
			history:             &struct{ matchesSourceType bool }{false},
			expectErrorContains: ptr.To("source index 999 not found"),
		},
		{
			name:        "single-source app with multi-source history, valid source index, present version ID",
			multiSource: false,
			sourceIndex: ptr.To(int32(1)),
			history:     &struct{ matchesSourceType bool }{false},
			versionId:   ptr.To(int32(1)),
		},
		{
			name:        "multi-source app without history, no source index, no version ID",
			multiSource: true,
		},
		{
			name:                "multi-source app without history, no source index, missing version ID",
			multiSource:         true,
			versionId:           ptr.To(int32(999)),
			expectErrorContains: ptr.To("the app has no history"),
		},
		{
			name:        "multi-source app without history, present source index, no version ID",
			multiSource: true,
			sourceIndex: ptr.To(int32(1)),
		},
		{
			name:                "multi-source app without history, invalid source index, no version ID",
			multiSource:         true,
			sourceIndex:         ptr.To(int32(999)),
			expectErrorContains: ptr.To("source index 999 not found"),
		},
		{
			name:        "multi-source app with matching history, no source index, no version ID",
			multiSource: true,
			history:     &struct{ matchesSourceType bool }{true},
		},
		{
			name:                "multi-source app with matching history, no source index, missing version ID",
			multiSource:         true,
			history:             &struct{ matchesSourceType bool }{true},
			versionId:           ptr.To(int32(999)),
			expectErrorContains: ptr.To("history not found for version ID 999"),
		},
		{
			name:        "multi-source app with matching history, no source index, present version ID",
			multiSource: true,
			history:     &struct{ matchesSourceType bool }{true},
			versionId:   ptr.To(int32(1)),
		},
		{
			name:        "multi-source app with single-source history, no source index, no version ID",
			multiSource: true,
			history:     &struct{ matchesSourceType bool }{false},
		},
		{
			name:                "multi-source app with single-source history, no source index, missing version ID",
			multiSource:         true,
			history:             &struct{ matchesSourceType bool }{false},
			versionId:           ptr.To(int32(999)),
			expectErrorContains: ptr.To("history not found for version ID 999"),
		},
		{
			name:        "multi-source app with single-source history, no source index, present version ID",
			multiSource: true,
			history:     &struct{ matchesSourceType bool }{false},
			versionId:   ptr.To(int32(1)),
		},
		{
			name:        "multi-source app with single-source history, source index 1, no version ID",
			multiSource: true,
			sourceIndex: ptr.To(int32(1)),
			history:     &struct{ matchesSourceType bool }{false},
		},
		{
			name:                "multi-source app with single-source history, invalid source index, no version ID",
			multiSource:         true,
			sourceIndex:         ptr.To(int32(999)),
			history:             &struct{ matchesSourceType bool }{false},
			expectErrorContains: ptr.To("source index 999 not found"),
		},
		{
			name:        "multi-source app with single-source history, valid source index, present version ID",
			multiSource: true,
			sourceIndex: ptr.To(int32(0)),
			history:     &struct{ matchesSourceType bool }{false},
			versionId:   ptr.To(int32(1)),
		},
		{
			name:                "multi-source app with single-source history, source index 1, present version ID",
			multiSource:         true,
			sourceIndex:         ptr.To(int32(1)),
			history:             &struct{ matchesSourceType bool }{false},
			versionId:           ptr.To(int32(1)),
			expectErrorContains: ptr.To("source index 1 not found"),
		},
	}

	for _, tc := range testCases {
		tcc := tc
		t.Run(tcc.name, func(t *testing.T) {
			t.Parallel()

			app := singleSourceApp.DeepCopy()
			if tcc.multiSource {
				app = multiSourceApp.DeepCopy()
			}
			if tcc.history != nil {
				if tcc.history.matchesSourceType {
					if tcc.multiSource {
						app.Status.History = multiSourceHistory
					} else {
						app.Status.History = singleSourceHistory
					}
				} else {
					if tcc.multiSource {
						app.Status.History = singleSourceHistory
					} else {
						app.Status.History = multiSourceHistory
					}
				}
			}

			s := newTestAppServer(t, app)

			request := &application.RevisionMetadataQuery{
				Name:        ptr.To(app.Name),
				Revision:    ptr.To("HEAD"),
				SourceIndex: tcc.sourceIndex,
				VersionId:   tcc.versionId,
			}

			_, err := s.RevisionMetadata(t.Context(), request)
			if tcc.expectErrorContains != nil {
				require.ErrorContains(t, err, *tcc.expectErrorContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_DeepCopyInformers(t *testing.T) {
	t.Parallel()

	namespace := "test-namespace"
	var ro []runtime.Object
	appOne := newTestApp(func(app *v1alpha1.Application) {
		app.Name = "appOne"
		app.Namespace = namespace
		app.Spec = v1alpha1.ApplicationSpec{}
	})
	appTwo := newTestApp(func(app *v1alpha1.Application) {
		app.Name = "appTwo"
		app.Namespace = namespace
		app.Spec = v1alpha1.ApplicationSpec{}
	})
	appThree := newTestApp(func(app *v1alpha1.Application) {
		app.Name = "appThree"
		app.Namespace = namespace
		app.Spec = v1alpha1.ApplicationSpec{}
	})
	ro = append(ro, appOne, appTwo, appThree)
	appls := []v1alpha1.Application{*appOne, *appTwo, *appThree}

	appSetOne := &v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{Name: "appSetOne", Namespace: namespace},
		Spec:       v1alpha1.ApplicationSetSpec{},
	}
	appSetTwo := &v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{Name: "appSetTwo", Namespace: namespace},
		Spec:       v1alpha1.ApplicationSetSpec{},
	}
	appSetThree := &v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{Name: "appSetThree", Namespace: namespace},
		Spec:       v1alpha1.ApplicationSetSpec{},
	}
	ro = append(ro, appSetOne, appSetTwo, appSetThree)
	appSets := []v1alpha1.ApplicationSet{*appSetOne, *appSetTwo, *appSetThree}

	appProjects := createAppProject("projOne", "projTwo", "projThree")
	for i := range appProjects {
		ro = append(ro, &appProjects[i])
	}

	s := newTestAppServer(t, ro...)

	appList, err := s.appclientset.ArgoprojV1alpha1().Applications(namespace).List(t.Context(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.ElementsMatch(t, appls, appList.Items)
	sAppList := appList.Items
	slices.SortFunc(sAppList, func(a, b v1alpha1.Application) int {
		return strings.Compare(a.Name, b.Name)
	})
	slices.SortFunc(appls, func(a, b v1alpha1.Application) int {
		return strings.Compare(a.Name, b.Name)
	})
	// ensure there is a deep copy
	for i := range appls {
		assert.NotSame(t, &appls[i], &sAppList[i])
		assert.NotSame(t, &appls[i].Spec, &sAppList[i].Spec)
		a, err := s.appclientset.ArgoprojV1alpha1().Applications(namespace).Get(t.Context(), sAppList[i].Name, metav1.GetOptions{})
		require.NoError(t, err)
		assert.NotSame(t, a, &sAppList[i])
	}

	appSetList, err := s.appclientset.ArgoprojV1alpha1().ApplicationSets(namespace).List(t.Context(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.ElementsMatch(t, appSets, appSetList.Items)
	sAppSetList := appSetList.Items
	slices.SortFunc(sAppSetList, func(a, b v1alpha1.ApplicationSet) int {
		return strings.Compare(a.Name, b.Name)
	})
	slices.SortFunc(appSets, func(a, b v1alpha1.ApplicationSet) int {
		return strings.Compare(a.Name, b.Name)
	})
	for i := range appSets {
		assert.NotSame(t, &appSets[i], &sAppSetList[i])
		assert.NotSame(t, &appSets[i].Spec, &sAppSetList[i].Spec)
		a, err := s.appclientset.ArgoprojV1alpha1().ApplicationSets(namespace).Get(t.Context(),
			sAppSetList[i].Name, metav1.GetOptions{})
		require.NoError(t, err)
		assert.NotSame(t, a, &sAppSetList[i])
	}

	projList, err := s.appclientset.ArgoprojV1alpha1().AppProjects("deep-copy-ns").List(t.Context(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.ElementsMatch(t, appProjects, projList.Items)
	spList := projList.Items
	slices.SortFunc(spList, func(a, b v1alpha1.AppProject) int {
		return strings.Compare(a.Name, b.Name)
	})
	slices.SortFunc(appProjects, func(a, b v1alpha1.AppProject) int {
		return strings.Compare(a.Name, b.Name)
	})
	for i := range appProjects {
		assert.NotSame(t, &appProjects[i], &spList[i])
		assert.NotSame(t, &appProjects[i].Spec, &spList[i].Spec)
		p, err := s.appclientset.ArgoprojV1alpha1().AppProjects("deep-copy-ns").Get(t.Context(),
			spList[i].Name, metav1.GetOptions{})
		require.NoError(t, err)
		assert.NotSame(t, p, &spList[i])
	}
}

func TestServerSideDiff(t *testing.T) {
	// Create test projects (avoid "default" which is already created by newTestAppServerWithEnforcerConfigure)
	testProj := &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: "test-project", Namespace: testNamespace},
		Spec: v1alpha1.AppProjectSpec{
			SourceRepos:  []string{"*"},
			Destinations: []v1alpha1.ApplicationDestination{{Server: "*", Namespace: "*"}},
		},
	}

	forbiddenProj := &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: "forbidden-project", Namespace: testNamespace},
		Spec: v1alpha1.AppProjectSpec{
			SourceRepos:  []string{"*"},
			Destinations: []v1alpha1.ApplicationDestination{{Server: "*", Namespace: "*"}},
		},
	}

	// Create test applications that will exist in the server
	testApp := newTestApp(func(app *v1alpha1.Application) {
		app.Name = "test-app"
		app.Namespace = testNamespace
		app.Spec.Project = "test-project"
	})

	forbiddenApp := newTestApp(func(app *v1alpha1.Application) {
		app.Name = "forbidden-app"
		app.Namespace = testNamespace
		app.Spec.Project = "forbidden-project"
	})

	appServer := newTestAppServer(t, testProj, forbiddenProj, testApp, forbiddenApp)

	t.Run("InputValidation", func(t *testing.T) {
		// Test missing application name
		query := &application.ApplicationServerSideDiffQuery{
			AppName:         ptr.To(""), // Empty name instead of nil
			AppNamespace:    ptr.To(testNamespace),
			Project:         ptr.To("test-project"),
			LiveResources:   []*v1alpha1.ResourceDiff{},
			TargetManifests: []string{},
		}

		_, err := appServer.ServerSideDiff(t.Context(), query)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")

		// Test nil application name
		queryNil := &application.ApplicationServerSideDiffQuery{
			AppName:         nil,
			AppNamespace:    ptr.To(testNamespace),
			Project:         ptr.To("test-project"),
			LiveResources:   []*v1alpha1.ResourceDiff{},
			TargetManifests: []string{},
		}

		_, err = appServer.ServerSideDiff(t.Context(), queryNil)
		assert.Error(t, err)
		// Should get an error when name is nil
	})

	t.Run("InvalidManifest", func(t *testing.T) {
		// Test error handling for malformed JSON in target manifests
		query := &application.ApplicationServerSideDiffQuery{
			AppName:         ptr.To("test-app"),
			AppNamespace:    ptr.To(testNamespace),
			Project:         ptr.To("test-project"),
			LiveResources:   []*v1alpha1.ResourceDiff{},
			TargetManifests: []string{`invalid json`},
		}

		_, err := appServer.ServerSideDiff(t.Context(), query)

		// Should return error for invalid JSON
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error unmarshaling target manifest")
	})

	t.Run("InvalidLiveState", func(t *testing.T) {
		// Test error handling for malformed JSON in live state
		liveResource := &v1alpha1.ResourceDiff{
			Group:       "apps",
			Kind:        "Deployment",
			Namespace:   "default",
			Name:        "test-deployment",
			LiveState:   `invalid json`,
			TargetState: "",
			Modified:    true,
		}

		query := &application.ApplicationServerSideDiffQuery{
			AppName:         ptr.To("test-app"),
			AppNamespace:    ptr.To(testNamespace),
			Project:         ptr.To("test-project"),
			LiveResources:   []*v1alpha1.ResourceDiff{liveResource},
			TargetManifests: []string{`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"test"}}`},
		}

		_, err := appServer.ServerSideDiff(t.Context(), query)

		// Should return error for invalid JSON in live state
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error unmarshaling live state")
	})

	t.Run("EmptyRequest", func(t *testing.T) {
		// Test with empty resources - should succeed without errors but no diffs
		query := &application.ApplicationServerSideDiffQuery{
			AppName:         ptr.To("test-app"),
			AppNamespace:    ptr.To(testNamespace),
			Project:         ptr.To("test-project"),
			LiveResources:   []*v1alpha1.ResourceDiff{},
			TargetManifests: []string{},
		}

		resp, err := appServer.ServerSideDiff(t.Context(), query)

		// Should succeed with empty response
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.False(t, *resp.Modified)
		assert.Empty(t, resp.Items)
	})

	t.Run("MissingAppPermission", func(t *testing.T) {
		// Test RBAC enforcement
		query := &application.ApplicationServerSideDiffQuery{
			AppName:         ptr.To("nonexistent-app"),
			AppNamespace:    ptr.To(testNamespace),
			Project:         ptr.To("nonexistent-project"),
			LiveResources:   []*v1alpha1.ResourceDiff{},
			TargetManifests: []string{},
		}

		_, err := appServer.ServerSideDiff(t.Context(), query)

		// Should fail with permission error since nonexistent-app doesn't exist
		require.Error(t, err)
		assert.Contains(t, err.Error(), "application")
	})
}

// TestTerminateOperationWithConflicts tests that TerminateOperation properly handles
// concurrent update conflicts by retrying with the fresh application object.
//
// This test reproduces a bug where the retry loop discards the fresh app object
// fetched from Get(), causing all retries to fail with stale resource versions.
func TestTerminateOperationWithConflicts(t *testing.T) {
	testApp := newTestApp()
	testApp.ResourceVersion = "1"
	testApp.Operation = &v1alpha1.Operation{
		Sync: &v1alpha1.SyncOperation{},
	}
	testApp.Status.OperationState = &v1alpha1.OperationState{
		Operation: *testApp.Operation,
		Phase:     synccommon.OperationRunning,
	}

	appServer := newTestAppServer(t, testApp)
	ctx := context.Background()

	// Get the fake clientset from the deepCopy wrapper
	fakeAppCs := appServer.appclientset.(*deepCopyAppClientset).GetUnderlyingClientSet().(*apps.Clientset)

	getCallCount := 0
	updateCallCount := 0

	// Remove default reactors and add our custom ones
	fakeAppCs.ReactionChain = nil

	// Mock Get to return original version first, then fresh version
	fakeAppCs.AddReactor("get", "applications", func(_ kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		getCallCount++
		freshApp := testApp.DeepCopy()
		if getCallCount == 1 {
			// First Get (for initialization) returns original version
			freshApp.ResourceVersion = "1"
		} else {
			// Subsequent Gets (during retry) return fresh version
			freshApp.ResourceVersion = "2"
		}
		return true, freshApp, nil
	})

	// Mock Update to return conflict on first call, success on second
	fakeAppCs.AddReactor("update", "applications", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		updateCallCount++
		updateAction := action.(kubetesting.UpdateAction)
		app := updateAction.GetObject().(*v1alpha1.Application)

		// First call (with original resource version): return conflict
		if app.ResourceVersion == "1" {
			return true, nil, apierrors.NewConflict(
				schema.GroupResource{Group: "argoproj.io", Resource: "applications"},
				app.Name,
				stderrors.New("the object has been modified"),
			)
		}

		// Second call (with refreshed resource version from Get): return success
		updatedApp := app.DeepCopy()
		return true, updatedApp, nil
	})

	// Attempt to terminate the operation
	_, err := appServer.TerminateOperation(ctx, &application.OperationTerminateRequest{
		Name: ptr.To(testApp.Name),
	})

	// Should succeed after retrying with the fresh app
	require.NoError(t, err)
	assert.GreaterOrEqual(t, updateCallCount, 2, "Update should be called at least twice (once with conflict, once with success)")
}
