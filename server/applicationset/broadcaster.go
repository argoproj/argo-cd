package applicationset

import (
	"sync"

	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/watch"
)

type subscriber struct {
	ch      chan *appv1.ApplicationSetWatchEvent
	filters []func(*appv1.ApplicationSetWatchEvent) bool
}

func (s *subscriber) matches(event *appv1.ApplicationSetWatchEvent) bool {
	for i := range s.filters {
		if !s.filters[i](event) {
			return false
		}
	}
	return true
}

// Broadcaster is an interface for broadcasting application informer watch events to multiple subscribers.
type Broadcaster interface {
	Subscribe(ch chan *appv1.ApplicationSetWatchEvent, filters ...func(event *appv1.ApplicationSetWatchEvent) bool) func()
	OnAdd(interface{})
	OnUpdate(interface{}, interface{})
	OnDelete(interface{})
}

type broadcasterHandler struct {
	lock        sync.Mutex
	subscribers []*subscriber
}

func (b *broadcasterHandler) notify(event *appv1.ApplicationSetWatchEvent) {
	// Make a local copy of b.subscribers, then send channel events outside the lock,
	// to avoid data race on b.subscribers changes
	subscribers := []*subscriber{}
	b.lock.Lock()
	subscribers = append(subscribers, b.subscribers...)
	b.lock.Unlock()

	for _, s := range subscribers {
		if s.matches(event) {
			select {
			case s.ch <- event:
			default:
				// drop event if cannot send right away
				log.WithField("applicationset", event.ApplicationSet.Name).Warn("unable to send event notification")
			}
		}
	}
}

// Subscribe forward application informer watch events to the provided channel.
// The watch events are dropped if no receives are reading events from the channel so the channel must have
// buffer if dropping events is not acceptable.
func (b *broadcasterHandler) Subscribe(ch chan *appv1.ApplicationSetWatchEvent, filters ...func(event *appv1.ApplicationSetWatchEvent) bool) func() {
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
	if app, ok := obj.(*appv1.ApplicationSet); ok {
		b.notify(&appv1.ApplicationSetWatchEvent{ApplicationSet: *app, Type: watch.Added})
	}
}

func (b *broadcasterHandler) OnUpdate(_, newObj interface{}) {
	if app, ok := newObj.(*appv1.ApplicationSet); ok {
		b.notify(&appv1.ApplicationSetWatchEvent{ApplicationSet: *app, Type: watch.Modified})
	}
}

func (b *broadcasterHandler) OnDelete(obj interface{}) {
	if app, ok := obj.(*appv1.ApplicationSet); ok {
		b.notify(&appv1.ApplicationSetWatchEvent{ApplicationSet: *app, Type: watch.Deleted})
	}
}
