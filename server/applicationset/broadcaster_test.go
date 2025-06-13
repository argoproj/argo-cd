package applicationset

import (
	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestBroadcasterHandler_SubscribeUnsubscribe(t *testing.T) {
	broadcaster := broadcasterHandler{}

	subscriber := make(chan *appv1.ApplicationSetWatchEvent)
	unsubscribe := broadcaster.Subscribe(subscriber)

	assert.Len(t, broadcaster.subscribers, 1)

	unsubscribe()
	assert.Empty(t, broadcaster.subscribers)
}

func TestBroadcasterHandler_ReceiveEvents(t *testing.T) {
	broadcaster := broadcasterHandler{}

	subscriber1 := make(chan *appv1.ApplicationSetWatchEvent, 1000)
	subscriber2 := make(chan *appv1.ApplicationSetWatchEvent, 1000)

	_ = broadcaster.Subscribe(subscriber1)
	_ = broadcaster.Subscribe(subscriber2)

	firstReceived := false
	secondReceived := false

	go broadcaster.notify(&appv1.ApplicationSetWatchEvent{})

	for {
		select {
		case <-time.After(1 * time.Second):
			t.Error("timeout expired")
			return
		case <-subscriber1:
			firstReceived = true
		case <-subscriber2:
			secondReceived = true
		}
		if firstReceived && secondReceived {
			return
		}
	}
}
