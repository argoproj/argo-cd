package metrics

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"

	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	applister "github.com/argoproj/argo-cd/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/healthz"
)

type MetricsServer struct {
	*http.Server
	syncCounter             *prometheus.CounterVec
	kubectlExecCounter      *prometheus.CounterVec
	kubectlExecPendingGauge *prometheus.GaugeVec
	k8sRequestCounter       *prometheus.CounterVec
	clusterEventsCounter    *prometheus.CounterVec
	redisRequestCounter     *prometheus.CounterVec
	reconcileHistogram      *prometheus.HistogramVec
	redisRequestHistogram   *prometheus.HistogramVec
	registry                *prometheus.Registry
	hostname                string
}

const (
	// MetricsPath is the endpoint to collect application metrics
	MetricsPath = "/metrics"
	// EnvVarLegacyControllerMetrics is a env var to re-enable deprecated prometheus metrics
	EnvVarLegacyControllerMetrics = "ARGOCD_LEGACY_CONTROLLER_METRICS"
)

// Follow Prometheus naming practices
// https://prometheus.io/docs/practices/naming/
var (
	descAppDefaultLabels = []string{"namespace", "name", "project"}

	descAppInfo = prometheus.NewDesc(
		"argocd_app_info",
		"Information about application.",
		append(descAppDefaultLabels, "repo", "dest_server", "dest_namespace", "sync_status", "health_status", "operation"),
		nil,
	)
	// DEPRECATED
	descAppCreated = prometheus.NewDesc(
		"argocd_app_created_time",
		"Creation time in unix timestamp for an application.",
		descAppDefaultLabels,
		nil,
	)
	// DEPRECATED: superceded by sync_status label in argocd_app_info
	descAppSyncStatusCode = prometheus.NewDesc(
		"argocd_app_sync_status",
		"The application current sync status.",
		append(descAppDefaultLabels, "sync_status"),
		nil,
	)
	// DEPRECATED: superceded by health_status label in argocd_app_info
	descAppHealthStatus = prometheus.NewDesc(
		"argocd_app_health_status",
		"The application current health status.",
		append(descAppDefaultLabels, "health_status"),
		nil,
	)

	syncCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argocd_app_sync_total",
			Help: "Number of application syncs.",
		},
		append(descAppDefaultLabels, "dest_server", "phase"),
	)

	k8sRequestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argocd_app_k8s_request_total",
			Help: "Number of kubernetes requests executed during application reconciliation.",
		},
		append(descAppDefaultLabels, "server", "response_code", "verb", "resource_kind", "resource_namespace"),
	)

	kubectlExecCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "argocd_kubectl_exec_total",
		Help: "Number of kubectl executions",
	}, []string{"hostname", "command"})

	kubectlExecPendingGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "argocd_kubectl_exec_pending",
		Help: "Number of pending kubectl executions",
	}, []string{"hostname", "command"})

	reconcileHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "argocd_app_reconcile",
			Help: "Application reconciliation performance.",
			// Buckets chosen after observing a ~2100ms mean reconcile time
			Buckets: []float64{0.25, .5, 1, 2, 4, 8, 16},
		},
		[]string{"namespace", "dest_server"},
	)

	clusterEventsCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "argocd_cluster_events_total",
		Help: "Number of processes k8s resource events.",
	}, append(descClusterDefaultLabels, "group", "kind"))

	redisRequestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argocd_redis_request_total",
			Help: "Number of kubernetes requests executed during application reconciliation.",
		},
		[]string{"hostname", "initiator", "failed"},
	)

	redisRequestHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "argocd_redis_request_duration",
			Help:    "Redis requests duration.",
			Buckets: []float64{0.01, 0.05, 0.10, 0.25, .5, 1},
		},
		[]string{"hostname", "initiator"},
	)
)

// NewMetricsServer returns a new prometheus server which collects application metrics
func NewMetricsServer(addr string, appLister applister.ApplicationLister, appFilter func(obj interface{}) bool, healthCheck func(r *http.Request) error) (*MetricsServer, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	registry := NewAppRegistry(appLister, appFilter)
	mux.Handle(MetricsPath, promhttp.HandlerFor(prometheus.Gatherers{
		// contains app controller specific metrics
		registry,
		// contains process, golang and controller workqueues metrics
		prometheus.DefaultGatherer,
	}, promhttp.HandlerOpts{}))
	healthz.ServeHealthCheck(mux, healthCheck)

	registry.MustRegister(syncCounter)
	registry.MustRegister(k8sRequestCounter)
	registry.MustRegister(kubectlExecCounter)
	registry.MustRegister(kubectlExecPendingGauge)
	registry.MustRegister(reconcileHistogram)
	registry.MustRegister(clusterEventsCounter)
	registry.MustRegister(redisRequestCounter)
	registry.MustRegister(redisRequestHistogram)

	return &MetricsServer{
		registry: registry,
		Server: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
		syncCounter:             syncCounter,
		k8sRequestCounter:       k8sRequestCounter,
		kubectlExecCounter:      kubectlExecCounter,
		kubectlExecPendingGauge: kubectlExecPendingGauge,
		reconcileHistogram:      reconcileHistogram,
		clusterEventsCounter:    clusterEventsCounter,
		redisRequestCounter:     redisRequestCounter,
		redisRequestHistogram:   redisRequestHistogram,
		hostname:                hostname,
	}, nil
}

func (m *MetricsServer) RegisterClustersInfoSource(ctx context.Context, source HasClustersInfo) {
	collector := &clusterCollector{infoSource: source}
	go collector.Run(ctx)
	m.registry.MustRegister(collector)
}

// IncSync increments the sync counter for an application
func (m *MetricsServer) IncSync(app *argoappv1.Application, state *argoappv1.OperationState) {
	if !state.Phase.Completed() {
		return
	}
	m.syncCounter.WithLabelValues(app.Namespace, app.Name, app.Spec.GetProject(), app.Spec.Destination.Server, string(state.Phase)).Inc()
}

func (m *MetricsServer) IncKubectlExec(command string) {
	m.kubectlExecCounter.WithLabelValues(m.hostname, command).Inc()
}

func (m *MetricsServer) IncKubectlExecPending(command string) {
	m.kubectlExecPendingGauge.WithLabelValues(m.hostname, command).Inc()
}

func (m *MetricsServer) DecKubectlExecPending(command string) {
	m.kubectlExecPendingGauge.WithLabelValues(m.hostname, command).Dec()
}

// IncClusterEventsCount increments the number of cluster events
func (m *MetricsServer) IncClusterEventsCount(server, group, kind string) {
	m.clusterEventsCounter.WithLabelValues(server, group, kind).Inc()
}

// IncKubernetesRequest increments the kubernetes requests counter for an application
func (m *MetricsServer) IncKubernetesRequest(app *argoappv1.Application, server, statusCode, verb, resourceKind, resourceNamespace string) {
	var namespace, name, project string
	if app != nil {
		namespace = app.Namespace
		name = app.Name
		project = app.Spec.GetProject()
	}
	m.k8sRequestCounter.WithLabelValues(
		namespace, name, project, server, statusCode,
		verb, resourceKind, resourceNamespace,
	).Inc()
}

func (m *MetricsServer) IncRedisRequest(failed bool) {
	m.redisRequestCounter.WithLabelValues(m.hostname, "argocd-application-controller", strconv.FormatBool(failed)).Inc()
}

// ObserveRedisRequestDuration observes redis request duration
func (m *MetricsServer) ObserveRedisRequestDuration(duration time.Duration) {
	m.redisRequestHistogram.WithLabelValues(m.hostname, "argocd-application-controller").Observe(duration.Seconds())
}

// IncReconcile increments the reconcile counter for an application
func (m *MetricsServer) IncReconcile(app *argoappv1.Application, duration time.Duration) {
	m.reconcileHistogram.WithLabelValues(app.Namespace, app.Spec.Destination.Server).Observe(duration.Seconds())
}

type appCollector struct {
	store     applister.ApplicationLister
	appFilter func(obj interface{}) bool
}

// NewAppCollector returns a prometheus collector for application metrics
func NewAppCollector(appLister applister.ApplicationLister, appFilter func(obj interface{}) bool) prometheus.Collector {
	return &appCollector{
		store:     appLister,
		appFilter: appFilter,
	}
}

// NewAppRegistry creates a new prometheus registry that collects applications
func NewAppRegistry(appLister applister.ApplicationLister, appFilter func(obj interface{}) bool) *prometheus.Registry {
	registry := prometheus.NewRegistry()
	registry.MustRegister(NewAppCollector(appLister, appFilter))
	return registry
}

// Describe implements the prometheus.Collector interface
func (c *appCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- descAppInfo
	ch <- descAppSyncStatusCode
	ch <- descAppHealthStatus
}

// Collect implements the prometheus.Collector interface
func (c *appCollector) Collect(ch chan<- prometheus.Metric) {
	apps, err := c.store.List(labels.NewSelector())
	if err != nil {
		log.Warnf("Failed to collect applications: %v", err)
		return
	}
	for _, app := range apps {
		if c.appFilter(app) {
			collectApps(ch, app)
		}
	}
}

func boolFloat64(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func collectApps(ch chan<- prometheus.Metric, app *argoappv1.Application) {
	addConstMetric := func(desc *prometheus.Desc, t prometheus.ValueType, v float64, lv ...string) {
		project := app.Spec.GetProject()
		lv = append([]string{app.Namespace, app.Name, project}, lv...)
		ch <- prometheus.MustNewConstMetric(desc, t, v, lv...)
	}
	addGauge := func(desc *prometheus.Desc, v float64, lv ...string) {
		addConstMetric(desc, prometheus.GaugeValue, v, lv...)
	}

	var operation string
	if app.DeletionTimestamp != nil {
		operation = "delete"
	} else if app.Operation != nil && app.Operation.Sync != nil {
		operation = "sync"
	}
	syncStatus := app.Status.Sync.Status
	if syncStatus == "" {
		syncStatus = argoappv1.SyncStatusCodeUnknown
	}
	healthStatus := app.Status.Health.Status
	if healthStatus == "" {
		healthStatus = health.HealthStatusUnknown
	}

	addGauge(descAppInfo, 1, git.NormalizeGitURL(app.Spec.Source.RepoURL), app.Spec.Destination.Server, app.Spec.Destination.Namespace, string(syncStatus), string(healthStatus), operation)

	// Deprecated controller metrics
	if os.Getenv(EnvVarLegacyControllerMetrics) == "true" {
		addGauge(descAppCreated, float64(app.CreationTimestamp.Unix()))

		addGauge(descAppSyncStatusCode, boolFloat64(syncStatus == argoappv1.SyncStatusCodeSynced), string(argoappv1.SyncStatusCodeSynced))
		addGauge(descAppSyncStatusCode, boolFloat64(syncStatus == argoappv1.SyncStatusCodeOutOfSync), string(argoappv1.SyncStatusCodeOutOfSync))
		addGauge(descAppSyncStatusCode, boolFloat64(syncStatus == argoappv1.SyncStatusCodeUnknown || syncStatus == ""), string(argoappv1.SyncStatusCodeUnknown))

		healthStatus := app.Status.Health.Status
		addGauge(descAppHealthStatus, boolFloat64(healthStatus == health.HealthStatusUnknown || healthStatus == ""), string(health.HealthStatusUnknown))
		addGauge(descAppHealthStatus, boolFloat64(healthStatus == health.HealthStatusProgressing), string(health.HealthStatusProgressing))
		addGauge(descAppHealthStatus, boolFloat64(healthStatus == health.HealthStatusSuspended), string(health.HealthStatusSuspended))
		addGauge(descAppHealthStatus, boolFloat64(healthStatus == health.HealthStatusHealthy), string(health.HealthStatusHealthy))
		addGauge(descAppHealthStatus, boolFloat64(healthStatus == health.HealthStatusDegraded), string(health.HealthStatusDegraded))
		addGauge(descAppHealthStatus, boolFloat64(healthStatus == health.HealthStatusMissing), string(health.HealthStatusMissing))
	}
}
