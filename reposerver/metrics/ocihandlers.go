package metrics

import (
	"time"

	"github.com/argoproj/argo-cd/v3/util/oci"
	log "github.com/sirupsen/logrus"
)

type OCIRequestType string

const (
	OCIRequestTypeExtract         = "extract"
	OCIRequestTypeResolveRevision = "resolve-revision"
	OCIRequestTypeDigestMetadata  = "digest-metadata"
	OCIRequestTypeGetTags         = "get-tags"
	OCIRequestTypeTestRepo        = "test-repo"
)

// NewOCIClientEventHandlers creates event handlers to update OCI repo, related metrics
func NewOCIClientEventHandlers(metricsServer *MetricsServer) oci.EventHandlers {
	return oci.EventHandlers{
		OnExtract: func(repo string) func() {
			return processMetricFunc(metricsServer, repo, OCIRequestTypeExtract)
		},
		OnResolveRevision: func(repo string) func() {
			return processMetricFunc(metricsServer, repo, OCIRequestTypeResolveRevision)
		},
		OnDigestMetadata: func(repo string) func() {
			return processMetricFunc(metricsServer, repo, OCIRequestTypeDigestMetadata)
		},
		OnGetTags: func(repo string) func() {
			return processMetricFunc(metricsServer, repo, OCIRequestTypeGetTags)
		},
		OnTestRepo: func(repo string) func() {
			return processMetricFunc(metricsServer, repo, OCIRequestTypeTestRepo)
		},
	}
}

func processMetricFunc(metricsServer *MetricsServer, repo string, requestType OCIRequestType) func() {
	log.Warnf("processing metric %s for repo %s", requestType, repo)
	startTime := time.Now()
	metricsServer.IncOCIRequest(repo, requestType)
	return func() {
		metricsServer.ObserveOCIRequestDuration(repo, requestType, time.Since(startTime))
	}
}
