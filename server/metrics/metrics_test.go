package metrics

import (
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned/fake"
	appinformer "github.com/argoproj/argo-cd/pkg/client/informers/externalversions"
	applister "github.com/argoproj/argo-cd/pkg/client/listers/application/v1alpha1"
)

var fakeApp = `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
  namespace: argocd
spec:
  destination:
    namespace: dummy-namespace
    server: https://localhost:6443
  project: default
  source:
    path: some/path
    repoURL: https://github.com/argoproj/argocd-example-apps.git
status:
  comparisonResult:
    status: Synced
  health:
    status: Healthy
`

var expectedResponse = `# HELP argocd_app_created_time Creation time in unix timestamp for an application.
# TYPE argocd_app_created_time gauge
argocd_app_created_time{name="my-app",namespace="argocd"} -6.21355968e+10
# HELP argocd_app_health_status The application current health status.
# TYPE argocd_app_health_status gauge
argocd_app_health_status{health_status="Degraded",name="my-app",namespace="argocd"} 0
argocd_app_health_status{health_status="Healthy",name="my-app",namespace="argocd"} 1
argocd_app_health_status{health_status="Missing",name="my-app",namespace="argocd"} 0
argocd_app_health_status{health_status="Progressing",name="my-app",namespace="argocd"} 0
argocd_app_health_status{health_status="Unknown",name="my-app",namespace="argocd"} 0
# HELP argocd_app_info Information about application.
# TYPE argocd_app_info gauge
argocd_app_info{dest_namespace="dummy-namespace",dest_server="https://localhost:6443",name="my-app",namespace="argocd",project="default",repo="https://github.com/argoproj/argocd-example-apps.git"} 1
# HELP argocd_app_sync_status The application current sync status.
# TYPE argocd_app_sync_status gauge
argocd_app_sync_status{name="my-app",namespace="argocd",sync_status="OutOfSync"} 0
argocd_app_sync_status{name="my-app",namespace="argocd",sync_status="Synced"} 1
argocd_app_sync_status{name="my-app",namespace="argocd",sync_status="Unknown"} 0
`

func newFakeApp() *argoappv1.Application {
	var app argoappv1.Application
	err := yaml.Unmarshal([]byte(fakeApp), &app)
	if err != nil {
		panic(err)
	}
	return &app
}

func newFakeLister() (context.CancelFunc, applister.ApplicationLister) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	appClientset := appclientset.NewSimpleClientset(newFakeApp())
	factory := appinformer.NewFilteredSharedInformerFactory(appClientset, 0, "argocd", func(options *metav1.ListOptions) {})
	appInformer := factory.Argoproj().V1alpha1().Applications().Informer()
	go appInformer.Run(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), appInformer.HasSynced) {
		log.Fatal("Timed out waiting for caches to sync")
	}
	return cancel, factory.Argoproj().V1alpha1().Applications().Lister()
}

func TestMetrics(t *testing.T) {
	cancel, appLister := newFakeLister()
	defer cancel()
	metricsServ := NewMetricsServer(8082, appLister)
	req, err := http.NewRequest("GET", "/metrics", nil)
	assert.NoError(t, err)
	rr := httptest.NewRecorder()
	metricsServ.Handler.ServeHTTP(rr, req)
	assert.Equal(t, rr.Code, http.StatusOK)
	body := rr.Body.String()
	log.Println(body)
	assert.Equal(t, expectedResponse, body)
}
