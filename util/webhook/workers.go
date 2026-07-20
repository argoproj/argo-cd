package webhook

import (
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/util/guard"
)

// DefaultPayloadQueueSize limits the number of webhook payloads waiting for processing.
const DefaultPayloadQueueSize = 50000

// StartWorkerPool starts webhook event workers with consistent panic recovery.
func StartWorkerPool(waitGroup *sync.WaitGroup, queue <-chan any, parallelism int, component, panicMessage string, handleEvent func(any)) {
	componentLog := log.WithField("component", component)
	for range parallelism {
		waitGroup.Go(func() {
			for payload := range queue {
				guard.RecoverAndLog(func() { handleEvent(payload) }, componentLog, panicMessage)
			}
		})
	}
}
