package applicationset

import (
	"sync"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/watch"

	appsetv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

type Brodcaster interface {
	Subscribe(ch chan *appsetv1.ApplicationSetWatchEvent, filters ...func(event *appsetv1.ApplicationSetWatchEvent) bool) func()
	OnAdd(any, bool)
	OnUpdate(any, any)
	OnDelete(any)
}

type subscriber struct {
	ch      chan *appsetv1.ApplicationSetWatchEvent
	filters []func(*appsetv1.ApplicationSetWatchEvent) bool
}

type broadcasterHandler struct {
	lock        sync.Mutex
	subscribers []*subscriber
}

func (b *broadcasterHandler) notify(event *appsetv1.ApplicationSetWatchEvent) {
	log.Infof("broadcaster: received event: %s for %s", event.Type, event.ApplicationSet.Name)

	// Copy subscribers before unlocking
	subscribers := []*subscriber{}
	b.lock.Lock()
	subscribers = append(subscribers, b.subscribers...)
	b.lock.Unlock()

	for _, s := range subscribers {
		if s.matches(event) {
			log.Infof("broadcaster: notifying subscriber for %s", event.ApplicationSet.Name)
			select {
			case s.ch <- event:
				log.Infof("broadcaster: successfully sent event for %s", event.ApplicationSet.Name)
			default:
				log.Warnf("broadcaster: failed to send event (channel full?) for %s", event.ApplicationSet.Name)
			}
		}
	}
}

func (s *subscriber) matches(event *appsetv1.ApplicationSetWatchEvent) bool {
	for i := range s.filters {
		if !s.filters[i](event) {
			return false
		}
	}
	return true
}

// Subscribe forward applicationset informer watch events to the provided channel.
// The watch events are dropped if no receives are reading events from the channel so the channel must have
// buffer if dropping events is not acceptable.
func (b *broadcasterHandler) Subscribe(ch chan *appsetv1.ApplicationSetWatchEvent, filters ...func(event *appsetv1.ApplicationSetWatchEvent) bool) func() {
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

func (b *broadcasterHandler) OnAdd(obj any, _ bool) {
	if appset, ok := obj.(*appsetv1.ApplicationSet); ok {
		b.notify(&appsetv1.ApplicationSetWatchEvent{
			ApplicationSet: *appset,
			Type:           watch.Added,
		})
	}
}

func (b *broadcasterHandler) OnUpdate(_, newObj any) {
	if appset, ok := newObj.(*appsetv1.ApplicationSet); ok {
		b.notify(&appsetv1.ApplicationSetWatchEvent{ApplicationSet: *appset, Type: watch.Modified})
	}
}

func (b *broadcasterHandler) OnDelete(obj any) {
	if appset, ok := obj.(*appsetv1.ApplicationSet); ok {
		b.notify(&appsetv1.ApplicationSetWatchEvent{ApplicationSet: *appset, Type: watch.Deleted})
	}
}
