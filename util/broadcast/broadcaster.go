package broadcast

import (
	"sync"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/watch"
)

type EventFunc[T any, E any] func(content *T, eventType watch.EventType) *E

type subscriber[T any] struct {
	ch      chan *T
	filters []func(*T) bool
}

func (s *subscriber[T]) matches(event *T) bool {
	for i := range s.filters {
		if !s.filters[i](event) {
			return false
		}
	}
	return true
}

// Broadcaster is an interface for broadcasting application informer watch events to multiple subscribers.
type Broadcaster[T any] interface {
	Subscribe(ch chan *T, filters ...func(event *T) bool) func()
	OnAdd(interface{})
	OnUpdate(interface{}, interface{})
	OnDelete(interface{})
}

type broadcasterHandler[T any, E any] struct {
	lock        sync.Mutex
	subscribers []*subscriber[E]
	eventFunc   EventFunc[T, E]
}

func (b *broadcasterHandler[T, E]) notify(event *E) {
	// Make a local copy of b.subscribers, then send channel events outside the lock,
	// to avoid data race on b.subscribers changes
	subscribers := []*subscriber[E]{}
	b.lock.Lock()
	subscribers = append(subscribers, b.subscribers...)
	b.lock.Unlock()

	for _, s := range subscribers {
		if s.matches(event) {
			select {
			case s.ch <- event:
			default:
				// drop event if cannot send right away
				log.Warn("unable to send event notification")
			}
		}
	}
}

// Subscribe forward application informer watch events to the provided channel.
// The watch events are dropped if no receives are reading events from the channel so the channel must have
// buffer if dropping events is not acceptable.
func (b *broadcasterHandler[T, E]) Subscribe(ch chan *E, filters ...func(event *E) bool) func() {
	b.lock.Lock()
	defer b.lock.Unlock()
	subscriber := &subscriber[E]{ch, filters}
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

func (b *broadcasterHandler[T, E]) OnAdd(obj interface{}) {
	if app, ok := obj.(*T); ok {
		b.notify(b.eventFunc(app, watch.Added))
	}
}

func (b *broadcasterHandler[T, E]) OnUpdate(_, newObj interface{}) {
	if app, ok := newObj.(*T); ok {
		b.notify(b.eventFunc(app, watch.Modified))
	}
}

func (b *broadcasterHandler[T, E]) OnDelete(obj interface{}) {
	if app, ok := obj.(*T); ok {
		b.notify(b.eventFunc(app, watch.Deleted))
	}
}

func NewHandler[T any, E any](eventFunc EventFunc[T, E]) *broadcasterHandler[T, E] {
	return &broadcasterHandler[T, E]{
		eventFunc: eventFunc,
	}
}
