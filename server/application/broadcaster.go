package application

import (
	"sync"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/watch"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

type subscriber struct {
	ch      chan *appv1.ApplicationWatchEvent
	filters []func(*appv1.ApplicationWatchEvent) bool
}

func (s *subscriber) matches(event *appv1.ApplicationWatchEvent) bool {
	for i := range s.filters {
		if !s.filters[i](event) {
			return false
		}
	}
	return true
}

type broadcasterHandler struct {
	lock        sync.Mutex
	subscribers []*subscriber
}

func (b *broadcasterHandler) notify(event *appv1.ApplicationWatchEvent) {
	subscribers := b.subscribers
	for _, s := range subscribers {
		if s.matches(event) {
			select {
			case s.ch <- event:
			default:
				// drop event if cannot send right away
				log.WithField("application", event.Application.Name).Warn("unable to send event notification")
			}
		}
	}
}

func (b *broadcasterHandler) Subscribe(ch chan *appv1.ApplicationWatchEvent, filters ...func(event *appv1.ApplicationWatchEvent) bool) func() {
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
	if app, ok := obj.(*appv1.Application); ok {
		b.notify(&appv1.ApplicationWatchEvent{Application: *app, Type: watch.Added})
	}
}

func (b *broadcasterHandler) OnUpdate(_, newObj interface{}) {
	if app, ok := newObj.(*appv1.Application); ok {
		b.notify(&appv1.ApplicationWatchEvent{Application: *app, Type: watch.Modified})
	}
}

func (b *broadcasterHandler) OnDelete(obj interface{}) {
	if app, ok := obj.(*appv1.Application); ok {
		b.notify(&appv1.ApplicationWatchEvent{Application: *app, Type: watch.Deleted})
	}
}
