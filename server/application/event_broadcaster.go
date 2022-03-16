package application

import (
	v1 "k8s.io/api/core/v1"
	"sync"

	"k8s.io/apimachinery/pkg/watch"

	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

type event_subscriber struct {
	ch      chan *appv1.ResourceEventWatchEvent
	filters []func(*appv1.ResourceEventWatchEvent) bool
}

func (s *event_subscriber) matches(event *appv1.ResourceEventWatchEvent) bool {
	for i := range s.filters {
		if !s.filters[i](event) {
			return false
		}
	}
	return true
}

type eventBroadcasterHandler struct {
	lock        sync.Mutex
	subscribers []*event_subscriber
}

func (b *eventBroadcasterHandler) notify(event *appv1.ResourceEventWatchEvent) {
	// Make a local copy of b.subscribers, then send channel events outside the lock,
	// to avoid data race on b.subscribers changes
	subscribers := []*event_subscriber{}
	b.lock.Lock()
	subscribers = append(subscribers, b.subscribers...)
	b.lock.Unlock()

	for _, s := range subscribers {
		if s.matches(event) {
			select {
			case s.ch <- event:
			default:
				// drop event if cannot send right away
				//log.WithField("application", event.Resource).Warn("unable to send event notification")
			}
		}
	}
}

// Subscribe forward application informer watch events to the provided channel.
// The watch events are dropped if no receives are reading events from the channel so the channel must have
// buffer if dropping events is not acceptable.
func (b *eventBroadcasterHandler) Subscribe(ch chan *appv1.ResourceEventWatchEvent, filters ...func(event *appv1.ResourceEventWatchEvent) bool) func() {
	b.lock.Lock()
	defer b.lock.Unlock()
	subscriber := &event_subscriber{ch, filters}
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

func (b *eventBroadcasterHandler) OnAdd(obj interface{}) {
	if event, ok := obj.(*v1.Event); ok {
		b.notify(&appv1.ResourceEventWatchEvent{Event: *event, Type: watch.Added})
	}
}

func (b *eventBroadcasterHandler) OnUpdate(_, newObj interface{}) {
	if event, ok := newObj.(*v1.Event); ok {
		b.notify(&appv1.ResourceEventWatchEvent{Event: *event, Type: watch.Modified})
	}
}

func (b *eventBroadcasterHandler) OnDelete(obj interface{}) {
	if event, ok := obj.(*v1.Event); ok {
		b.notify(&appv1.ResourceEventWatchEvent{Event: *event, Type: watch.Deleted})
	}
}
