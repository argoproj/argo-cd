package broadcast

import (
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/watch"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestBroadcasterHandler_SubscribeUnsubscribe(t *testing.T) {
	t.Parallel()
	broadcaster := NewHandler[appv1.Application, appv1.ApplicationWatchEvent](
		func(app *appv1.Application, eventType watch.EventType) *appv1.ApplicationWatchEvent {
			return &appv1.ApplicationWatchEvent{Application: *app, Type: eventType}
		},
		func(app *appv1.Application) log.Fields {
			return log.Fields{"application": app.Name}
		},
	)

	subscriber := make(chan *appv1.ApplicationWatchEvent)
	unsubscribe := broadcaster.Subscribe(subscriber)

	assert.Len(t, broadcaster.subscribers, 1)

	unsubscribe()
	assert.Empty(t, broadcaster.subscribers)
}

func TestBroadcasterHandler_ReceiveEvents(t *testing.T) {
	t.Parallel()
	broadcaster := NewHandler[appv1.Application, appv1.ApplicationWatchEvent](
		func(app *appv1.Application, eventType watch.EventType) *appv1.ApplicationWatchEvent {
			return &appv1.ApplicationWatchEvent{Application: *app, Type: eventType}
		},
		func(app *appv1.Application) log.Fields {
			return log.Fields{"application": app.Name}
		},
	)

	subscriber1 := make(chan *appv1.ApplicationWatchEvent, 1000)
	subscriber2 := make(chan *appv1.ApplicationWatchEvent, 1000)

	_ = broadcaster.Subscribe(subscriber1)
	_ = broadcaster.Subscribe(subscriber2)

	firstReceived := false
	secondReceived := false

	go broadcaster.OnAdd(&appv1.Application{}, false)

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

func TestBroadcasterHandler_OnUpdatePassesOldObject(t *testing.T) {
	t.Parallel()
	type event struct {
		app    appv1.Application
		old    *appv1.Application
		update bool
	}
	broadcaster := NewHandler[appv1.Application, event](
		func(app *appv1.Application, _ watch.EventType) *event {
			return &event{app: *app}
		},
		func(app *appv1.Application) log.Fields {
			return log.Fields{"application": app.Name}
		},
	).WithUpdateEventFunc(func(oldApp, newApp *appv1.Application) *event {
		return &event{app: *newApp, old: oldApp, update: true}
	})

	ch := make(chan *event, 1)
	_ = broadcaster.Subscribe(ch)

	oldApp := &appv1.Application{Spec: appv1.ApplicationSpec{Project: "a"}}
	newApp := &appv1.Application{Spec: appv1.ApplicationSpec{Project: "b"}}
	broadcaster.OnUpdate(oldApp, newApp)

	got := <-ch
	assert.True(t, got.update)
	assert.Equal(t, "a", got.old.Spec.Project)
	assert.Equal(t, "b", got.app.Spec.Project)

	// Without an update function, OnUpdate keeps using the regular event function.
	plain := NewHandler[appv1.Application, event](
		func(app *appv1.Application, _ watch.EventType) *event { return &event{app: *app} },
		func(app *appv1.Application) log.Fields { return log.Fields{"application": app.Name} },
	)
	ch2 := make(chan *event, 1)
	_ = plain.Subscribe(ch2)
	plain.OnUpdate(oldApp, newApp)
	got = <-ch2
	assert.False(t, got.update)
	assert.Equal(t, "b", got.app.Spec.Project)
}
