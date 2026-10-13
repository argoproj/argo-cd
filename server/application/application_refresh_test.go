package application

import (
	"context"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"
	k8scache "k8s.io/client-go/tools/cache"

	kubetest "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube/kubetest"
	"github.com/argoproj/pkg/v2/sync"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	apps "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/fake"
	appinformer "github.com/argoproj/argo-cd/v3/pkg/client/informers/externalversions"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient/mocks"
	servercache "github.com/argoproj/argo-cd/v3/server/cache"
	utilassets "github.com/argoproj/argo-cd/v3/util/assets"
	cacheutil "github.com/argoproj/argo-cd/v3/util/cache"
	appstate "github.com/argoproj/argo-cd/v3/util/cache/appstate"
	dbutil "github.com/argoproj/argo-cd/v3/util/db"
	utilrbac "github.com/argoproj/argo-cd/v3/util/rbac"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

// droppingBroadcaster is a broadcast.Broadcaster which mimics the production
// Handler dropping events for a subscriber that cannot keep up. It still drives
// the informer (as the real broadcaster does, being registered as an informer
// event handler) but never delivers anything to subscriber channels.
//
// This reproduces https://github.com/argoproj/argo-cd/issues/30114: the refresh
// itself succeeds and the informer reflects the refreshed (annotation-free)
// application, but the completion notification is dropped, so Get blocks until
// the client context expires and reports "application refresh deadline
// exceeded".
type droppingBroadcaster struct {
	appServer *Server
	dropped   atomic.Bool
}

func (b *droppingBroadcaster) notify(newObj any) {
	app, ok := newObj.(*v1alpha1.Application)
	if !ok {
		return
	}
	// The broadcaster is registered on the informer, so it is what makes the
	// informer see the refreshed state.
	_ = b.appServer.appInformer.GetStore().Update(app)
	b.dropped.Store(true)
}

func (b *droppingBroadcaster) Subscribe(_ chan *v1alpha1.ApplicationWatchEvent, _ ...func(event *v1alpha1.ApplicationWatchEvent) bool) func() {
	return func() {}
}

func (b *droppingBroadcaster) OnAdd(obj any, _ bool)  { b.notify(obj) }
func (b *droppingBroadcaster) OnUpdate(_, newObj any) { b.notify(newObj) }
func (b *droppingBroadcaster) OnDelete(_ any)         {}

// newTestAppServerWithDroppingBroadcaster builds an app server whose broadcaster
// drops every notification. The completion event therefore never reaches the
// refresh waiter, which must fall back to the informer instead of blocking until
// the client context expires.
func newTestAppServerWithDroppingBroadcaster(t *testing.T, objects ...runtime.Object) *Server {
	t.Helper()
	testApp, ok := objects[0].(*v1alpha1.Application)
	require.True(t, ok, "first object must be an Application")
	testApp.Spec.Project = "default"

	objects = append(objects, &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: testNamespace},
		Spec: v1alpha1.AppProjectSpec{
			SourceRepos:  []string{"*"},
			Destinations: []v1alpha1.ApplicationDestination{{Server: "*", Namespace: "*"}},
		},
	})

	kubeclientset := fake.NewClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: "argocd-cm", Labels: map[string]string{"app.kubernetes.io/part-of": "argocd"}},
	}, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "argocd-secret", Namespace: testNamespace},
		Data:       map[string][]byte{"admin.password": []byte("test"), "server.secretkey": []byte("test")},
	})
	ctx := t.Context()
	db := dbutil.NewDB(testNamespace, settings.NewSettingsManager(ctx, kubeclientset, testNamespace), kubeclientset)

	fakeAppsClientset := apps.NewSimpleClientset(objects...)
	factory := appinformer.NewSharedInformerFactoryWithOptions(fakeAppsClientset, 0, appinformer.WithNamespace(""), appinformer.WithTweakListOptions(func(_ *metav1.ListOptions) {}))
	appInformer := factory.Argoproj().V1alpha1().Applications().Informer()
	go appInformer.Run(ctx.Done())
	if !k8scache.WaitForCacheSync(ctx.Done(), appInformer.HasSynced) {
		t.Fatal("Timed out waiting for caches to sync")
	}
	projInformer := factory.Argoproj().V1alpha1().AppProjects().Informer()
	go projInformer.Run(ctx.Done())
	if !k8scache.WaitForCacheSync(ctx.Done(), projInformer.HasSynced) {
		t.Fatal("Timed out waiting for caches to sync")
	}

	enforcer := utilrbac.NewEnforcer(kubeclientset, testNamespace, common.ArgoCDRBACConfigMapName, nil)
	require.NoError(t, enforcer.SetBuiltinPolicy(utilassets.BuiltinPolicyCSV))
	enforcer.SetDefaultRole("role:admin")

	appStateCache := appstate.NewCache(cacheutil.NewCache(cacheutil.NewInMemoryCache(time.Hour)), time.Hour)
	appCache := servercache.NewCache(appStateCache, time.Hour, time.Hour)

	server, _ := NewServer(
		testNamespace,
		kubeclientset,
		fakeAppsClientset,
		factory.Argoproj().V1alpha1().Applications().Lister(),
		appInformer,
		nil, // replaced below with the dropping broadcaster
		&mocks.Clientset{RepoServerServiceClient: fakeRepoServerClient(false)},
		appCache,
		&kubetest.MockKubectlCmd{},
		db,
		enforcer,
		sync.NewKeyLock(),
		settings.NewSettingsManager(ctx, kubeclientset, testNamespace),
		projInformer,
		[]string{},
		testEnableEventList,
		true,
	)
	appServer := server.(*Server)
	appServer.appBroadcaster = &droppingBroadcaster{appServer: appServer}

	// Simulate the application controller finishing the refresh as soon as the
	// server requests it: the refresh annotation is removed and the resource
	// version is bumped. The informer learns about it through the broadcaster,
	// which drops the notification to subscribers.
	fakeClientSet := appServer.appclientset.(*deepCopyAppClientset).GetUnderlyingClientSet().(*apps.Clientset)
	fakeClientSet.Lock()
	fakeClientSet.ReactionChain = nil
	fakeClientSet.WatchReactionChain = nil
	fakeClientSet.AddReactor("patch", "applications", func(_ kubetesting.Action) (bool, runtime.Object, error) {
		// The patch which requests the refresh bumps the resource version to 2
		// and sets the refresh annotation; the controller then completes the
		// refresh by removing it at a higher resource version.
		refreshing := testApp.DeepCopy()
		refreshing.ResourceVersion = strconv.Itoa(2)
		refreshing.SetAnnotations(map[string]string{v1alpha1.AnnotationKeyRefresh: string(v1alpha1.RefreshTypeNormal)})
		_ = appServer.appInformer.GetStore().Update(refreshing)

		refreshed := refreshing.DeepCopy()
		refreshed.ResourceVersion = strconv.Itoa(3)
		refreshed.SetAnnotations(map[string]string{})
		appServer.appBroadcaster.OnUpdate(refreshing, refreshed)
		return true, refreshing, nil
	})
	fakeClientSet.Unlock()

	return appServer
}

// TestGetAppRefresh_DroppedCompletionEvent verifies that a refresh does not hang
// when the broadcaster drops the completion event. The refresh has succeeded and
// the informer reflects the annotation-free application, so the waiter must
// unblock from the informer instead of waiting for a notification that was
// dropped (#30114).
func TestGetAppRefresh_DroppedCompletionEvent(t *testing.T) {
	testApp := newTestApp()
	testApp.ResourceVersion = "1"
	appServer := newTestAppServerWithDroppingBroadcaster(t, testApp)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	fetched, err := appServer.Get(ctx, &application.ApplicationQuery{
		Name:    &testApp.Name,
		Refresh: new(string(v1alpha1.RefreshTypeNormal)),
	})
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.True(t, appServer.appBroadcaster.(*droppingBroadcaster).dropped.Load(),
		"test setup must have dropped the completion event")
}
