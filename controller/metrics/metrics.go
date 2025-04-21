package metrics

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strconv"
	"time"

	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/robfig/cron/v3"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/argoproj/argo-cd/v2/common"
	argoappv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	applister "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/git"
	"github.com/argoproj/argo-cd/v2/util/healthz"
	metricsutil "github.com/argoproj/argo-cd/v2/util/metrics"
	"github.com/argoproj/argo-cd/v2/util/profile"

	ctrl_metrics "sigs.k8s.io/controller-runtime/pkg/metrics"
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
	cron                    *cron.Cron
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

	descAppLabels     *prometheus.Desc
	descAppConditions *prometheus.Desc

	descAppInfo = prometheus.NewDesc(
		"argocd_app_info",
		"Information about application.",
		append(descAppDefaultLabels, "autosync_enabled", "repo", "dest_server", "dest_namespace", "sync_status", "health_status", "operation"),
		nil,
	)

	// Deprecated
	descAppCreated = prometheus.NewDesc(
		"argocd_app_created_time",
		"Creation time in unix timestamp for an application.",
		descAppDefaultLabels,
		nil,
	)
	// Deprecated: superseded by sync_status label in argocd_app_info
	descAppSyncStatusCode = prometheus.NewDesc(
		"argocd_app_sync_status",
		"The application current sync status.",
		append(descAppDefaultLabels, "sync_status"),
		nil,
	)
	// Deprecated: superseded by health_status label in argocd_app_info
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
			Help: "Application reconciliation performance in seconds.",
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
			Help: "Number of redis requests executed during application reconciliation.",
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
func NewMetricsServer(addr string, appLister applister.ApplicationLister, appFilter func(obj interface{}) bool, healthCheck func(r *http.Request) error, appLabels []string, appConditions []string) (*MetricsServer, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	if len(appLabels) > 0 {
		normalizedLabels := metricsutil.NormalizeLabels("label", appLabels)
		descAppLabels = prometheus.NewDesc(
			"argocd_app_labels",
			"Argo Application labels converted to Prometheus labels",
			append(descAppDefaultLabels, normalizedLabels...),
			nil,
		)
	}

	if len(appConditions) > 0 {
		descAppConditions = prometheus.NewDesc(
			"argocd_app_condition",
			"Report application conditions.",
			append(descAppDefaultLabels, "condition"),
			nil,
		)
	}

	mux := http.NewServeMux()
	registry := NewAppRegistry(appLister, appFilter, appLabels, appConditions)

	mux.Handle(MetricsPath, promhttp.HandlerFor(prometheus.Gatherers{
		// contains app controller specific metrics
		registry,
		// contains workqueue metrics, process and golang metrics
		ctrl_metrics.Registry,
	}, promhttp.HandlerOpts{}))
	profile.RegisterProfiler(mux)
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
		// This cron is used to expire the metrics cache.
		// Currently clearing the metrics cache is logging and deleting from the map
		// so there is no possibility of panic, but we will add a chain to keep robfig/cron v1 behavior.
		cron: cron.New(cron.WithChain(cron.Recover(cron.PrintfLogger(log.StandardLogger())))),
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
	m.redisRequestCounter.WithLabelValues(m.hostname, common.ApplicationController, strconv.FormatBool(failed)).Inc()
}

// ObserveRedisRequestDuration observes redis request duration
func (m *MetricsServer) ObserveRedisRequestDuration(duration time.Duration) {
	m.redisRequestHistogram.WithLabelValues(m.hostname, common.ApplicationController).Observe(duration.Seconds())
}

// IncReconcile increments the reconcile counter for an application
func (m *MetricsServer) IncReconcile(app *argoappv1.Application, duration time.Duration) {
	m.reconcileHistogram.WithLabelValues(app.Namespace, app.Spec.Destination.Server).Observe(duration.Seconds())
}

// HasExpiration return true if expiration is set
func (m *MetricsServer) HasExpiration() bool {
	return len(m.cron.Entries()) > 0
}

// SetExpiration reset Prometheus metrics based on time duration interval
func (m *MetricsServer) SetExpiration(cacheExpiration time.Duration) error {
	if m.HasExpiration() {
		return errors.New("Expiration is already set")
	}

	_, err := m.cron.AddFunc(fmt.Sprintf("@every %s", cacheExpiration), func() {
		log.Infof("Reset Prometheus metrics based on existing expiration '%v'", cacheExpiration)
		m.syncCounter.Reset()
		m.kubectlExecCounter.Reset()
		m.kubectlExecPendingGauge.Reset()
		m.k8sRequestCounter.Reset()
		m.clusterEventsCounter.Reset()
		m.redisRequestCounter.Reset()
		m.reconcileHistogram.Reset()
		m.redisRequestHistogram.Reset()
	})
	if err != nil {
		return err
	}

	m.cron.Start()
	return nil
}

type appCollector struct {
	store         applister.ApplicationLister
	appFilter     func(obj interface{}) bool
	appLabels     []string
	appConditions []string
}

// NewAppCollector returns a prometheus collector for application metrics
func NewAppCollector(appLister applister.ApplicationLister, appFilter func(obj interface{}) bool, appLabels []string, appConditions []string) prometheus.Collector {
	return &appCollector{
		store:         appLister,
		appFilter:     appFilter,
		appLabels:     appLabels,
		appConditions: appConditions,
	}
}

// NewAppRegistry creates a new prometheus registry that collects applications
func NewAppRegistry(appLister applister.ApplicationLister, appFilter func(obj interface{}) bool, appLabels []string, appConditions []string) *prometheus.Registry {
	registry := prometheus.NewRegistry()
	registry.MustRegister(NewAppCollector(appLister, appFilter, appLabels, appConditions))
	return registry
}

// Describe implements the prometheus.Collector interface
func (c *appCollector) Describe(ch chan<- *prometheus.Desc) {
	if len(c.appLabels) > 0 {
		ch <- descAppLabels
	}
	if len(c.appConditions) > 0 {
		ch <- descAppConditions
	}
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
			c.collectApps(ch, app)
		}
	}
}

func boolFloat64(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func (c *appCollector) collectApps(ch chan<- prometheus.Metric, app *argoappv1.Application) {
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

	autoSyncEnabled := app.Spec.SyncPolicy != nil && app.Spec.SyncPolicy.Automated != nil

	addGauge(descAppInfo, 1, strconv.FormatBool(autoSyncEnabled), git.NormalizeGitURL(app.Spec.GetSource().RepoURL), app.Spec.Destination.Server, app.Spec.Destination.Namespace, string(syncStatus), string(healthStatus), operation)

	if len(c.appLabels) > 0 {
		labelValues := []string{}
		for _, desiredLabel := range c.appLabels {
			value := app.GetLabels()[desiredLabel]
			labelValues = append(labelValues, value)
		}
		addGauge(descAppLabels, 1, labelValues...)
	}

	if len(c.appConditions) > 0 {
		conditionCount := make(map[string]int)
		for _, condition := range app.Status.Conditions {
			if slices.Contains(c.appConditions, condition.Type) {
				conditionCount[condition.Type]++
			}
		}

		for conditionType, count := range conditionCount {
			addGauge(descAppConditions, float64(count), conditionType)
		}
	}

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
