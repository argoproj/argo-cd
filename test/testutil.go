package test

import (
	"context"
	"log"

	"k8s.io/client-go/tools/cache"
)

// StartInformer is a helper to start an informer, wait for its cache to sync and return a cancel func
func StartInformer(informer cache.SharedIndexInformer) context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())
	go informer.Run(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		log.Fatal("Timed out waiting for informer cache to sync")
	}
	return cancel
}
