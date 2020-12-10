package metrics

import (
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/argoproj/gitops-engine/pkg/sync/common"
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

const fakeApp2 = `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app-2
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
operation:
  sync:
    revision: 041eab7439ece92c99b043f0e171788185b8fc1d
    syncStrategy:
      hook: {}
`

const fakeApp3 = `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app-3
  namespace: argocd
  deletionTimestamp: "2020-03-16T09:17:45Z"
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
    status: OutOfSync
  health:
    status: Degraded
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

var noOpHealthCheck = func(r *http.Request) error {
	return nil
}

var appFilter = func(obj interface{}) bool {
	return true
}

func newFakeApp(fakeAppYAML string) *argoappv1.Application {
	var app argoappv1.Application
	err := yaml.Unmarshal([]byte(fakeAppYAML), &app)
	if err != nil {
		panic(err)
	}
	return &app
}

func newFakeLister(fakeAppYAMLs ...string) (context.CancelFunc, applister.ApplicationLister) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var fakeApps []runtime.Object
	for _, appYAML := range fakeAppYAMLs {
		a := newFakeApp(appYAML)
		fakeApps = append(fakeApps, a)
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

func testApp(t *testing.T, fakeAppYAMLs []string, expectedResponse string) {
	cancel, appLister := newFakeLister(fakeAppYAMLs...)
	defer cancel()
	metricsServ, err := NewMetricsServer("localhost:8082", appLister, appFilter, noOpHealthCheck)
	assert.NoError(t, err)
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
	applications     []string
	expectedResponse string
}

func TestMetrics(t *testing.T) {
	combinations := []testCombination{
		{
			applications: []string{fakeApp, fakeApp2, fakeApp3},
			expectedResponse: `
# HELP argocd_app_info Information about application.
# TYPE argocd_app_info gauge
argocd_app_info{dest_namespace="dummy-namespace",dest_server="https://localhost:6443",health_status="Degraded",name="my-app-3",namespace="argocd",operation="delete",project="important-project",repo="https://github.com/argoproj/argocd-example-apps",sync_status="OutOfSync"} 1
argocd_app_info{dest_namespace="dummy-namespace",dest_server="https://localhost:6443",health_status="Healthy",name="my-app",namespace="argocd",operation="",project="important-project",repo="https://github.com/argoproj/argocd-example-apps",sync_status="Synced"} 1
argocd_app_info{dest_namespace="dummy-namespace",dest_server="https://localhost:6443",health_status="Healthy",name="my-app-2",namespace="argocd",operation="sync",project="important-project",repo="https://github.com/argoproj/argocd-example-apps",sync_status="Synced"} 1
`,
		},
		{
			applications: []string{fakeDefaultApp},
			expectedResponse: `
# HELP argocd_app_info Information about application.
# TYPE argocd_app_info gauge
argocd_app_info{dest_namespace="dummy-namespace",dest_server="https://localhost:6443",health_status="Healthy",name="my-app",namespace="argocd",operation="",project="default",repo="https://github.com/argoproj/argocd-example-apps",sync_status="Synced"} 1
`,
		},
	}

	for _, combination := range combinations {
		testApp(t, combination.applications, combination.expectedResponse)
	}
}

func TestLegacyMetrics(t *testing.T) {
	os.Setenv(EnvVarLegacyControllerMetrics, "true")
	defer os.Unsetenv(EnvVarLegacyControllerMetrics)

	expectedResponse := `
# HELP argocd_app_created_time Creation time in unix timestamp for an application.
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
# HELP argocd_app_sync_status The application current sync status.
# TYPE argocd_app_sync_status gauge
argocd_app_sync_status{name="my-app",namespace="argocd",project="important-project",sync_status="OutOfSync"} 0
argocd_app_sync_status{name="my-app",namespace="argocd",project="important-project",sync_status="Synced"} 1
argocd_app_sync_status{name="my-app",namespace="argocd",project="important-project",sync_status="Unknown"} 0
`
	testApp(t, []string{fakeApp}, expectedResponse)
}

func TestMetricsSyncCounter(t *testing.T) {
	cancel, appLister := newFakeLister()
	defer cancel()
	metricsServ, err := NewMetricsServer("localhost:8082", appLister, appFilter, noOpHealthCheck)
	assert.NoError(t, err)

	appSyncTotal := `
# HELP argocd_app_sync_total Number of application syncs.
# TYPE argocd_app_sync_total counter
argocd_app_sync_total{dest_server="https://localhost:6443",name="my-app",namespace="argocd",phase="Error",project="important-project"} 1
argocd_app_sync_total{dest_server="https://localhost:6443",name="my-app",namespace="argocd",phase="Failed",project="important-project"} 1
argocd_app_sync_total{dest_server="https://localhost:6443",name="my-app",namespace="argocd",phase="Succeeded",project="important-project"} 2
`

	fakeApp := newFakeApp(fakeApp)
	metricsServ.IncSync(fakeApp, &argoappv1.OperationState{Phase: common.OperationRunning})
	metricsServ.IncSync(fakeApp, &argoappv1.OperationState{Phase: common.OperationFailed})
	metricsServ.IncSync(fakeApp, &argoappv1.OperationState{Phase: common.OperationError})
	metricsServ.IncSync(fakeApp, &argoappv1.OperationState{Phase: common.OperationSucceeded})
	metricsServ.IncSync(fakeApp, &argoappv1.OperationState{Phase: common.OperationSucceeded})

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
		if line == "" {
			continue
		}
		assert.Contains(t, body, line)
	}
}

func TestReconcileMetrics(t *testing.T) {
	cancel, appLister := newFakeLister()
	defer cancel()
	metricsServ, err := NewMetricsServer("localhost:8082", appLister, appFilter, noOpHealthCheck)
	assert.NoError(t, err)

	appReconcileMetrics := `
# HELP argocd_app_reconcile Application reconciliation performance.
# TYPE argocd_app_reconcile histogram
argocd_app_reconcile_bucket{dest_server="https://localhost:6443",namespace="argocd",le="0.25"} 0
argocd_app_reconcile_bucket{dest_server="https://localhost:6443",namespace="argocd",le="0.5"} 0
argocd_app_reconcile_bucket{dest_server="https://localhost:6443",namespace="argocd",le="1"} 0
argocd_app_reconcile_bucket{dest_server="https://localhost:6443",namespace="argocd",le="2"} 0
argocd_app_reconcile_bucket{dest_server="https://localhost:6443",namespace="argocd",le="4"} 0
argocd_app_reconcile_bucket{dest_server="https://localhost:6443",namespace="argocd",le="8"} 1
argocd_app_reconcile_bucket{dest_server="https://localhost:6443",namespace="argocd",le="16"} 1
argocd_app_reconcile_bucket{dest_server="https://localhost:6443",namespace="argocd",le="+Inf"} 1
argocd_app_reconcile_sum{dest_server="https://localhost:6443",namespace="argocd"} 5
argocd_app_reconcile_count{dest_server="https://localhost:6443",namespace="argocd"} 1
`
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
