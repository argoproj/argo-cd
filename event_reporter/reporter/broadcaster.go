package reporter

import (
	argocommon "github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/event_reporter/metrics"
	"github.com/argoproj/argo-cd/v2/event_reporter/sharding"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/env"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/watch"
	"math"
	"sync"
)

type subscriber struct {
	ch      chan *appv1.ApplicationWatchEvent
	filters []func(*appv1.ApplicationWatchEvent) bool
}

func (s *subscriber) matches(event *appv1.ApplicationWatchEvent) bool {
	for i := range s.filters {
		if !s.filters[i](event) {
			return false
		}
	}
	return true
}

// Broadcaster is an interface for broadcasting application informer watch events to multiple subscribers.
type Broadcaster interface {
	Subscribe(ch chan *appv1.ApplicationWatchEvent, filters ...func(event *appv1.ApplicationWatchEvent) bool) func()
	OnAdd(interface{})
	OnUpdate(interface{}, interface{})
	OnDelete(interface{})
}

type broadcasterHandler struct {
	lock           sync.Mutex
	subscribers    []*subscriber
	filter         sharding.ApplicationFilterFunction
	featureManager *FeatureManager
	metricsServer  *metrics.MetricsServer
	rateLimiter    *RateLimiter
}

func NewBroadcaster(featureManager *FeatureManager, metricsServer *metrics.MetricsServer, rateLimiterOpts *RateLimiterOpts) Broadcaster {
	// todo: pass real value here
	filter := getApplicationFilter("")
	return &broadcasterHandler{
		filter:         filter,
		featureManager: featureManager,
		metricsServer:  metricsServer,
		rateLimiter:    NewRateLimiter(rateLimiterOpts),
	}
}

func (b *broadcasterHandler) notify(event *appv1.ApplicationWatchEvent) {
	// Make a local copy of b.subscribers, then send channel events outside the lock,
	// to avoid data race on b.subscribers changes
	subscribers := []*subscriber{}
	b.lock.Lock()
	subscribers = append(subscribers, b.subscribers...)
	b.lock.Unlock()

	if !b.featureManager.ShouldReporterRun() {
		log.Infof("filtering application '%s', event reporting is turned off and old one is in use", event.Application.Name)
		return
	}

	if b.filter != nil {
		result, expectedShard := b.filter(&event.Application)
		if !result {
			log.Infof("filtering application '%s', wrong shard, should be %d", event.Application.Name, expectedShard)
			return
		}
	}

	for _, s := range subscribers {
		if s.matches(event) {

			duration, err, learningMode := b.rateLimiter.Limit(event.Application.Name)
			errorInLearningMode := learningMode && err != nil
			if err != nil {
				log.Errorf("adding application '%s' to channel failed, due to rate limit, duration left %s, learningMode %t", event.Application.Name, duration.String(), learningMode)
				// if learning mode is enabled, we will continue to send events
				if !learningMode {
					b.metricsServer.IncAppEventsCounter(event.Application.Name, false, false)
					continue
				}
			}

			select {
			case s.ch <- event:
				{
					log.Infof("adding application '%s' to channel", event.Application.Name)
					b.metricsServer.IncAppEventsCounter(event.Application.Name, true, errorInLearningMode)
				}
			default:
				// drop event if cannot send right away
				log.WithField("application", event.Application.Name).Warn("unable to send event notification")
				b.metricsServer.IncAppEventsCounter(event.Application.Name, false, errorInLearningMode)
			}
		}
	}
}

// Subscribe forward application informer watch events to the provided channel.
// The watch events are dropped if no receives are reading events from the channel so the channel must have
// buffer if dropping events is not acceptable.
func (b *broadcasterHandler) Subscribe(ch chan *appv1.ApplicationWatchEvent, filters ...func(event *appv1.ApplicationWatchEvent) bool) func() {
	b.lock.Lock()
	defer b.lock.Unlock()
	subscriber := &subscriber{ch, filters}
	b.subscribers = append(b.subscribers, subscriber)
	return func() {
		b.lock.Lock()
		defer b.lock.Unlock()
		for i := range b.subscribers {
			if b.subscribers[i] == subscriber {
				b.subscribers = append(b.subscribers[:i], b.subscribers[i+1:]...)
				break
			}
		}
	}
}

func (b *broadcasterHandler) OnAdd(obj interface{}) {
	if app, ok := obj.(*appv1.Application); ok {
		b.notify(&appv1.ApplicationWatchEvent{Application: *app, Type: watch.Added})
	}
}

func (b *broadcasterHandler) OnUpdate(_, newObj interface{}) {
	if app, ok := newObj.(*appv1.Application); ok {
		b.notify(&appv1.ApplicationWatchEvent{Application: *app, Type: watch.Modified})
	}
}

func (b *broadcasterHandler) OnDelete(obj interface{}) {
	if app, ok := obj.(*appv1.Application); ok {
		b.notify(&appv1.ApplicationWatchEvent{Application: *app, Type: watch.Deleted})
	}
}

func getApplicationFilter(shardingAlgorithm string) sharding.ApplicationFilterFunction {
	shardingSvc := sharding.NewSharding()
	replicas := env.ParseNumFromEnv(argocommon.EnvEventReporterReplicas, 0, 0, math.MaxInt32)
	var applicationFilter func(app *appv1.Application) (bool, int)
	if replicas > 1 {
		shard := sharding.GetShardNumber()
		log.Infof("Processing applications from shard %d", shard)
		log.Infof("Using filter function:  %s", shardingAlgorithm)
		distributionFunction := shardingSvc.GetDistributionFunction(shardingAlgorithm)
		applicationFilter = shardingSvc.GetApplicationFilter(distributionFunction, shard)
	} else {
		log.Info("Processing all application shards")
	}
	return applicationFilter
}
