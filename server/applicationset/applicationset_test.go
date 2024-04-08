package applicationset

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	broadcastermocks "github.com/argoproj/argo-cd/v2/util/broadcast/mocks"
	"github.com/argoproj/pkg/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/metadata"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	k8scache "k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/applicationset"
	appsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	apps "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/fake"
	appinformer "github.com/argoproj/argo-cd/v2/pkg/client/informers/externalversions"
	"github.com/argoproj/argo-cd/v2/server/rbacpolicy"
	"github.com/argoproj/argo-cd/v2/util/assets"
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
		Server: "https://cluster-api.example.com",
		Name:   "fake-cluster",
		Config: appsv1.ClusterConfig{},
	}
}

// return an ApplicationServiceServer which returns fake data
func newTestAppSetServer(objects ...runtime.Object) *Server {
	f := func(enf *rbac.Enforcer) {
		_ = enf.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
		enf.SetDefaultRole("role:admin")
	}
	scopedNamespaces := ""
	return newTestAppSetServerWithEnforcerConfigure(f, scopedNamespaces, []runtime.Object{}, objects...)
}

// return an ApplicationServiceServer which returns fake data
func newTestAppSetServerWithKubeObjects(kubeObjects []runtime.Object, objects ...runtime.Object) *Server {
	f := func(enf *rbac.Enforcer) {
		_ = enf.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
		enf.SetDefaultRole("role:admin")
	}
	scopedNamespaces := ""
	return newTestAppSetServerWithEnforcerConfigure(f, scopedNamespaces, kubeObjects, objects...)
}

// return an ApplicationServiceServer which returns fake data
func newTestNamespacedAppSetServer(objects ...runtime.Object) *Server {
	f := func(enf *rbac.Enforcer) {
		_ = enf.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
		enf.SetDefaultRole("role:admin")
	}
	scopedNamespaces := "argocd"
	return newTestAppSetServerWithEnforcerConfigure(f, scopedNamespaces, []runtime.Object{}, objects...)
}

func newTestAppSetServerWithEnforcerConfigure(f func(*rbac.Enforcer), namespace string, kubeObjects []runtime.Object, objects ...runtime.Object) *Server {
	kubeObjects = append(kubeObjects, &v1.ConfigMap{
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
	kubeclientset := fake.NewSimpleClientset(kubeObjects...)
	ctx := context.Background()
	db := db.NewDB(testNamespace, settings.NewSettingsManager(ctx, kubeclientset, testNamespace), kubeclientset)
	_, err := db.CreateRepository(ctx, fakeRepo())
	errors.CheckError(err)
	_, err = db.CreateCluster(ctx, fakeCluster())
	errors.CheckError(err)

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
	factory := appinformer.NewSharedInformerFactoryWithOptions(fakeAppsClientset, 0, appinformer.WithNamespace(namespace), appinformer.WithTweakListOptions(func(options *metav1.ListOptions) {}))
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
	// populate the appset informer with the fake objects
	appsetInformer := factory.Argoproj().V1alpha1().ApplicationSets().Informer()
	go appsetInformer.Run(ctx.Done())
	if !k8scache.WaitForCacheSync(ctx.Done(), appsetInformer.HasSynced) {
		panic("Timed out waiting for caches to sync")
	}

	broadcaster := new(broadcastermocks.Broadcaster[appsv1.ApplicationSetWatchEvent])
	broadcaster.On("Subscribe", mock.Anything, mock.Anything).Return(func() {}).Run(func(args mock.Arguments) {
		// Simulate the broadcaster notifying the subscriber of an applicationset update.
		// The second parameter to Subscribe is filters. For the purposes of tests, we ignore the filters. Future tests
		// might require implementing those.
		go func() {
			events := args.Get(0).(chan *appsv1.ApplicationSetWatchEvent)
			for _, obj := range objects {
				app, ok := obj.(*appsv1.ApplicationSet)
				if ok {
					oldVersion, err := strconv.Atoi(app.ResourceVersion)
					if err != nil {
						oldVersion = 0
					}
					clonedAppSet := app.DeepCopy()
					clonedAppSet.ResourceVersion = fmt.Sprintf("%d", oldVersion+1)
					events <- &appsv1.ApplicationSetWatchEvent{Type: watch.Added, ApplicationSet: *clonedAppSet}
				}
			}
		}()
	})
	broadcaster.On("OnAdd", mock.Anything).Return()
	broadcaster.On("OnUpdate", mock.Anything, mock.Anything).Return()
	broadcaster.On("OnDelete", mock.Anything).Return()

	projInformer := factory.Argoproj().V1alpha1().AppProjects().Informer()
	go projInformer.Run(ctx.Done())
	if !k8scache.WaitForCacheSync(ctx.Done(), projInformer.HasSynced) {
		panic("Timed out waiting for caches to sync")
	}

	server := NewServer(
		db,
		kubeclientset,
		enforcer,
		fakeAppsClientset,
		appInformer,
		broadcaster,
		factory.Argoproj().V1alpha1().ApplicationSets().Lister(),
		fakeProjLister,
		settingsMgr,
		testNamespace,
		sync.NewKeyLock(),
		[]string{testNamespace, "external-namespace"},
	)
	return server.(*Server)
}

func newTestAppSet(opts ...func(appset *appsv1.ApplicationSet)) *appsv1.ApplicationSet {
	appset := appsv1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
		},
		Spec: appsv1.ApplicationSetSpec{
			Template: appsv1.ApplicationSetTemplate{
				Spec: appsv1.ApplicationSpec{
					Project: "default",
				},
			},
		},
	}
	for i := range opts {
		opts[i](&appset)
	}
	return &appset
}

func testListAppsetsWithLabels(t *testing.T, appsetQuery applicationset.ApplicationSetListQuery, appServer *Server) {
	validTests := []struct {
		testName       string
		label          string
		expectedResult []string
	}{
		{testName: "Equality based filtering using '=' operator",
			label:          "key1=value1",
			expectedResult: []string{"AppSet1"}},
		{testName: "Equality based filtering using '==' operator",
			label:          "key1==value1",
			expectedResult: []string{"AppSet1"}},
		{testName: "Equality based filtering using '!=' operator",
			label:          "key1!=value1",
			expectedResult: []string{"AppSet2", "AppSet3"}},
		{testName: "Set based filtering using 'in' operator",
			label:          "key1 in (value1, value3)",
			expectedResult: []string{"AppSet1", "AppSet3"}},
		{testName: "Set based filtering using 'notin' operator",
			label:          "key1 notin (value1, value3)",
			expectedResult: []string{"AppSet2"}},
		{testName: "Set based filtering using 'exists' operator",
			label:          "key1",
			expectedResult: []string{"AppSet1", "AppSet2", "AppSet3"}},
		{testName: "Set based filtering using 'not exists' operator",
			label:          "!key2",
			expectedResult: []string{"AppSet2", "AppSet3"}},
	}
	//test valid scenarios
	for _, validTest := range validTests {
		t.Run(validTest.testName, func(t *testing.T) {
			appsetQuery.Selector = validTest.label
			res, err := appServer.List(context.Background(), &appsetQuery)
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
			appsetQuery.Selector = invalidTest.label
			_, err := appServer.List(context.Background(), &appsetQuery)
			assert.ErrorContains(t, err, invalidTest.errorMesage)
		})
	}
}

func TestListAppSetsInNamespaceWithLabels(t *testing.T) {
	testNamespace := "test-namespace"
	appSetServer := newTestAppSetServer(newTestAppSet(func(appset *appsv1.ApplicationSet) {
		appset.Name = "AppSet1"
		appset.ObjectMeta.Namespace = testNamespace
		appset.SetLabels(map[string]string{"key1": "value1", "key2": "value1"})
	}), newTestAppSet(func(appset *appsv1.ApplicationSet) {
		appset.Name = "AppSet2"
		appset.ObjectMeta.Namespace = testNamespace
		appset.SetLabels(map[string]string{"key1": "value2"})
	}), newTestAppSet(func(appset *appsv1.ApplicationSet) {
		appset.Name = "AppSet3"
		appset.ObjectMeta.Namespace = testNamespace
		appset.SetLabels(map[string]string{"key1": "value3"})
	}))
	appSetServer.enabledNamespaces = []string{testNamespace}
	appsetQuery := applicationset.ApplicationSetListQuery{AppsetNamespace: testNamespace}

	testListAppsetsWithLabels(t, appsetQuery, appSetServer)
}

func TestListAppSetsInDefaultNSWithLabels(t *testing.T) {
	appSetServer := newTestAppSetServer(newTestAppSet(func(appset *appsv1.ApplicationSet) {
		appset.Name = "AppSet1"
		appset.SetLabels(map[string]string{"key1": "value1", "key2": "value1"})
	}), newTestAppSet(func(appset *appsv1.ApplicationSet) {
		appset.Name = "AppSet2"
		appset.SetLabels(map[string]string{"key1": "value2"})
	}), newTestAppSet(func(appset *appsv1.ApplicationSet) {
		appset.Name = "AppSet3"
		appset.SetLabels(map[string]string{"key1": "value3"})
	}))
	appsetQuery := applicationset.ApplicationSetListQuery{}

	testListAppsetsWithLabels(t, appsetQuery, appSetServer)
}

// This test covers https://github.com/argoproj/argo-cd/issues/15429
// If the namespace isn't provided during listing action, argocd's
// default namespace must be used and not all the namespaces
func TestListAppSetsWithoutNamespace(t *testing.T) {
	testNamespace := "test-namespace"
	appSetServer := newTestNamespacedAppSetServer(newTestAppSet(func(appset *appsv1.ApplicationSet) {
		appset.Name = "AppSet1"
		appset.ObjectMeta.Namespace = testNamespace
		appset.SetLabels(map[string]string{"key1": "value1", "key2": "value1"})
	}), newTestAppSet(func(appset *appsv1.ApplicationSet) {
		appset.Name = "AppSet2"
		appset.ObjectMeta.Namespace = testNamespace
		appset.SetLabels(map[string]string{"key1": "value2"})
	}), newTestAppSet(func(appset *appsv1.ApplicationSet) {
		appset.Name = "AppSet3"
		appset.ObjectMeta.Namespace = testNamespace
		appset.SetLabels(map[string]string{"key1": "value3"})
	}))
	appSetServer.enabledNamespaces = []string{testNamespace}
	appsetQuery := applicationset.ApplicationSetListQuery{}

	res, err := appSetServer.List(context.Background(), &appsetQuery)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(res.Items))
}

func TestCreateAppSet(t *testing.T) {
	testAppSet := newTestAppSet()
	appServer := newTestAppSetServer()
	testAppSet.Spec.Generators = []appsv1.ApplicationSetGenerator{
		{
			List: &appsv1.ListGenerator{},
		},
	}
	createReq := applicationset.ApplicationSetCreateRequest{
		Applicationset: testAppSet,
	}
	_, err := appServer.Create(context.Background(), &createReq)
	assert.NoError(t, err)
}

func TestCreateAppSetTemplatedProject(t *testing.T) {
	testAppSet := newTestAppSet()
	appServer := newTestAppSetServer()
	testAppSet.Spec.Template.Spec.Project = "{{ .project }}"
	createReq := applicationset.ApplicationSetCreateRequest{
		Applicationset: testAppSet,
	}
	_, err := appServer.Create(context.Background(), &createReq)
	assert.Equal(t, "error validating ApplicationSets: the Argo CD API does not currently support creating ApplicationSets with templated `project` fields", err.Error())
}

func TestCreateAppSetWrongNamespace(t *testing.T) {
	testAppSet := newTestAppSet()
	appServer := newTestAppSetServer()
	testAppSet.ObjectMeta.Namespace = "NOT-ALLOWED"
	createReq := applicationset.ApplicationSetCreateRequest{
		Applicationset: testAppSet,
	}
	_, err := appServer.Create(context.Background(), &createReq)

	assert.Equal(t, "namespace 'NOT-ALLOWED' is not permitted", err.Error())
}

func TestGetAppSet(t *testing.T) {
	appSet1 := newTestAppSet(func(appset *appsv1.ApplicationSet) {
		appset.Name = "AppSet1"
	})

	appSet2 := newTestAppSet(func(appset *appsv1.ApplicationSet) {
		appset.Name = "AppSet2"
	})

	appSet3 := newTestAppSet(func(appset *appsv1.ApplicationSet) {
		appset.Name = "AppSet3"
	})

	t.Run("Get in default namespace", func(t *testing.T) {

		appSetServer := newTestAppSetServer(appSet1, appSet2, appSet3)

		appsetQuery := applicationset.ApplicationSetGetQuery{Name: "AppSet1"}

		res, err := appSetServer.Get(context.Background(), &appsetQuery)
		assert.NoError(t, err)
		assert.Equal(t, "AppSet1", res.Name)
	})

	t.Run("Get in named namespace", func(t *testing.T) {

		appSetServer := newTestAppSetServer(appSet1, appSet2, appSet3)

		appsetQuery := applicationset.ApplicationSetGetQuery{Name: "AppSet1", AppsetNamespace: testNamespace}

		res, err := appSetServer.Get(context.Background(), &appsetQuery)
		assert.NoError(t, err)
		assert.Equal(t, "AppSet1", res.Name)
	})

	t.Run("Get in not allowed namespace", func(t *testing.T) {

		appSetServer := newTestAppSetServer(appSet1, appSet2, appSet3)

		appsetQuery := applicationset.ApplicationSetGetQuery{Name: "AppSet1", AppsetNamespace: "NOT-ALLOWED"}

		_, err := appSetServer.Get(context.Background(), &appsetQuery)
		assert.Equal(t, "namespace 'NOT-ALLOWED' is not permitted", err.Error())
	})
}

func TestWatchAppSet(t *testing.T) {
	appSet1 := newTestAppSet(func(appset *appsv1.ApplicationSet) {
		appset.Name = "AppSet1"
	})

	appSet2 := newTestAppSet(func(appset *appsv1.ApplicationSet) {
		appset.Name = "AppSet2"
	})

	appSet3 := newTestAppSet(func(appset *appsv1.ApplicationSet) {
		appset.Name = "AppSet3"
	})

	t.Run("Watch in default namespace", func(t *testing.T) {

		appSetServer := newTestAppSetServer(appSet1, appSet2, appSet3)

		appsetQuery := applicationset.ApplicationSetWatchQuery{Name: "AppSet1"}

		ctx, cancel := context.WithCancel(context.Background())
		watchServer := &TestWatchServer{ctx: ctx}
		go func() {
			// Watch will block until canceled so give 500ms to read send events and cancel
			time.Sleep(500 * time.Millisecond)
			cancel()

		}()
		err := appSetServer.Watch(&appsetQuery, watchServer)

		assert.NoError(t, err)
		assert.Len(t, watchServer.sent, 2) // Sent twice because of workaround to avoid missing events
		assert.Equal(t, watchServer.sent[0].Type, watch.Added)
		assert.Equal(t, watchServer.sent[0].ApplicationSet.Name, "AppSet1")
	})

	t.Run("Watch in named namespace", func(t *testing.T) {

		appSetServer := newTestAppSetServer(appSet1, appSet2, appSet3)

		appsetQuery := applicationset.ApplicationSetWatchQuery{Name: "AppSet1", AppsetNamespace: testNamespace}

		ctx, cancel := context.WithCancel(context.Background())
		watchServer := &TestWatchServer{ctx: ctx}
		go func() {
			// Watch will block until canceled so give 500ms to read send events and cancel
			time.Sleep(500 * time.Millisecond)
			cancel()

		}()
		err := appSetServer.Watch(&appsetQuery, watchServer)

		assert.NoError(t, err)
		assert.Len(t, watchServer.sent, 2) // Sent twice because of workaround to avoid missing events
		assert.Equal(t, watchServer.sent[0].Type, watch.Added)
		assert.Equal(t, watchServer.sent[0].ApplicationSet.Name, "AppSet1")
	})

	t.Run("Watch in not allowed namespace", func(t *testing.T) {

		appSetServer := newTestAppSetServer(appSet1, appSet2, appSet3)

		appsetQuery := applicationset.ApplicationSetWatchQuery{Name: "AppSet1", AppsetNamespace: "NOT-ALLOWED"}

		watchServer := &TestWatchServer{ctx: context.Background()}
		err := appSetServer.Watch(&appsetQuery, watchServer)

		assert.Equal(t, "namespace 'NOT-ALLOWED' is not permitted", err.Error())
		assert.Len(t, watchServer.sent, 0)
	})

	t.Run("Watch all appsets in default namespace", func(t *testing.T) {

		appSetServer := newTestAppSetServer(appSet1, appSet2, appSet3)

		appsetQuery := applicationset.ApplicationSetWatchQuery{}

		ctx, cancel := context.WithCancel(context.Background())
		watchServer := &TestWatchServer{ctx: ctx}
		go func() {
			// Watch will block until canceled so give 500ms to read events and cancel
			time.Sleep(500 * time.Millisecond)
			cancel()

		}()
		err := appSetServer.Watch(&appsetQuery, watchServer)

		assert.NoError(t, err)
		assert.Len(t, watchServer.sent, 6) // Sent twice because of workaround to avoid missing events
	})
}

func TestDeleteAppSet(t *testing.T) {
	appSet1 := newTestAppSet(func(appset *appsv1.ApplicationSet) {
		appset.Name = "AppSet1"
	})

	appSet2 := newTestAppSet(func(appset *appsv1.ApplicationSet) {
		appset.Name = "AppSet2"
	})

	appSet3 := newTestAppSet(func(appset *appsv1.ApplicationSet) {
		appset.Name = "AppSet3"
	})

	t.Run("Delete in default namespace", func(t *testing.T) {

		appSetServer := newTestAppSetServer(appSet1, appSet2, appSet3)

		appsetQuery := applicationset.ApplicationSetDeleteRequest{Name: "AppSet1"}

		res, err := appSetServer.Delete(context.Background(), &appsetQuery)
		assert.NoError(t, err)
		assert.Equal(t, &applicationset.ApplicationSetResponse{}, res)
	})

	t.Run("Delete in named namespace", func(t *testing.T) {

		appSetServer := newTestAppSetServer(appSet1, appSet2, appSet3)

		appsetQuery := applicationset.ApplicationSetDeleteRequest{Name: "AppSet1", AppsetNamespace: testNamespace}

		res, err := appSetServer.Delete(context.Background(), &appsetQuery)
		assert.NoError(t, err)
		assert.Equal(t, &applicationset.ApplicationSetResponse{}, res)
	})
}

func TestUpdateAppSet(t *testing.T) {
	appSet := newTestAppSet(func(appset *appsv1.ApplicationSet) {
		appset.ObjectMeta.Annotations = map[string]string{
			"annotation-key1": "annotation-value1",
			"annotation-key2": "annotation-value2",
		}
		appset.ObjectMeta.Labels = map[string]string{
			"label-key1": "label-value1",
			"label-key2": "label-value2",
		}
	})

	newAppSet := newTestAppSet(func(appset *appsv1.ApplicationSet) {
		appset.ObjectMeta.Annotations = map[string]string{
			"annotation-key1": "annotation-value1-updated",
		}
		appset.ObjectMeta.Labels = map[string]string{
			"label-key1": "label-value1-updated",
		}
	})

	t.Run("Update merge", func(t *testing.T) {

		appServer := newTestAppSetServer(appSet)

		updated, err := appServer.updateAppSet(appSet, newAppSet, context.Background(), true)

		assert.NoError(t, err)
		assert.Equal(t, map[string]string{
			"annotation-key1": "annotation-value1-updated",
			"annotation-key2": "annotation-value2",
		}, updated.Annotations)
		assert.Equal(t, map[string]string{
			"label-key1": "label-value1-updated",
			"label-key2": "label-value2",
		}, updated.Labels)
	})

	t.Run("Update no merge", func(t *testing.T) {

		appServer := newTestAppSetServer(appSet)

		updated, err := appServer.updateAppSet(appSet, newAppSet, context.Background(), false)

		assert.NoError(t, err)
		assert.Equal(t, map[string]string{
			"annotation-key1": "annotation-value1-updated",
		}, updated.Annotations)
		assert.Equal(t, map[string]string{
			"label-key1": "label-value1-updated",
		}, updated.Labels)
	})

}

func TestListResourceEventsAppSet(t *testing.T) {
	appSet := newTestAppSet(func(appset *appsv1.ApplicationSet) {
		appset.Name = "AppSet1"
	})

	event1 := &v1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "event1",
			Namespace: testNamespace,
		},
		InvolvedObject: v1.ObjectReference{
			Name:      appSet.Name,
			Namespace: testNamespace,
			UID:       appSet.UID,
		},
	}

	event2 := &v1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "event2",
			Namespace: "other-namespace",
		},
		InvolvedObject: v1.ObjectReference{
			Name:      appSet.Name,
			Namespace: "other-namespace",
			UID:       appSet.UID,
		},
	}

	t.Run("List events in default namespace", func(t *testing.T) {

		appSetServer := newTestAppSetServerWithKubeObjects([]runtime.Object{event1, event2}, appSet)

		q := applicationset.ApplicationSetResourceEventsQuery{Name: "AppSet1"}

		res, err := appSetServer.ListResourceEvents(context.Background(), &q)
		assert.NoError(t, err)
		assert.Len(t, res.Items, 1)
		assert.Equal(t, res.Items[0].Message, event1.Message)
	})

	t.Run("List events in named namespace", func(t *testing.T) {

		appSetServer := newTestAppSetServerWithKubeObjects([]runtime.Object{event1, event2}, appSet)

		q := applicationset.ApplicationSetResourceEventsQuery{Name: "AppSet1", AppsetNamespace: testNamespace}

		res, err := appSetServer.ListResourceEvents(context.Background(), &q)
		assert.NoError(t, err)
		assert.Len(t, res.Items, 1)
		assert.Equal(t, res.Items[0].Message, event1.Message)
	})

	t.Run("List events in not allowed namespace", func(t *testing.T) {

		appSetServer := newTestAppSetServerWithKubeObjects([]runtime.Object{event1, event2}, appSet)

		q := applicationset.ApplicationSetResourceEventsQuery{Name: "AppSet1", AppsetNamespace: "NOT-ALLOWED"}

		_, err := appSetServer.ListResourceEvents(context.Background(), &q)
		assert.Equal(t, "namespace 'NOT-ALLOWED' is not permitted", err.Error())
	})
}

type TestWatchServer struct {
	ctx  context.Context
	sent []*appsv1.ApplicationSetWatchEvent
}

func (t *TestWatchServer) Send(event *appsv1.ApplicationSetWatchEvent) error {
	t.sent = append(t.sent, event)
	return nil
}

func (t *TestWatchServer) SetHeader(metadata.MD) error {
	return nil
}

func (t *TestWatchServer) SendHeader(metadata.MD) error {
	return nil
}

func (t *TestWatchServer) SetTrailer(metadata.MD) {}

func (t *TestWatchServer) Context() context.Context {
	return t.ctx
}

func (t *TestWatchServer) SendMsg(m interface{}) error {
	return nil
}

func (t *TestWatchServer) RecvMsg(m interface{}) error {
	return nil
}
