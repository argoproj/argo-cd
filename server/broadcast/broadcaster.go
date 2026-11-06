package broadcast

import (
	"sync"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/watch"
)

// EventFunc creates a watch event from an object and event type.
// T is the resource type (e.g., Application), E is the event type (e.g., ApplicationWatchEvent).
type EventFunc[T any, E any] func(obj *T, eventType watch.EventType) *E

// LogFieldsFunc returns log fields for an object (for logging dropped events)
type LogFieldsFunc[T any] func(obj *T) log.Fields

type subscriber[E any] struct {
	ch      chan *E
	filters []func(event *E) bool
}

func (s *subscriber[E]) matches(event *E) bool {
	for i := range s.filters {
		if !s.filters[i](event) {
			return false
		}
	}
	return true
}

// Broadcaster is an interface for broadcasting informer watch events to multiple subscribers.
// T is the resource type (e.g., Application), E is the event type (e.g., ApplicationWatchEvent).
type Broadcaster[E any] interface {
	Subscribe(ch chan *E, filters ...func(event *E) bool) func()
	OnAdd(any, bool)
	OnUpdate(any, any)
	OnDelete(any)
}

// Handler is a generic broadcaster handler that can be used for any resource type.
// T is the resource type (e.g., Application), E is the event type (e.g., ApplicationWatchEvent).
type Handler[T any, E any] struct {
	lock        sync.Mutex
	subscribers []*subscriber[E]
	eventFunc   EventFunc[T, E]
	logFields   LogFieldsFunc[T]
}

// NewHandler creates a new generic broadcaster handler.
// T is the resource type (e.g., Application), E is the event type (e.g., ApplicationWatchEvent).
func NewHandler[T any, E any](eventFunc EventFunc[T, E], logFields LogFieldsFunc[T]) *Handler[T, E] {
	return &Handler[T, E]{
		eventFunc: eventFunc,
		logFields: logFields,
	}
}

func (b *Handler[T, E]) notify(event *E, obj *T) {
	// Make a local copy of b.subscribers, then send channel events outside the lock,
	// to avoid data race on b.subscribers changes
	var subscribers []*subscriber[E]
	b.lock.Lock()
	subscribers = append(subscribers, b.subscribers...)
	b.lock.Unlock()

	for _, s := range subscribers {
		if s.matches(event) {
			select {
			case s.ch <- event:
			default:
				// drop event if cannot send right away
				log.WithFields(b.logFields(obj)).Warn("unable to send event notification")
			}
		}
	}
}

// Subscribe forwards informer watch events to the provided channel.
// The watch events are dropped if no receivers are reading events from the channel so the channel must have
// buffer if dropping events is not acceptable.
func (b *Handler[T, E]) Subscribe(ch chan *E, filters ...func(event *E) bool) func() {
	b.lock.Lock()
	defer b.lock.Unlock()
	sub := &subscriber[E]{ch, filters}
	b.subscribers = append(b.subscribers, sub)
	return func() {
		b.lock.Lock()
		defer b.lock.Unlock()
		for i := range b.subscribers {
			if b.subscribers[i] == sub {
				b.subscribers = append(b.subscribers[:i], b.subscribers[i+1:]...)
				break
			}
		}
	}
}

func (b *Handler[T, E]) OnAdd(obj any, _ bool) {
	if typedObj, ok := obj.(*T); ok {
		event := b.eventFunc(typedObj, watch.Added)
		b.notify(event, typedObj)
	}
}

func (b *Handler[T, E]) OnUpdate(_, newObj any) {
	if typedObj, ok := newObj.(*T); ok {
		event := b.eventFunc(typedObj, watch.Modified)
		b.notify(event, typedObj)
	}
}

func (b *Handler[T, E]) OnDelete(obj any) {
	if typedObj, ok := obj.(*T); ok {
		event := b.eventFunc(typedObj, watch.Deleted)
		b.notify(event, typedObj)
	}
}
