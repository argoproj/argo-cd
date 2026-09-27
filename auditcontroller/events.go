package auditcontroller

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/audit"
)

// DefaultEventReasons are the event reasons turned into audit records by
// default. Status updates (sync/health status changes) are left out because
// they describe the state of the cluster, not actions somebody took.
var DefaultEventReasons = []string{
	"OperationStarted",
	"OperationCompleted",
	"ResourceCreated",
	"ResourceDeleted",
	"ResourceActionRan",
}

// DefaultIgnoredEventSources are event sources whose events are not turned into
// audit records by default: the API server ships richer records itself.
var DefaultIgnoredEventSources = []string{common.CommandServer}

// EventWatcherOptions configure an EventWatcher.
type EventWatcherOptions struct {
	// Namespaces to watch; an empty string means all namespaces.
	Namespaces []string
	// Reasons of the events to record; empty means all reasons.
	Reasons []string
	// IgnoredSources are event source components whose events are ignored.
	IgnoredSources []string
}

// EventWatcher turns Kubernetes Events emitted by Argo CD for its own objects
// (Applications, ApplicationSets, AppProjects) into audit records.
type EventWatcher struct {
	client         kubernetes.Interface
	recorder       *Recorder
	metrics        *Metrics
	namespaces     []string
	reasons        map[string]bool
	ignoredSources map[string]bool
	startedAt      time.Time
	seen           *seenCache
}

// NewEventWatcher returns an EventWatcher.
func NewEventWatcher(client kubernetes.Interface, recorder *Recorder, metrics *Metrics, opts EventWatcherOptions) *EventWatcher {
	toSet := func(items []string) map[string]bool {
		set := map[string]bool{}
		for _, i := range items {
			if i = strings.TrimSpace(i); i != "" {
				set[i] = true
			}
		}
		return set
	}
	namespaces := opts.Namespaces
	if len(namespaces) == 0 {
		namespaces = []string{metav1.NamespaceAll}
	}
	return &EventWatcher{
		client:         client,
		recorder:       recorder,
		metrics:        metrics,
		namespaces:     namespaces,
		reasons:        toSet(opts.Reasons),
		ignoredSources: toSet(opts.IgnoredSources),
		startedAt:      time.Now(),
		seen:           newSeenCache(10000),
	}
}

// Run watches events until ctx is done.
func (w *EventWatcher) Run(ctx context.Context) error {
	// Only events about Argo CD's own objects.
	selector := fields.OneTermEqualSelector("involvedObject.apiVersion", v1alpha1.SchemeGroupVersion.String()).String()

	var synced []cache.InformerSynced
	for _, ns := range w.namespaces {
		informer := cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListWithContextFunc: func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
					options.FieldSelector = selector
					return w.client.CoreV1().Events(ns).List(ctx, options)
				},
				WatchFuncWithContext: func(ctx context.Context, options metav1.ListOptions) (watch.Interface, error) {
					options.FieldSelector = selector
					return w.client.CoreV1().Events(ns).Watch(ctx, options)
				},
			},
			&corev1.Event{},
			0,
			cache.Indexers{},
		)
		_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj any) {
				if e, ok := obj.(*corev1.Event); ok {
					w.handle(e)
				}
			},
			UpdateFunc: func(_, obj any) {
				// Kubernetes aggregates identical events by bumping their count.
				if e, ok := obj.(*corev1.Event); ok {
					w.handle(e)
				}
			},
		})
		if err != nil {
			return fmt.Errorf("failed to add event handler: %w", err)
		}
		go informer.Run(ctx.Done())
		synced = append(synced, informer.HasSynced)
		log.Infof("Watching Argo CD events in namespace %q", displayNamespace(ns))
	}
	if !cache.WaitForCacheSync(ctx.Done(), synced...) {
		return fmt.Errorf("timed out waiting for event caches to sync")
	}
	<-ctx.Done()
	return nil
}

func displayNamespace(ns string) string {
	if ns == metav1.NamespaceAll {
		return "*"
	}
	return ns
}

func (w *EventWatcher) handle(e *corev1.Event) {
	rec, reason := w.toRecord(e)
	if rec == nil {
		w.metrics.eventProcessed(reason)
		return
	}
	if err := w.recorder.Record(*rec); err != nil {
		log.WithError(err).Error("Failed to write audit record for event")
	}
	w.metrics.eventProcessed("recorded")
}

// toRecord converts an event into an audit record. It returns nil and the
// reason when the event is not recorded.
func (w *EventWatcher) toRecord(e *corev1.Event) (*audit.Record, string) {
	source := eventSource(e)
	if w.ignoredSources[source] {
		return nil, "ignored_source"
	}
	if len(w.reasons) > 0 && !w.reasons[e.Reason] {
		return nil, "ignored_reason"
	}
	ts := eventTime(e)
	// Events that happened before the controller started were recorded by a
	// previous instance (or predate auditing); don't record them twice.
	if ts.Before(w.startedAt) {
		return nil, "before_start"
	}
	if !w.seen.add(fmt.Sprintf("%s/%d", e.UID, eventCount(e))) {
		return nil, "duplicate"
	}

	username := e.Annotations["user"]
	if username == "" {
		username = "system:" + source
	}
	details := map[string]string{
		"event.reason": e.Reason,
		"event.type":   e.Type,
	}
	for _, key := range []string{"dest-server", "dest-namespace"} {
		if v := e.Annotations[key]; v != "" {
			details[key] = v
		}
	}
	if strings.HasPrefix(e.Message, "Initiated automated sync") {
		details["automated"] = "true"
	}
	project := e.Labels["project"]
	if e.InvolvedObject.Kind == application.AppProjectKind {
		project = e.InvolvedObject.Name
	}

	result := audit.Result{Status: audit.ResultSuccess, Code: e.Reason, Message: audit.Truncate(e.Message, audit.MaxMessageLength)}
	if e.Type == corev1.EventTypeWarning {
		result.Status = audit.ResultFailure
	}

	resourceType := strings.ToLower(e.InvolvedObject.Kind)
	if resourceType == strings.ToLower(application.AppProjectKind) {
		resourceType = "project"
	}
	return &audit.Record{
		Kind:      audit.RecordKind,
		ID:        string(e.UID) + "-" + fmt.Sprint(eventCount(e)),
		Timestamp: ts.UTC(),
		Source:    audit.SourceKubernetesEvent,
		Actor:     audit.Actor{Username: username, UserAgent: source},
		Action:    resourceType + "." + eventVerb(e.Reason),
		Verb:      eventVerb(e.Reason),
		Resource: audit.Resource{
			Type:      resourceType,
			Name:      e.InvolvedObject.Name,
			Namespace: e.InvolvedObject.Namespace,
			Project:   project,
		},
		Details: details,
		Result:  result,
	}, ""
}

// eventVerb maps the reasons used by Argo CD's audit events to verbs.
func eventVerb(reason string) string {
	switch reason {
	case "OperationStarted":
		return "operation-started"
	case "OperationCompleted":
		return "operation-completed"
	case "ResourceCreated":
		return "create"
	case "ResourceUpdated":
		return "update"
	case "ResourceDeleted":
		return "delete"
	case "ResourceActionRan":
		return "run-action"
	default:
		return strings.ToLower(reason)
	}
}

func eventSource(e *corev1.Event) string {
	if e.Source.Component != "" {
		return e.Source.Component
	}
	if e.ReportingController != "" {
		return e.ReportingController
	}
	return "unknown"
}

func eventTime(e *corev1.Event) time.Time {
	switch {
	case !e.LastTimestamp.IsZero():
		return e.LastTimestamp.Time
	case !e.EventTime.IsZero():
		return e.EventTime.Time
	case !e.FirstTimestamp.IsZero():
		return e.FirstTimestamp.Time
	default:
		return e.CreationTimestamp.Time
	}
}

func eventCount(e *corev1.Event) int32 {
	if e.Series != nil && e.Series.Count > 0 {
		return e.Series.Count
	}
	return e.Count
}

// seenCache remembers a bounded number of keys, forgetting the oldest first.
type seenCache struct {
	mu    sync.Mutex
	keys  map[string]struct{}
	order []string
	next  int
}

func newSeenCache(size int) *seenCache {
	return &seenCache{keys: make(map[string]struct{}, size), order: make([]string, size)}
}

// add records key and reports whether it was new. Several informers (one per
// watched namespace) may call it concurrently.
func (c *seenCache) add(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.keys[key]; ok {
		return false
	}
	if old := c.order[c.next]; old != "" {
		delete(c.keys, old)
	}
	c.order[c.next] = key
	c.next = (c.next + 1) % len(c.order)
	c.keys[key] = struct{}{}
	return true
}
