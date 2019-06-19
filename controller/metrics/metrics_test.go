package metrics

import (
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"

	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned/fake"
	appinformer "github.com/argoproj/argo-cd/pkg/client/informers/externalversions"
	applister "github.com/argoproj/argo-cd/pkg/client/listers/application/v1alpha1"
)

const fakeApp = `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
  namespace: argocd
spec:
  destination:
    namespace: dummy-namespace
    server: https://localhost:6443
  project: important-project
  source:
    path: some/path
    repoURL: https://github.com/argoproj/argocd-example-apps.git
status:
  sync:
    status: Synced
  health:
    status: Healthy
`

const expectedResponse = `# HELP argocd_app_created_time Creation time in unix timestamp for an application.
# TYPE argocd_app_created_time gauge
argocd_app_created_time{name="my-app",namespace="argocd",project="important-project"} -6.21355968e+10
# HELP argocd_app_health_status The application current health status.
# TYPE argocd_app_health_status gauge
argocd_app_health_status{health_status="Degraded",name="my-app",namespace="argocd",project="important-project"} 0
argocd_app_health_status{health_status="Healthy",name="my-app",namespace="argocd",project="important-project"} 1
argocd_app_health_status{health_status="Missing",name="my-app",namespace="argocd",project="important-project"} 0
argocd_app_health_status{health_status="Progressing",name="my-app",namespace="argocd",project="important-project"} 0
argocd_app_health_status{health_status="Suspended",name="my-app",namespace="argocd",project="important-project"} 0
argocd_app_health_status{health_status="Unknown",name="my-app",namespace="argocd",project="important-project"} 0
# HELP argocd_app_info Information about application.
# TYPE argocd_app_info gauge
argocd_app_info{dest_namespace="dummy-namespace",dest_server="https://localhost:6443",name="my-app",namespace="argocd",project="important-project",repo="https://github.com/argoproj/argocd-example-apps"} 1
# HELP argocd_app_sync_status The application current sync status.
# TYPE argocd_app_sync_status gauge
argocd_app_sync_status{name="my-app",namespace="argocd",project="important-project",sync_status="OutOfSync"} 0
argocd_app_sync_status{name="my-app",namespace="argocd",project="important-project",sync_status="Synced"} 1
argocd_app_sync_status{name="my-app",namespace="argocd",project="important-project",sync_status="Unknown"} 0
`

const fakeDefaultApp = `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
  namespace: argocd
spec:
  destination:
    namespace: dummy-namespace
    server: https://localhost:6443
  source:
    path: some/path
    repoURL: https://github.com/argoproj/argocd-example-apps.git
status:
  sync:
    status: Synced
  health:
    status: Healthy
`

const expectedDefaultResponse = `# HELP argocd_app_created_time Creation time in unix timestamp for an application.
# TYPE argocd_app_created_time gauge
argocd_app_created_time{name="my-app",namespace="argocd",project="default"} -6.21355968e+10
# HELP argocd_app_health_status The application current health status.
# TYPE argocd_app_health_status gauge
argocd_app_health_status{health_status="Degraded",name="my-app",namespace="argocd",project="default"} 0
argocd_app_health_status{health_status="Healthy",name="my-app",namespace="argocd",project="default"} 1
argocd_app_health_status{health_status="Missing",name="my-app",namespace="argocd",project="default"} 0
argocd_app_health_status{health_status="Progressing",name="my-app",namespace="argocd",project="default"} 0
argocd_app_health_status{health_status="Suspended",name="my-app",namespace="argocd",project="default"} 0
argocd_app_health_status{health_status="Unknown",name="my-app",namespace="argocd",project="default"} 0
# HELP argocd_app_info Information about application.
# TYPE argocd_app_info gauge
argocd_app_info{dest_namespace="dummy-namespace",dest_server="https://localhost:6443",name="my-app",namespace="argocd",project="default",repo="https://github.com/argoproj/argocd-example-apps"} 1
# HELP argocd_app_sync_status The application current sync status.
# TYPE argocd_app_sync_status gauge
argocd_app_sync_status{name="my-app",namespace="argocd",project="default",sync_status="OutOfSync"} 0
argocd_app_sync_status{name="my-app",namespace="argocd",project="default",sync_status="Synced"} 1
argocd_app_sync_status{name="my-app",namespace="argocd",project="default",sync_status="Unknown"} 0
`

var noOpHealthCheck = func() error {
	return nil
}

func newFakeApp(fakeApp string) *argoappv1.Application {
	var app argoappv1.Application
	err := yaml.Unmarshal([]byte(fakeApp), &app)
	if err != nil {
		panic(err)
	}
	return &app
}

func newFakeLister(fakeApp ...string) (context.CancelFunc, applister.ApplicationLister) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var fakeApps []runtime.Object
	for _, name := range fakeApp {
		fakeApps = append(fakeApps, newFakeApp(name))
	}
	appClientset := appclientset.NewSimpleClientset(fakeApps...)
	factory := appinformer.NewFilteredSharedInformerFactory(appClientset, 0, "argocd", func(options *metav1.ListOptions) {})
	appInformer := factory.Argoproj().V1alpha1().Applications().Informer()
	go appInformer.Run(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), appInformer.HasSynced) {
		log.Fatal("Timed out waiting for caches to sync")
	}
	return cancel, factory.Argoproj().V1alpha1().Applications().Lister()
}

func testApp(t *testing.T, fakeApp string, expectedResponse string) {
	cancel, appLister := newFakeLister(fakeApp)
	defer cancel()
	metricsServ := NewMetricsServer("localhost:8082", appLister, noOpHealthCheck)
	req, err := http.NewRequest("GET", "/metrics", nil)
	assert.NoError(t, err)
	rr := httptest.NewRecorder()
	metricsServ.Handler.ServeHTTP(rr, req)
	assert.Equal(t, rr.Code, http.StatusOK)
	body := rr.Body.String()
	log.Println(body)
	assertMetricsPrinted(t, expectedResponse, body)
}

type testCombination struct {
	application      string
	expectedResponse string
}

func TestMetrics(t *testing.T) {
	combinations := []testCombination{
		{
			application:      fakeApp,
			expectedResponse: expectedResponse,
		},
		{
			application:      fakeDefaultApp,
			expectedResponse: expectedDefaultResponse,
		},
	}

	for _, combination := range combinations {
		testApp(t, combination.application, combination.expectedResponse)
	}
}

const appSyncTotal = `# HELP argocd_app_sync_total Number of application syncs.
# TYPE argocd_app_sync_total counter
argocd_app_sync_total{name="my-app",namespace="argocd",phase="Error",project="important-project"} 1
argocd_app_sync_total{name="my-app",namespace="argocd",phase="Failed",project="important-project"} 1
argocd_app_sync_total{name="my-app",namespace="argocd",phase="Succeeded",project="important-project"} 2
`

func TestMetricsSyncCounter(t *testing.T) {
	cancel, appLister := newFakeLister()
	defer cancel()
	metricsServ := NewMetricsServer("localhost:8082", appLister, noOpHealthCheck)

	fakeApp := newFakeApp(fakeApp)
	metricsServ.IncSync(fakeApp, &argoappv1.OperationState{Phase: argoappv1.OperationRunning})
	metricsServ.IncSync(fakeApp, &argoappv1.OperationState{Phase: argoappv1.OperationFailed})
	metricsServ.IncSync(fakeApp, &argoappv1.OperationState{Phase: argoappv1.OperationError})
	metricsServ.IncSync(fakeApp, &argoappv1.OperationState{Phase: argoappv1.OperationSucceeded})
	metricsServ.IncSync(fakeApp, &argoappv1.OperationState{Phase: argoappv1.OperationSucceeded})

	req, err := http.NewRequest("GET", "/metrics", nil)
	assert.NoError(t, err)
	rr := httptest.NewRecorder()
	metricsServ.Handler.ServeHTTP(rr, req)
	assert.Equal(t, rr.Code, http.StatusOK)
	body := rr.Body.String()
	log.Println(body)
	assertMetricsPrinted(t, appSyncTotal, body)
}

// assertMetricsPrinted asserts every line in the expected lines appears in the body
func assertMetricsPrinted(t *testing.T, expectedLines, body string) {
	for _, line := range strings.Split(expectedLines, "\n") {
		assert.Contains(t, body, line)
	}
}

const appReconcileMetrics = `argocd_app_reconcile_bucket{name="my-app",namespace="argocd",project="important-project",le="0.25"} 0
argocd_app_reconcile_bucket{name="my-app",namespace="argocd",project="important-project",le="0.5"} 0
argocd_app_reconcile_bucket{name="my-app",namespace="argocd",project="important-project",le="1"} 0
argocd_app_reconcile_bucket{name="my-app",namespace="argocd",project="important-project",le="2"} 0
argocd_app_reconcile_bucket{name="my-app",namespace="argocd",project="important-project",le="4"} 0
argocd_app_reconcile_bucket{name="my-app",namespace="argocd",project="important-project",le="8"} 1
argocd_app_reconcile_bucket{name="my-app",namespace="argocd",project="important-project",le="16"} 1
argocd_app_reconcile_bucket{name="my-app",namespace="argocd",project="important-project",le="+Inf"} 1
argocd_app_reconcile_sum{name="my-app",namespace="argocd",project="important-project"} 5
argocd_app_reconcile_count{name="my-app",namespace="argocd",project="important-project"} 1
`

func TestReconcileMetrics(t *testing.T) {
	cancel, appLister := newFakeLister()
	defer cancel()
	metricsServ := NewMetricsServer("localhost:8082", appLister, noOpHealthCheck)

	fakeApp := newFakeApp(fakeApp)
	metricsServ.IncReconcile(fakeApp, 5*time.Second)

	req, err := http.NewRequest("GET", "/metrics", nil)
	assert.NoError(t, err)
	rr := httptest.NewRecorder()
	metricsServ.Handler.ServeHTTP(rr, req)
	assert.Equal(t, rr.Code, http.StatusOK)
	body := rr.Body.String()
	log.Println(body)
	assertMetricsPrinted(t, appReconcileMetrics, body)
}
