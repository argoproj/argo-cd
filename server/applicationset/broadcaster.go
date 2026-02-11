package applicationset

import (
	"sync"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/watch"

	appsetv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	applog "github.com/argoproj/argo-cd/v3/util/app/log"
)

type subscriber struct {
	ch      chan *appsetv1.ApplicationSetWatchEvent
	filters []func(event *appsetv1.ApplicationSetWatchEvent) bool
}

func (s *subscriber) matches(event *appsetv1.ApplicationSetWatchEvent) bool {
	for i := range s.filters {
		if !s.filters[i](event) {
			return false
		}
	}

	return true
}

// Broadcaster is an interface for broadcasting applicationset informer watch events to multiple subscribers.
type Broadcaster interface {
	Subscribe(ch chan *appsetv1.ApplicationSetWatchEvent, filters ...func(event *appsetv1.ApplicationSetWatchEvent) bool) func()
	OnAdd(any, bool)
	OnUpdate(any, any)
	OnDelete(any)
}

type BroadcasterHandler struct {
	lock        sync.Mutex
	subscribers []*subscriber
}

func (b *BroadcasterHandler) notify(event *appsetv1.ApplicationSetWatchEvent) {
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
				log.WithFields(applog.GetAppSetLogFields(&event.ApplicationSet)).Warn("unable to send event notification")
			}
		}
	}
}

// Subscribe forward application informer watch events to the provided channel.
// The watch events are dropped if no receives are reading events from the channel so the channel must have
// buffer if dropping events is not acceptable.
func (b *BroadcasterHandler) Subscribe(ch chan *appsetv1.ApplicationSetWatchEvent, filters ...func(event *appsetv1.ApplicationSetWatchEvent) bool) func() {
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

func (b *BroadcasterHandler) OnAdd(obj any, _ bool) {
	if appset, ok := obj.(*appsetv1.ApplicationSet); ok {
		b.notify(&appsetv1.ApplicationSetWatchEvent{ApplicationSet: *appset, Type: watch.Added})
	}
}

func (b *BroadcasterHandler) OnUpdate(_, newObj any) {
	if appset, ok := newObj.(*appsetv1.ApplicationSet); ok {
		b.notify(&appsetv1.ApplicationSetWatchEvent{
			ApplicationSet: *appset,
			Type:           watch.Modified,
		},
		)
	}
}

func (b *BroadcasterHandler) OnDelete(obj any) {
	if appset, ok := obj.(*appsetv1.ApplicationSet); ok {
		b.notify(&appsetv1.ApplicationSetWatchEvent{ApplicationSet: *appset, Type: watch.Deleted})
	}
}
