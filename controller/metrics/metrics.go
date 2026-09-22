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

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/health"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/robfig/cron/v3"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/argoproj/argo-cd/v3/common"
	argoappv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	applister "github.com/argoproj/argo-cd/v3/pkg/client/listers/application/v1alpha1"
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
		append(descAppDefaultLabels, "autosync_enabled", "repo", "dest_server", "dest_namespace", "sync_status", "health_status", "hydrator_status", "operation", "phase"),
		nil,
	)

	descAppSyncWindow = prometheus.NewDesc(
		"argocd_app_sync_window",
		"Whether a sync window of the given kind is currently active for the application. Emitted as a 0/1 gauge per window_kind (\"allow\", \"deny\"); 1 means at least one matching window of that kind is currently active.",
		append(descAppDefaultLabels, "window_kind"),
		nil,
	)

	descAppSyncBlocked = prometheus.NewDesc(
		"argocd_app_sync_blocked",
		"Whether automatic syncs of the application are currently blocked by its project's sync windows. Emitted as a 0/1 gauge: 1 means an automatic sync attempt right now would be rejected. Reports 0 when no sync windows are configured, distinguishing that case from \"allow=0, deny=0\" caused by inactive allow windows. Also reports 1 when the windows cannot be evaluated, because a real sync attempt would fail in the same state; use argocd_app_sync_window_error to tell the two apart.",
		descAppDefaultLabels,
		nil,
	)

	descAppSyncWindowError = prometheus.NewDesc(
		"argocd_app_sync_window_error",
		"Whether the application's sync windows could not be evaluated. Emitted as a 0/1 gauge: 1 means the AppProject could not be resolved, or a window matching the application has a schedule or duration that cannot be parsed, so argocd_app_sync_window does not reflect the configured windows and argocd_app_sync_blocked is reported fail-closed as 1.",
		descAppDefaultLabels,
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
		[]string{"hostname", "initiator", "command", "failed"},
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

	argoVersion = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "argocd_info",
			Help: "ArgoCD version information",
		},
		[]string{"version"},
	)
)

// AppProjectGetter resolves the AppProject for a given Application. It may be nil,
// in which case sync window metrics are not emitted.
type AppProjectGetter func(app *argoappv1.Application) (*argoappv1.AppProject, error)

// NewMetricsServer returns a new prometheus server which collects application metrics
func NewMetricsServer(addr string, appLister applister.ApplicationLister, appFilter AppFilter, healthCheck func(r *http.Request) error, appLabels []string, appConditions []string, getAppProject AppProjectGetter) (*MetricsServer, error) {
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
	registry := NewAppRegistry(appLister, appFilter, appLabels, appConditions, getAppProject)

	mux.Handle(MetricsPath, promhttp.HandlerFor(prometheus.Gatherers{
		// contains app controller specific metrics
		registry,
		// contains workqueue metrics, process and golang metrics
		ctrlmetrics.Registry,
	}, promhttp.HandlerOpts{}))
	argoVersion.WithLabelValues(common.GetVersion().Version).Set(1)
	profile.RegisterProfiler(mux)
	healthz.ServeHealthCheck(mux, healthCheck)

	registry.MustRegister(argoVersion)
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

func (m *MetricsServer) IncRedisRequest(command string, failed bool) {
	m.redisRequestCounter.WithLabelValues(m.hostname, common.CommandApplicationController, command, strconv.FormatBool(failed)).Inc()
}

// ObserveRedisRequestDuration observes redis request duration
func (m *MetricsServer) ObserveRedisRequestDuration(duration time.Duration) {
	m.redisRequestHistogram.WithLabelValues(m.hostname, common.CommandApplicationController).Observe(duration.Seconds())
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

// AppFilter reports whether an Application should be exported, and returns the destination server
// it resolved on the way. Resolving isn't free, so the collector reuses this instead of resolving
// again. destServer is empty and err is set when the destination doesn't resolve, which the
// collector logs per scrape; keep is independent of err, an unresolvable destination is still
// exported.
type AppFilter func(obj any) (keep bool, destServer string, err error)

type appCollector struct {
	store         applister.ApplicationLister
	appFilter     AppFilter
	appLabels     []string
	appConditions []string
	getAppProject AppProjectGetter
}

// NewAppCollector returns a prometheus collector for application metrics
func NewAppCollector(appLister applister.ApplicationLister, appFilter AppFilter, appLabels []string, appConditions []string, getAppProject AppProjectGetter) prometheus.Collector {
	return &appCollector{
		store:         appLister,
		appFilter:     appFilter,
		appLabels:     appLabels,
		appConditions: appConditions,
		getAppProject: getAppProject,
	}
}

// NewAppRegistry creates a new prometheus registry that collects applications
func NewAppRegistry(appLister applister.ApplicationLister, appFilter AppFilter, appLabels []string, appConditions []string, getAppProject AppProjectGetter) *prometheus.Registry {
	registry := prometheus.NewRegistry()
	registry.MustRegister(NewAppCollector(appLister, appFilter, appLabels, appConditions, getAppProject))
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
	if c.getAppProject != nil {
		ch <- descAppSyncWindow
		ch <- descAppSyncBlocked
		ch <- descAppSyncWindowError
	}
}

// Collect implements the prometheus.Collector interface
func (c *appCollector) Collect(ch chan<- prometheus.Metric) {
	apps, err := c.store.List(labels.NewSelector())
	if err != nil {
		log.Warnf("Failed to collect applications: %v", err)
		return
	}
	var syncWindows *syncWindowScrape
	if c.getAppProject != nil {
		syncWindows = newSyncWindowScrape(c.getAppProject)
	}
	for _, app := range apps {
		keep, destServer, err := c.appFilter(app)
		if !keep {
			continue
		}
		if err != nil {
			log.Warnf("Failed to get destination cluster for application %s: %v", app.Name, err)
		}
		c.collectApps(ch, app, destServer, syncWindows)
	}
}

func boolFloat64(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func (c *appCollector) collectApps(ch chan<- prometheus.Metric, app *argoappv1.Application, destServer string, syncWindows *syncWindowScrape) {
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
	var hydratorStatus string
	if app.Spec.SourceHydrator != nil {
		hydratorStatus = string(argoappv1.HydrateOperationPhaseUnknown)
		if op := app.Status.SourceHydrator.CurrentOperation; op != nil && op.Phase != "" {
			hydratorStatus = string(op.Phase)
		}
	}

	autoSyncEnabled := app.Spec.SyncPolicy != nil && app.Spec.SyncPolicy.IsAutomatedSyncEnabled()

	var operationPhase string
	if app.Status.OperationState != nil {
		operationPhase = string(app.Status.OperationState.Phase)
	}

	addGauge(descAppInfo, 1, strconv.FormatBool(autoSyncEnabled), git.NormalizeGitURL(app.Spec.GetSource().RepoURL), destServer, app.Spec.Destination.Namespace, string(syncStatus), string(healthStatus), hydratorStatus, operation, operationPhase)

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

	if syncWindows != nil {
		allowActive, denyActive, blocked, failed, err := syncWindows.evaluate(app)
		if err != nil {
			// Once per project per scrape: evaluate only returns the error to
			// the application that built the project's window state. It says
			// nothing about this application, which may not match the window
			// that failed.
			log.Debugf("Sync windows of AppProject %s could not be evaluated, reporting the applications they match as blocked: %v", app.Spec.GetProject(), err)
		}
		addGauge(descAppSyncWindow, boolFloat64(allowActive), "allow")
		addGauge(descAppSyncWindow, boolFloat64(denyActive), "deny")
		addGauge(descAppSyncBlocked, boolFloat64(blocked))
		addGauge(descAppSyncWindowError, boolFloat64(failed))
	}
}

// projectWindows is one project's sync window state for one scrape: an
// evaluator built from its windows, or a project that could not be resolved at
// all. A project that resolves but has malformed windows still gets an
// evaluator: those windows block only the applications they match.
type projectWindows struct {
	evaluator    *argoappv1.SyncWindowEvaluator
	lookupFailed bool
	// Whether building the evaluator reported malformed windows, which
	// windows() has already returned once for the project.
	windowsFailed bool
}

// syncWindowScrape is the state shared by every application in one scrape.
// Window state is per project, but collection walks applications, so without
// this a project shared by N applications resolves and evaluates its windows N
// times per scrape. AppProjectGetter caches successful lookups but not
// failures, and caches nothing about the windows themselves.
type syncWindowScrape struct {
	getAppProject AppProjectGetter
	// One instant for the whole scrape, so applications cannot disagree about
	// a window boundary crossed while it runs.
	now      time.Time
	projects map[string]projectWindows
}

func newSyncWindowScrape(getAppProject AppProjectGetter) *syncWindowScrape {
	return &syncWindowScrape{
		getAppProject: getAppProject,
		now:           time.Now(),
		projects:      map[string]projectWindows{},
	}
}

// windows resolves app's project and builds its evaluator, once per project per
// scrape. Keyed by namespace too, since the getter also checks whether the
// project permits it. The error is returned only on that first build, so a
// broken project is reported once per scrape rather than once per application;
// later applications still see failed and report fail-closed.
func (s *syncWindowScrape) windows(app *argoappv1.Application) (projectWindows, error) {
	key := app.Spec.GetProject() + "/" + app.Namespace
	if cached, ok := s.projects[key]; ok {
		return cached, nil
	}
	entry, err := s.buildWindows(app)
	s.projects[key] = entry
	return entry, err
}

func (s *syncWindowScrape) buildWindows(app *argoappv1.Application) (projectWindows, error) {
	proj, err := s.getAppProject(app)
	if err != nil {
		return projectWindows{lookupFailed: true}, fmt.Errorf("failed to get the AppProject of applications in namespace %s: %w", app.Namespace, err)
	}
	if proj == nil {
		// Indistinguishable from a project that configures no windows.
		return projectWindows{}, nil
	}
	// A malformed window only blocks the applications it matches, so the
	// evaluator stays usable; the error is returned to be logged once for the
	// project rather than once per application.
	evaluator, err := proj.Spec.SyncWindows.Evaluator(s.now)
	if err != nil {
		return projectWindows{evaluator: evaluator, windowsFailed: true}, fmt.Errorf("some of its sync windows could not be evaluated: %w", err)
	}
	return projectWindows{evaluator: evaluator}, nil
}

// evaluate returns the sync window gauge values for app. Any failure sets
// blocked and failed, fail-closed, because a real sync would fail in the same
// state.
//
// The gauges come from entry, never from err: err is only the project-level
// failure the caller logs, and windows() returns it to the one application that
// built the entry. Every later application in a broken project sees err == nil
// and must still report fail-closed.
func (s *syncWindowScrape) evaluate(app *argoappv1.Application) (allowActive, denyActive, blocked, failed bool, err error) {
	entry, err := s.windows(app)
	if entry.lookupFailed {
		return false, false, true, true, err
	}
	if entry.evaluator == nil {
		return false, false, false, false, err
	}
	// blocked comes from CanSync, the same call that gates automatic syncs, so
	// the gauge cannot drift from it. One evaluation feeds all three gauges:
	// asking separately reads the clock twice, and a boundary in between would
	// report an active allow window alongside blocked=1.
	canSync, allowActive, denyActive, evalErr := entry.evaluator.CanSyncWithActiveKinds(app)
	if evalErr != nil {
		// Unreachable today: the evaluator can only fail on a matched window
		// that Evaluator already reported for the project, so windowsFailed is
		// always set here and err carries that report. canSyncFrom's own error
		// paths need an operation start time, which the evaluator never has.
		// Kept so that a future one cannot pass silently.
		if !entry.windowsFailed {
			err = evalErr
		}
		return false, false, true, true, err
	}
	return allowActive, denyActive, !canSync, false, err
}
