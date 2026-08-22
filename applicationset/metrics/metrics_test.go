package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v3/applicationset/utils"
	argoappv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	metricsutil "github.com/argoproj/argo-cd/v3/util/metrics"
)

var (
	applicationsetNamespaces = []string{"argocd", "test-namespace1"}

	filter = func(appset *argoappv1.ApplicationSet) bool {
		return utils.IsNamespaceAllowed(applicationsetNamespaces, appset.Namespace)
	}

	collectedLabels = []string{"included/test"}
)

const fakeAppsetList = `
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: test1
  namespace: argocd
  labels:
    included/test: test
    not-included.label/test: test
spec:
  generators:
  - git:
      directories:
      - path: test/*
      repoURL: https://github.com/test/test.git
      revision: HEAD
  template:
    metadata:
      name: '{{.path.basename}}'
    spec:
      destination:
        namespace: '{{.path.basename}}'
        server: https://kubernetes.default.svc
      project: default
      source:
        path: '{{.path.path}}'
        repoURL: https://github.com/test/test.git
        targetRevision: HEAD
status:
  resources:
  - group: argoproj.io
    health:
      status: Missing
    kind: Application
    name: test-app1
    namespace: argocd
    status: OutOfSync
    version: v1alpha1
  - group: argoproj.io
    health:
      status: Missing
    kind: Application
    name: test-app2
    namespace: argocd
    status: OutOfSync
    version: v1alpha1
  conditions:
  - lastTransitionTime: "2024-01-01T00:00:00Z"
    message: Successfully generated parameters for all Applications
    reason: ApplicationSetUpToDate
    status: "False"
    type: ErrorOccurred
  - lastTransitionTime: "2024-01-01T00:00:00Z"
    message: Successfully generated parameters for all Applications
    reason: ParametersGenerated
    status: "True"
    type: ParametersGenerated
  - lastTransitionTime: "2024-01-01T00:00:00Z"
    message: ApplicationSet up to date
    reason: ApplicationSetUpToDate
    status: "True"
    type: ResourcesUpToDate
---
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: test2
  namespace: argocd
  labels:
    not-included.label/test: test
spec:
  generators:
  - git:
      directories:
      - path: test/*
      repoURL: https://github.com/test/test.git
      revision: HEAD
  template:
    metadata:
      name: '{{.path.basename}}'
    spec:
      destination:
        namespace: '{{.path.basename}}'
        server: https://kubernetes.default.svc
      project: default
      source:
        path: '{{.path.path}}'
        repoURL: https://github.com/test/test.git
        targetRevision: HEAD
---
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: should-be-filtered-out
  namespace: not-allowed
spec:
  generators:
  - git:
      directories:
      - path: test/*
      repoURL: https://github.com/test/test.git
      revision: HEAD
  template:
    metadata:
      name: '{{.path.basename}}'
    spec:
      destination:
        namespace: '{{.path.basename}}'
        server: https://kubernetes.default.svc
      project: default
      source:
        path: '{{.path.path}}'
        repoURL: https://github.com/test/test.git
        targetRevision: HEAD
---
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: appset-progressive-sync-enabled
  namespace: argocd
spec:
  generators:
  - list:
      elements:
      - environment: dev
        name: app1
      - environment: dev
        name: app2
      - environment: staging
        name: app3
      - environment: staging
        name: app4
      - environment: staging
        name: app5
      - environment: prod
        name: app6
      - environment: prod
        name: app7
      - environment: prod
        name: app8
      - environment: prod
        name: app9
  goTemplate: true
  goTemplateOptions:
  - missingkey=error
  strategy:
    rollingSync:
      steps:
      - matchExpressions:
        - key: environment
          operator: In
          values:
          - dev
      - matchExpressions:
        - key: environmen
          operator: In
          values:
          - staging
      - matchExpressions:
        - key: environment
          operator: In
          values:
          - prod
    type: RollingSync
  template:
    metadata:
      labels:
        environment: '{{ .environment }}'
      name: refresh-{{ .environment }}-{{ .name }}
    spec:
      destination:
        namespace: refresh-{{ .environment }}-{{ .name }}
        server: https://kubernetes.default.svc
      project: default
      source:
        path: guestbook
        repoURL: https://github.com/test/test.git
        targetRevision: HEAD
      syncPolicy:
        syncOptions:
        - CreateNamespace=true
status:
  applicationStatus:
  - application: refresh-dev-app1
    lastTransitionTime: "2026-08-04T02:28:30Z"
    message: Application resource became Healthy, updating status from Progressing
      to Healthy
    status: Healthy
    step: "1"
    targetRevisions:
    - 6a7e2a01addf0f9eed142fae6d6c2a6cf7ca3f1b
  - application: refresh-dev-app2
    lastTransitionTime: "2026-08-04T02:28:29Z"
    message: Application resource became Healthy, updating status from Progressing
      to Healthy
    status: Healthy
    step: "1"
    targetRevisions:
    - 6a7e2a01addf0f9eed142fae6d6c2a6cf7ca3f1b
  - application: refresh-prod-app6
    lastTransitionTime: "2026-08-04T02:28:33Z"
    message: Application resource became Healthy, updating status from Progressing
      to Healthy
    status: Healthy
    step: "3"
    targetRevisions:
    - 6a7e2a01addf0f9eed142fae6d6c2a6cf7ca3f1b
  - application: refresh-prod-app7
    lastTransitionTime: "2026-08-04T02:28:33Z"
    message: Application resource became Healthy, updating status from Progressing
      to Healthy
    status: Healthy
    step: "3"
    targetRevisions:
    - 6a7e2a01addf0f9eed142fae6d6c2a6cf7ca3f1b
  - application: refresh-prod-app8
    lastTransitionTime: "2026-08-04T02:28:33Z"
    message: Application resource became Healthy, updating status from Progressing
      to Healthy
    status: Healthy
    step: "3"
    targetRevisions:
    - 6a7e2a01addf0f9eed142fae6d6c2a6cf7ca3f1b
  - application: refresh-prod-app9
    lastTransitionTime: "2026-08-04T02:28:33Z"
    message: Application resource became Healthy, updating status from Progressing
      to Healthy
    status: Healthy
    step: "3"
    targetRevisions:
    - 6a7e2a01addf0f9eed142fae6d6c2a6cf7ca3f1b
  - application: refresh-staging-app3
    lastTransitionTime: "2026-08-04T02:28:33Z"
    message: Application resource became Healthy, updating status from Progressing
      to Healthy
    status: Healthy
    step: "2"
    targetRevisions:
    - 6a7e2a01addf0f9eed142fae6d6c2a6cf7ca3f1b
  - application: refresh-staging-app4
    lastTransitionTime: "2026-08-04T02:28:33Z"
    message: Application resource became Healthy, updating status from Progressing
      to Healthy
    status: Healthy
    step: "2"
    targetRevisions:
    - 6a7e2a01addf0f9eed142fae6d6c2a6cf7ca3f1b
  - application: refresh-staging-app5
    lastTransitionTime: "2026-08-04T02:28:33Z"
    message: Application resource became Healthy, updating status from Progressing
      to Healthy
    status: Healthy
    step: "2"
    targetRevisions:
    - 6a7e2a01addf0f9eed142fae6d6c2a6cf7ca3f1b
  conditions:
  - lastTransitionTime: "2026-08-04T02:28:22Z"
    message: All applications have been generated successfully
    reason: ApplicationSetUpToDate
    status: "False"
    type: ErrorOccurred
  - lastTransitionTime: "2026-08-04T02:28:22Z"
    message: ''
    reason: ApplicationSetInvalidRolloutConfig
    status: "False"
    type: InvalidRolloutConfig
  - lastTransitionTime: "2026-08-04T02:28:22Z"
    message: Successfully generated parameters for all Applications
    reason: ParametersGenerated
    status: "True"
    type: ParametersGenerated
  - lastTransitionTime: "2026-08-04T02:28:22Z"
    message: All applications have been generated successfully
    reason: ApplicationSetUpToDate
    status: "True"
    type: ResourcesUpToDate
  - lastTransitionTime: "2026-08-04T02:28:33Z"
    message: ApplicationSet Rollout has completed
    reason: ApplicationSetRolloutComplete
    status: "False"
    type: RolloutProgressing
  health:
    message: All applications have been generated successfully
    status: Healthy
  resources:
  - group: argoproj.io
    health:
      status: Healthy
    kind: Application
    name: refresh-dev-app1
    namespace: argocd
    status: Synced
    version: v1alpha1
  - group: argoproj.io
    health:
      status: Healthy
    kind: Application
    name: refresh-dev-app2
    namespace: argocd
    status: Synced
    version: v1alpha1
  - group: argoproj.io
    health:
      status: Healthy
    kind: Application
    name: refresh-prod-app6
    namespace: argocd
    status: Synced
    version: v1alpha1
  - group: argoproj.io
    health:
      status: Healthy
    kind: Application
    name: refresh-prod-app7
    namespace: argocd
    status: Synced
    version: v1alpha1
  - group: argoproj.io
    health:
      status: Healthy
    kind: Application
    name: refresh-prod-app8
    namespace: argocd
    status: Synced
    version: v1alpha1
  - group: argoproj.io
    health:
      status: Healthy
    kind: Application
    name: refresh-prod-app9
    namespace: argocd
    status: Synced
    version: v1alpha1
  - group: argoproj.io
    health:
      status: Healthy
    kind: Application
    name: refresh-staging-app3
    namespace: argocd
    status: Synced
    version: v1alpha1
  - group: argoproj.io
    health:
      status: Healthy
    kind: Application
    name: refresh-staging-app4
    namespace: argocd
    status: Synced
    version: v1alpha1
  - group: argoproj.io
    health:
      status: Healthy
    kind: Application
    name: refresh-staging-app5
    namespace: argocd
    status: Synced
    version: v1alpha1
  resourcesCount: 9
`

func newFakeAppsets(fakeAppsetYAML string) []argoappv1.ApplicationSet {
	var results []argoappv1.ApplicationSet

	appsetRawYamls := strings.SplitSeq(fakeAppsetYAML, "---")

	for appsetRawYaml := range appsetRawYamls {
		var appset argoappv1.ApplicationSet
		err := yaml.Unmarshal([]byte(appsetRawYaml), &appset)
		if err != nil {
			panic(err)
		}

		results = append(results, appset)
	}

	return results
}

func TestApplicationsetCollector(t *testing.T) {
	appsetList := newFakeAppsets(fakeAppsetList)
	client := initializeClient(appsetList)
	metrics.Registry = prometheus.NewRegistry()

	appsetCollector := newAppsetCollector(utils.NewAppsetLister(client), collectedLabels, filter)

	metrics.Registry.MustRegister(appsetCollector)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics", http.NoBody)
	require.NoError(t, err)
	rr := httptest.NewRecorder()
	handler := promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{})
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	// Test correct appset_info and owned applications
	assert.Contains(t, rr.Body.String(), `
argocd_appset_info{name="test1",namespace="argocd",resource_update_status="ApplicationSetUpToDate"} 1
`)
	assert.Contains(t, rr.Body.String(), `
argocd_appset_owned_applications{name="test1",namespace="argocd"} 2
`)
	// Test labels collection - should not include labels not included in the list of collected labels and include the ones that do.
	assert.Contains(t, rr.Body.String(), `
argocd_appset_labels{label_included_test="test",name="test1",namespace="argocd"} 1
`)
	assert.NotContains(t, rr.Body.String(), normalizeLabel("not-included.label/test"))
	// If collected label is not present on the applicationset the value should be empty
	assert.Contains(t, rr.Body.String(), `
argocd_appset_labels{label_included_test="",name="test2",namespace="argocd"} 1
`)
	// If ResourcesUpToDate condition is not present on the applicationset the status should be reported as 'Unknown'
	assert.Contains(t, rr.Body.String(), `
argocd_appset_info{name="test2",namespace="argocd",resource_update_status="Unknown"} 1
`)
	// If there are no resources on the applicationset the owned application gague should return 0
	assert.Contains(t, rr.Body.String(), `
argocd_appset_owned_applications{name="test2",namespace="argocd"} 0
`)
	// Test that filter is working
	assert.NotContains(t, rr.Body.String(), `name="should-be-filtered-out"`)
}

func TestObserveReconcile(t *testing.T) {
	appsetList := newFakeAppsets(fakeAppsetList)
	client := initializeClient(appsetList)
	metrics.Registry = prometheus.NewRegistry()

	appsetMetrics := NewApplicationsetMetrics(utils.NewAppsetLister(client), collectedLabels, filter)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics", http.NoBody)
	require.NoError(t, err)
	rr := httptest.NewRecorder()
	handler := promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{})
	appsetMetrics.ObserveReconcile(&appsetList[0], 5*time.Second)
	handler.ServeHTTP(rr, req)
	assert.Contains(t, rr.Body.String(), `
argocd_appset_reconcile_sum{name="test1",namespace="argocd"} 5
`)
	// If there are no resources on the applicationset the owned application gague should return 0
	assert.Contains(t, rr.Body.String(), `
argocd_appset_reconcile_count{name="test1",namespace="argocd"} 1
`)
}

func TestObserveRolloutDuration(t *testing.T) {
	appsetList := newFakeAppsets(fakeAppsetList)
	client := initializeClient(appsetList)
	metrics.Registry = prometheus.NewRegistry()

	appsetMetrics := NewApplicationsetMetrics(utils.NewAppsetLister(client), collectedLabels, filter)

	appsetMetrics.ObserveRolloutDuration(&appsetList[3], 120*time.Second)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics", http.NoBody)
	require.NoError(t, err)
	rr := httptest.NewRecorder()
	handler := promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{})
	handler.ServeHTTP(rr, req)

	body := rr.Body.String()
	assert.Contains(t, body, `argocd_appset_progressive_sync_rollout_duration_seconds_bucket{name="appset-progressive-sync-enabled",namespace="argocd",le="120"} 1`)
	assert.Contains(t, body, `argocd_appset_progressive_sync_rollout_duration_seconds_count{name="appset-progressive-sync-enabled",namespace="argocd"} 1`)
	assert.Contains(t, body, `argocd_appset_progressive_sync_rollout_duration_seconds_sum{name="appset-progressive-sync-enabled",namespace="argocd"} 120`)
}

func TestObserveStepCompletionDuration(t *testing.T) {
	appsetList := newFakeAppsets(fakeAppsetList)
	client := initializeClient(appsetList)
	metrics.Registry = prometheus.NewRegistry()

	appsetMetrics := NewApplicationsetMetrics(utils.NewAppsetLister(client), collectedLabels, filter)

	appsetMetrics.ObserveStepCompletionDuration(&appsetList[3], "1", 45*time.Second)
	appsetMetrics.ObserveStepCompletionDuration(&appsetList[3], "2", 10*time.Second)
	appsetMetrics.ObserveStepCompletionDuration(&appsetList[3], "3", 30*time.Second)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics", http.NoBody)
	require.NoError(t, err)
	rr := httptest.NewRecorder()
	handler := promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{})
	handler.ServeHTTP(rr, req)

	body := rr.Body.String()
	assert.Contains(t, body, `argocd_appset_progressive_sync_step_duration_seconds_bucket{name="appset-progressive-sync-enabled",namespace="argocd",step="1",le="60"} 1`)
	assert.Contains(t, body, `argocd_appset_progressive_sync_step_duration_seconds_count{name="appset-progressive-sync-enabled",namespace="argocd",step="1"} 1`)
	assert.Contains(t, body, `argocd_appset_progressive_sync_step_duration_seconds_sum{name="appset-progressive-sync-enabled",namespace="argocd",step="1"} 45`)
	assert.Contains(t, body, `argocd_appset_progressive_sync_step_duration_seconds_bucket{name="appset-progressive-sync-enabled",namespace="argocd",step="2",le="10"} 1`)
	assert.Contains(t, body, `argocd_appset_progressive_sync_step_duration_seconds_count{name="appset-progressive-sync-enabled",namespace="argocd",step="2"} 1`)
	assert.Contains(t, body, `argocd_appset_progressive_sync_step_duration_seconds_sum{name="appset-progressive-sync-enabled",namespace="argocd",step="2"} 10`)
	assert.Contains(t, body, `argocd_appset_progressive_sync_step_duration_seconds_bucket{name="appset-progressive-sync-enabled",namespace="argocd",step="3",le="30"} 1`)
	assert.Contains(t, body, `argocd_appset_progressive_sync_step_duration_seconds_count{name="appset-progressive-sync-enabled",namespace="argocd",step="3"} 1`)
	assert.Contains(t, body, `argocd_appset_progressive_sync_step_duration_seconds_sum{name="appset-progressive-sync-enabled",namespace="argocd",step="3"} 30`)
}

func initializeClient(appsets []argoappv1.ApplicationSet) ctrlclient.WithWatch {
	scheme := runtime.NewScheme()
	err := argoappv1.AddToScheme(scheme)
	if err != nil {
		panic(err)
	}

	var clientObjects []ctrlclient.Object

	for _, appset := range appsets {
		clientObjects = append(clientObjects, appset.DeepCopy())
	}

	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(clientObjects...).Build()
}

func normalizeLabel(label string) string {
	return metricsutil.NormalizeLabels("label", []string{label})[0]
}
