package application

import (
	"sync"

	"k8s.io/apimachinery/pkg/watch"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

type broadcasterHandler struct {
	lock        sync.Mutex
	subscribers []chan *appv1.ApplicationWatchEvent
}

func (b *broadcasterHandler) notify(event *appv1.ApplicationWatchEvent) {
	subscribers := b.subscribers
	for i := range subscribers {
		s := subscribers[i]
		go func() {
			s <- event
		}()
	}
}

func (b *broadcasterHandler) Subscribe(subscriber chan *appv1.ApplicationWatchEvent) func() {
	b.lock.Lock()
	defer b.lock.Unlock()
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
