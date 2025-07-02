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
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/argoproj/argo-cd/v3/common"
	argoappv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	applister "github.com/argoproj/argo-cd/v3/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/argo"
	"github.com/argoproj/argo-cd/v3/util/db"
	"github.com/argoproj/argo-cd/v3/util/git"
	"github.com/argoproj/argo-cd/v3/util/healthz"
	metricsutil "github.com/argoproj/argo-cd/v3/util/metrics"
	"github.com/argoproj/argo-cd/v3/util/metrics/kubectl"
	"github.com/argoproj/argo-cd/v3/util/profile"
)

type MetricsServer struct {
	*http.Server
	syncCounter                       *prometheus.CounterVec
	syncDuration                      *prometheus.CounterVec
	kubectlExecCounter                *prometheus.CounterVec
	kubectlExecPendingGauge           *prometheus.GaugeVec
	orphanedResourcesGauge            *prometheus.GaugeVec
	k8sRequestCounter                 *prometheus.CounterVec
	clusterEventsCounter              *prometheus.CounterVec
	redisRequestCounter               *prometheus.CounterVec
	reconcileHistogram                *prometheus.HistogramVec
	redisRequestHistogram             *prometheus.HistogramVec
	resourceEventsProcessingHistogram *prometheus.HistogramVec
	resourceEventsNumberGauge         *prometheus.GaugeVec
	registry                          *prometheus.Registry
	hostname                          string
	cron                              *cron.Cron
}

const (
	// MetricsPath is the endpoint to collect application metrics
	MetricsPath = "/metrics"
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

	syncCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argocd_app_sync_total",
			Help: "Number of application syncs.",
		},
		append(descAppDefaultLabels, "dest_server", "phase", "dry_run"),
	)

	syncDuration = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argocd_app_sync_duration_seconds_total",
			Help: "Application sync performance in seconds total.",
		},
		append(descAppDefaultLabels, "dest_server"),
	)

	k8sRequestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argocd_app_k8s_request_total",
			Help: "Number of kubernetes requests executed during application reconciliation.",
		},
		append(descAppDefaultLabels, "server", "response_code", "verb", "resource_kind", "resource_namespace", "dry_run"),
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

	orphanedResourcesGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "argocd_app_orphaned_resources_count",
			Help: "Number of orphaned resources per application",
		},
		descAppDefaultLabels,
	)

	resourceEventsProcessingHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "argocd_resource_events_processing",
			Help:    "Time to process resource events in seconds.",
			Buckets: []float64{0.25, .5, 1, 2, 4, 8, 16},
		},
		[]string{"server"},
	)

	resourceEventsNumberGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "argocd_resource_events_processed_in_batch",
		Help: "Number of resource events processed in batch",
	}, []string{"server"})
)

// NewMetricsServer returns a new prometheus server which collects application metrics
func NewMetricsServer(addr string, appLister applister.ApplicationLister, appFilter func(obj any) bool, healthCheck func(r *http.Request) error, appLabels []string, appConditions []string, db db.ArgoDB) (*MetricsServer, error) {
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
	registry := NewAppRegistry(appLister, appFilter, appLabels, appConditions, db)

	mux.Handle(MetricsPath, promhttp.HandlerFor(prometheus.Gatherers{
		// contains app controller specific metrics
		registry,
		// contains workqueue metrics, process and golang metrics
		ctrlmetrics.Registry,
	}, promhttp.HandlerOpts{}))
	profile.RegisterProfiler(mux)
	healthz.ServeHealthCheck(mux, healthCheck)

	registry.MustRegister(syncCounter)
	registry.MustRegister(syncDuration)
	registry.MustRegister(k8sRequestCounter)
	registry.MustRegister(kubectlExecCounter)
	registry.MustRegister(kubectlExecPendingGauge)
	registry.MustRegister(orphanedResourcesGauge)
	registry.MustRegister(reconcileHistogram)
	registry.MustRegister(clusterEventsCounter)
	registry.MustRegister(redisRequestCounter)
	registry.MustRegister(redisRequestHistogram)
	registry.MustRegister(resourceEventsProcessingHistogram)
	registry.MustRegister(resourceEventsNumberGauge)

	kubectl.RegisterWithClientGo()
	kubectl.RegisterWithPrometheus(registry)

	metricsServer := &MetricsServer{
		registry: registry,
		Server: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
		syncCounter:                       syncCounter,
		syncDuration:                      syncDuration,
		k8sRequestCounter:                 k8sRequestCounter,
		kubectlExecCounter:                kubectlExecCounter,
		kubectlExecPendingGauge:           kubectlExecPendingGauge,
		orphanedResourcesGauge:            orphanedResourcesGauge,
		reconcileHistogram:                reconcileHistogram,
		clusterEventsCounter:              clusterEventsCounter,
		redisRequestCounter:               redisRequestCounter,
		redisRequestHistogram:             redisRequestHistogram,
		resourceEventsProcessingHistogram: resourceEventsProcessingHistogram,
		resourceEventsNumberGauge:         resourceEventsNumberGauge,
		hostname:                          hostname,
		// This cron is used to expire the metrics cache.
		// Currently clearing the metrics cache is logging and deleting from the map
		// so there is no possibility of panic, but we will add a chain to keep robfig/cron v1 behavior.
		cron: cron.New(cron.WithChain(cron.Recover(cron.PrintfLogger(log.StandardLogger())))),
	}

	return metricsServer, nil
}

func (m *MetricsServer) RegisterClustersInfoSource(ctx context.Context, source HasClustersInfo, db db.ArgoDB, clusterLabels []string) {
	collector := NewClusterCollector(ctx, source, db.ListClusters, clusterLabels)
	m.registry.MustRegister(collector)
}

// IncSync increments the sync counter for an application
func (m *MetricsServer) IncSync(app *argoappv1.Application, destServer string, state *argoappv1.OperationState) {
	if !state.Phase.Completed() {
		return
	}
	isDryRun := app.Operation != nil && app.Operation.DryRun()
	m.syncCounter.WithLabelValues(app.Namespace, app.Name, app.Spec.GetProject(), destServer, string(state.Phase), strconv.FormatBool(isDryRun)).Inc()
}

// IncAppSyncDuration observes app sync duration
func (m *MetricsServer) IncAppSyncDuration(app *argoappv1.Application, destServer string, state *argoappv1.OperationState) {
	if state.FinishedAt != nil {
		m.syncDuration.WithLabelValues(app.Namespace, app.Name, app.Spec.GetProject(), destServer).
			Add(float64(time.Duration(state.FinishedAt.Unix() - state.StartedAt.Unix())))
	}
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

func (m *MetricsServer) SetOrphanedResourcesMetric(app *argoappv1.Application, numOrphanedResources int) {
	m.orphanedResourcesGauge.WithLabelValues(app.Namespace, app.Name, app.Spec.GetProject()).Set(float64(numOrphanedResources))
}

// IncClusterEventsCount increments the number of cluster events
func (m *MetricsServer) IncClusterEventsCount(server, group, kind string) {
	m.clusterEventsCounter.WithLabelValues(server, group, kind).Inc()
}

// IncKubernetesRequest increments the kubernetes requests counter for an application
func (m *MetricsServer) IncKubernetesRequest(app *argoappv1.Application, server, statusCode, verb, resourceKind, resourceNamespace string) {
	var namespace, name, project string
	isDryRun := false
	if app != nil {
		namespace = app.Namespace
		name = app.Name
		project = app.Spec.GetProject()
		isDryRun = app.Operation != nil && app.Operation.DryRun()
	}
	m.k8sRequestCounter.WithLabelValues(
		namespace, name, project, server, statusCode,
		verb, resourceKind, resourceNamespace, strconv.FormatBool(isDryRun),
	).Inc()
}

func (m *MetricsServer) IncRedisRequest(failed bool) {
	m.redisRequestCounter.WithLabelValues(m.hostname, common.ApplicationController, strconv.FormatBool(failed)).Inc()
}

// ObserveRedisRequestDuration observes redis request duration
func (m *MetricsServer) ObserveRedisRequestDuration(duration time.Duration) {
	m.redisRequestHistogram.WithLabelValues(m.hostname, common.ApplicationController).Observe(duration.Seconds())
}

// ObserveResourceEventsProcessingDuration observes resource events processing duration
func (m *MetricsServer) ObserveResourceEventsProcessingDuration(server string, duration time.Duration, processedEventsNumber int) {
	m.resourceEventsProcessingHistogram.WithLabelValues(server).Observe(duration.Seconds())
	m.resourceEventsNumberGauge.WithLabelValues(server).Set(float64(processedEventsNumber))
}

// IncReconcile increments the reconcile counter for an application
func (m *MetricsServer) IncReconcile(app *argoappv1.Application, destServer string, duration time.Duration) {
	m.reconcileHistogram.WithLabelValues(app.Namespace, destServer).Observe(duration.Seconds())
}

// HasExpiration return true if expiration is set
func (m *MetricsServer) HasExpiration() bool {
	return len(m.cron.Entries()) > 0
}

// SetExpiration reset Prometheus metrics based on time duration interval
func (m *MetricsServer) SetExpiration(cacheExpiration time.Duration) error {
	if m.HasExpiration() {
		return errors.New("expiration is already set")
	}

	_, err := m.cron.AddFunc(fmt.Sprintf("@every %s", cacheExpiration), func() {
		log.Infof("Reset Prometheus metrics based on existing expiration '%v'", cacheExpiration)
		m.syncCounter.Reset()
		m.syncDuration.Reset()
		m.kubectlExecCounter.Reset()
		m.kubectlExecPendingGauge.Reset()
		m.orphanedResourcesGauge.Reset()
		m.k8sRequestCounter.Reset()
		m.clusterEventsCounter.Reset()
		m.redisRequestCounter.Reset()
		m.reconcileHistogram.Reset()
		m.redisRequestHistogram.Reset()
		m.resourceEventsProcessingHistogram.Reset()
		m.resourceEventsNumberGauge.Reset()
		kubectl.ResetAll()
	})
	if err != nil {
		return err
	}

	m.cron.Start()
	return nil
}

type appCollector struct {
	store         applister.ApplicationLister
	appFilter     func(obj any) bool
	appLabels     []string
	appConditions []string
	db            db.ArgoDB
}

// NewAppCollector returns a prometheus collector for application metrics
func NewAppCollector(appLister applister.ApplicationLister, appFilter func(obj any) bool, appLabels []string, appConditions []string, db db.ArgoDB) prometheus.Collector {
	return &appCollector{
		store:         appLister,
		appFilter:     appFilter,
		appLabels:     appLabels,
		appConditions: appConditions,
		db:            db,
	}
}

// NewAppRegistry creates a new prometheus registry that collects applications
func NewAppRegistry(appLister applister.ApplicationLister, appFilter func(obj any) bool, appLabels []string, appConditions []string, db db.ArgoDB) *prometheus.Registry {
	registry := prometheus.NewRegistry()
	registry.MustRegister(NewAppCollector(appLister, appFilter, appLabels, appConditions, db))
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
}

// Collect implements the prometheus.Collector interface
func (c *appCollector) Collect(ch chan<- prometheus.Metric) {
	apps, err := c.store.List(labels.NewSelector())
	if err != nil {
		log.Warnf("Failed to collect applications: %v", err)
		return
	}
	for _, app := range apps {
		if !c.appFilter(app) {
			continue
		}
		destCluster, err := argo.GetDestinationCluster(context.Background(), app.Spec.Destination, c.db)
		if err != nil {
			log.Warnf("Failed to get destination cluster for application %s: %v", app.Name, err)
		}
		destServer := ""
		if destCluster != nil {
			destServer = destCluster.Server
		}
		c.collectApps(ch, app, destServer)
	}
}

func boolFloat64(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func (c *appCollector) collectApps(ch chan<- prometheus.Metric, app *argoappv1.Application, destServer string) {
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

	autoSyncEnabled := app.Spec.SyncPolicy != nil && app.Spec.SyncPolicy.IsAutomatedSyncEnabled()

	addGauge(descAppInfo, 1, strconv.FormatBool(autoSyncEnabled), git.NormalizeGitURL(app.Spec.GetSource().RepoURL), destServer, app.Spec.Destination.Namespace, string(syncStatus), string(healthStatus), operation)

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
}
