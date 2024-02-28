package controller

import (
	"context"
	appclient "github.com/argoproj/argo-cd/v2/event_reporter/application"
	"math"
	"strings"
	"time"

	argocommon "github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/event_reporter/codefresh"
	"github.com/argoproj/argo-cd/v2/event_reporter/metrics"
	"github.com/argoproj/argo-cd/v2/event_reporter/reporter"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	applisters "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	servercache "github.com/argoproj/argo-cd/v2/server/cache"
	argoutil "github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/env"
	"github.com/argoproj/argo-cd/v2/util/settings"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

var (
	watchAPIBufferSize              = 1000
	applicationEventCacheExpiration = time.Minute * time.Duration(env.ParseNumFromEnv(argocommon.EnvApplicationEventCacheDuration, 20, 0, math.MaxInt32))
)

type EventReporterController interface {
	Run(ctx context.Context)
}

type eventReporterController struct {
	settingsMgr              *settings.SettingsManager
	appBroadcaster           reporter.Broadcaster
	applicationEventReporter reporter.ApplicationEventReporter
	cache                    *servercache.Cache
	appLister                applisters.ApplicationLister
	applicationServiceClient appclient.ApplicationClient
	metricsServer            *metrics.MetricsServer
}

func NewEventReporterController(appInformer cache.SharedIndexInformer, cache *servercache.Cache, settingsMgr *settings.SettingsManager, applicationServiceClient appclient.ApplicationClient, appLister applisters.ApplicationLister, codefreshConfig *codefresh.CodefreshConfig, metricsServer *metrics.MetricsServer, featureManager *reporter.FeatureManager) EventReporterController {
	appBroadcaster := reporter.NewBroadcaster(featureManager, metricsServer)
	appInformer.AddEventHandler(appBroadcaster)
	return &eventReporterController{
		appBroadcaster:           appBroadcaster,
		applicationEventReporter: reporter.NewApplicationEventReporter(cache, applicationServiceClient, appLister, codefreshConfig, metricsServer),
		cache:                    cache,
		settingsMgr:              settingsMgr,
		applicationServiceClient: applicationServiceClient,
		appLister:                appLister,
		metricsServer:            metricsServer,
	}
}

func (c *eventReporterController) Run(ctx context.Context) {
	var (
		logCtx log.FieldLogger = log.StandardLogger()
	)

	// sendIfPermitted is a helper to send the application to the client's streaming channel if the
	// caller has RBAC privileges permissions to view it
	sendIfPermitted := func(ctx context.Context, a appv1.Application, eventType watch.EventType, ts string, ignoreResourceCache bool) error {
		if eventType == watch.Bookmark {
			return nil // ignore this event
		}

		appInstanceLabelKey, err := c.settingsMgr.GetAppInstanceLabelKey()
		if err != nil {
			return err
		}
		trackingMethod := argoutil.GetTrackingMethod(c.settingsMgr)

		err = c.applicationEventReporter.StreamApplicationEvents(ctx, &a, ts, ignoreResourceCache, appInstanceLabelKey, trackingMethod)
		if err != nil {
			return err
		}

		if err := c.cache.SetLastApplicationEvent(&a, applicationEventCacheExpiration); err != nil {
			logCtx.WithError(err).Error("failed to cache last sent application event")
			return err
		}
		return nil
	}

	//TODO: move to abstraction
	eventsChannel := make(chan *appv1.ApplicationWatchEvent, watchAPIBufferSize)
	unsubscribe := c.appBroadcaster.Subscribe(eventsChannel)
	defer unsubscribe()
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-eventsChannel:
			logCtx.Infof("channel size is %d", len(eventsChannel))
			c.metricsServer.SetQueueSizeCounter(len(eventsChannel))
			shouldProcess, ignoreResourceCache := c.applicationEventReporter.ShouldSendApplicationEvent(event)
			if !shouldProcess {
				logCtx.Infof("Skipping event %s/%s", event.Application.Name, event.Type)
				c.metricsServer.IncCachedIgnoredEventsCounter(metrics.MetricAppEventType, event.Application.Name)
				continue
			}
			ts := time.Now().Format("2006-01-02T15:04:05.000Z")
			ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			err := sendIfPermitted(ctx, event.Application, event.Type, ts, ignoreResourceCache)
			if err != nil {
				logCtx.WithError(err).Error("failed to stream application events")
				if strings.Contains(err.Error(), "context deadline exceeded") {
					logCtx.Info("Closing event-source connection")
					cancel()
				}
			}
			cancel()
		}
	}
}
