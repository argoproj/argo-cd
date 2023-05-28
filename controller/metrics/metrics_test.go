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

	gitopsCache "github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/yaml"

	argoappv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/fake"
	appinformer "github.com/argoproj/argo-cd/v2/pkg/client/informers/externalversions"
	applister "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
)

const fakeApp = `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
  namespace: argocd
  labels:
    team-name: my-team
    team-bu: bu-id
    argoproj.io/cluster: test-cluster
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
  labels:
    team-name: my-team
    team-bu: bu-id
    argoproj.io/cluster: test-cluster
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
  labels:
    team-name: my-team
    team-bu: bu-id
    argoproj.io/cluster: test-cluster
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
	factory := appinformer.NewSharedInformerFactoryWithOptions(appClientset, 0, appinformer.WithNamespace("argocd"), appinformer.WithTweakListOptions(func(options *metav1.ListOptions) {}))
	appInformer := factory.Argoproj().V1alpha1().Applications().Informer()
	go appInformer.Run(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), appInformer.HasSynced) {
		log.Fatal("Timed out waiting for caches to sync")
	}
	return cancel, factory.Argoproj().V1alpha1().Applications().Lister()
}

func testApp(t *testing.T, fakeAppYAMLs []string, expectedResponse string) {
	t.Helper()
	testMetricServer(t, fakeAppYAMLs, expectedResponse, []string{})
}

type fakeClusterInfo struct {
	clustersInfo []gitopsCache.ClusterInfo
}

func (f *fakeClusterInfo) GetClustersInfo() []gitopsCache.ClusterInfo {
	return f.clustersInfo
}

type TestMetricServerConfig struct {
	FakeAppYAMLs     []string
	ExpectedResponse string
	AppLabels        []string
	ClustersInfo     []gitopsCache.ClusterInfo
}

func testMetricServer(t *testing.T, fakeAppYAMLs []string, expectedResponse string, appLabels []string) {
	t.Helper()
	cfg := TestMetricServerConfig{
		FakeAppYAMLs:     fakeAppYAMLs,
		ExpectedResponse: expectedResponse,
		AppLabels:        appLabels,
		ClustersInfo:     []gitopsCache.ClusterInfo{},
	}
	runTest(t, cfg)
}

func runTest(t *testing.T, cfg TestMetricServerConfig) {
	t.Helper()
	cancel, appLister := newFakeLister(cfg.FakeAppYAMLs...)
	defer cancel()
	metricsServ, err := NewMetricsServer("localhost:8082", appLister, appFilter, noOpHealthCheck, cfg.AppLabels)
	assert.NoError(t, err)

	if len(cfg.ClustersInfo) > 0 {
		ci := &fakeClusterInfo{clustersInfo: cfg.ClustersInfo}
		collector := &clusterCollector{
			infoSource: ci,
			info:       ci.GetClustersInfo(),
		}
		metricsServ.registry.MustRegister(collector)
	}

	req, err := http.NewRequest(http.MethodGet, "/metrics", nil)
	assert.NoError(t, err)
	rr := httptest.NewRecorder()
	metricsServ.Handler.ServeHTTP(rr, req)
	assert.Equal(t, rr.Code, http.StatusOK)
	body := rr.Body.String()
	assertMetricsPrinted(t, cfg.ExpectedResponse, body)
}

type testCombination struct {
	applications     []string
	responseContains string
}

func TestMetrics(t *testing.T) {
	combinations := []testCombination{
		{
			applications: []string{fakeApp, fakeApp2, fakeApp3},
			responseContains: `
# HELP argocd_app_info Information about application.
# TYPE argocd_app_info gauge
argocd_app_info{dest_namespace="dummy-namespace",dest_server="https://localhost:6443",health_status="Degraded",name="my-app-3",namespace="argocd",operation="delete",project="important-project",repo="https://github.com/argoproj/argocd-example-apps",sync_status="OutOfSync"} 1
argocd_app_info{dest_namespace="dummy-namespace",dest_server="https://localhost:6443",health_status="Healthy",name="my-app",namespace="argocd",operation="",project="important-project",repo="https://github.com/argoproj/argocd-example-apps",sync_status="Synced"} 1
argocd_app_info{dest_namespace="dummy-namespace",dest_server="https://localhost:6443",health_status="Healthy",name="my-app-2",namespace="argocd",operation="sync",project="important-project",repo="https://github.com/argoproj/argocd-example-apps",sync_status="Synced"} 1
`,
		},
		{
			applications: []string{fakeDefaultApp},
			responseContains: `
# HELP argocd_app_info Information about application.
# TYPE argocd_app_info gauge
argocd_app_info{dest_namespace="dummy-namespace",dest_server="https://localhost:6443",health_status="Healthy",name="my-app",namespace="argocd",operation="",project="default",repo="https://github.com/argoproj/argocd-example-apps",sync_status="Synced"} 1
`,
		},
	}

	for _, combination := range combinations {
		testApp(t, combination.applications, combination.responseContains)
	}
}

func TestMetricLabels(t *testing.T) {
	type testCases struct {
		testCombination
		description  string
		metricLabels []string
	}
	cases := []testCases{
		{
			description:  "will return the labels metrics successfully",
			metricLabels: []string{"team-name", "team-bu", "argoproj.io/cluster"},
			testCombination: testCombination{
				applications: []string{fakeApp, fakeApp2, fakeApp3},
				responseContains: `
# TYPE argocd_app_labels gauge
argocd_app_labels{label_argoproj_io_cluster="test-cluster",label_team_bu="bu-id",label_team_name="my-team",name="my-app",namespace="argocd",project="important-project"} 1
argocd_app_labels{label_argoproj_io_cluster="test-cluster",label_team_bu="bu-id",label_team_name="my-team",name="my-app-2",namespace="argocd",project="important-project"} 1
argocd_app_labels{label_argoproj_io_cluster="test-cluster",label_team_bu="bu-id",label_team_name="my-team",name="my-app-3",namespace="argocd",project="important-project"} 1
`,
			},
		},
		{
			description:  "metric will have empty label value if not present in the application",
			metricLabels: []string{"non-existing"},
			testCombination: testCombination{
				applications: []string{fakeApp, fakeApp2, fakeApp3},
				responseContains: `
# TYPE argocd_app_labels gauge
argocd_app_labels{label_non_existing="",name="my-app",namespace="argocd",project="important-project"} 1
argocd_app_labels{label_non_existing="",name="my-app-2",namespace="argocd",project="important-project"} 1
argocd_app_labels{label_non_existing="",name="my-app-3",namespace="argocd",project="important-project"} 1
`,
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.description, func(t *testing.T) {
			testMetricServer(t, c.applications, c.responseContains, c.metricLabels)
		})
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
	metricsServ, err := NewMetricsServer("localhost:8082", appLister, appFilter, noOpHealthCheck, []string{})
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

	req, err := http.NewRequest(http.MethodGet, "/metrics", nil)
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
	t.Helper()
	for _, line := range strings.Split(expectedLines, "\n") {
		if line == "" {
			continue
		}
		assert.Contains(t, body, line, "expected metrics mismatch")
	}
}

// assertMetricNotPrinted
func assertMetricsNotPrinted(t *testing.T, expectedLines, body string) {
	for _, line := range strings.Split(expectedLines, "\n") {
		if line == "" {
			continue
		}
		assert.False(t, strings.Contains(body, expectedLines))
	}
}

func TestReconcileMetrics(t *testing.T) {
	cancel, appLister := newFakeLister()
	defer cancel()
	metricsServ, err := NewMetricsServer("localhost:8082", appLister, appFilter, noOpHealthCheck, []string{})
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

	req, err := http.NewRequest(http.MethodGet, "/metrics", nil)
	assert.NoError(t, err)
	rr := httptest.NewRecorder()
	metricsServ.Handler.ServeHTTP(rr, req)
	assert.Equal(t, rr.Code, http.StatusOK)
	body := rr.Body.String()
	log.Println(body)
	assertMetricsPrinted(t, appReconcileMetrics, body)
}

func TestMetricsReset(t *testing.T) {
	cancel, appLister := newFakeLister()
	defer cancel()
	metricsServ, err := NewMetricsServer("localhost:8082", appLister, appFilter, noOpHealthCheck, []string{})
	assert.NoError(t, err)

	appSyncTotal := `
# HELP argocd_app_sync_total Number of application syncs.
# TYPE argocd_app_sync_total counter
argocd_app_sync_total{dest_server="https://localhost:6443",name="my-app",namespace="argocd",phase="Error",project="important-project"} 1
argocd_app_sync_total{dest_server="https://localhost:6443",name="my-app",namespace="argocd",phase="Failed",project="important-project"} 1
argocd_app_sync_total{dest_server="https://localhost:6443",name="my-app",namespace="argocd",phase="Succeeded",project="important-project"} 2
`

	req, err := http.NewRequest(http.MethodGet, "/metrics", nil)
	assert.NoError(t, err)
	rr := httptest.NewRecorder()
	metricsServ.Handler.ServeHTTP(rr, req)
	assert.Equal(t, rr.Code, http.StatusOK)
	body := rr.Body.String()
	assertMetricsPrinted(t, appSyncTotal, body)

	err = metricsServ.SetExpiration(time.Second)
	assert.NoError(t, err)
	time.Sleep(2 * time.Second)
	req, err = http.NewRequest(http.MethodGet, "/metrics", nil)
	assert.NoError(t, err)
	rr = httptest.NewRecorder()
	metricsServ.Handler.ServeHTTP(rr, req)
	assert.Equal(t, rr.Code, http.StatusOK)
	body = rr.Body.String()
	log.Println(body)
	assertMetricsNotPrinted(t, appSyncTotal, body)
	err = metricsServ.SetExpiration(time.Second)
	assert.Error(t, err)
}
