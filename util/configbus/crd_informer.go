package configbus

import (
	"context"
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application"
	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned"
	appinformer "github.com/argoproj/argo-cd/v3/pkg/client/informers/externalversions"
)

// InformerCRDSource watches the singleton ArgoCDConfiguration and implements CRDSource.
type InformerCRDSource struct {
	mu          sync.RWMutex
	current     *appv1.ArgoCDConfiguration
	synced      cache.InformerSynced
	subscribers []chan<- struct{}
}

// CRDChangeNotifier is optionally implemented by CRDSource backends that can
// push change notifications (e.g. InformerCRDSource).
type CRDChangeNotifier interface {
	Subscribe(subCh chan<- struct{})
	Unsubscribe(subCh chan<- struct{})
}

// NewInformerCRDSource starts a namespaced informer for ArgoCDConfiguration and
// returns a CRDSource backed by its cache. Callers should cancel ctx on shutdown.
// If the CRD is not installed, WaitForCacheSync may fail — callers should treat
// that as a soft error and fall back to nil CRDSource.
func NewInformerCRDSource(ctx context.Context, client appclientset.Interface, namespace string) (*InformerCRDSource, error) {
	if client == nil {
		return nil, fmt.Errorf("config: application clientset is nil")
	}
	// Fail fast when the CRD is not installed. Without this probe, WaitForCacheSync
	// blocks indefinitely while the reflector retries list/watch failures.
	if _, err := client.ArgoprojV1alpha1().ArgoCDConfigurations(namespace).List(ctx, metav1.ListOptions{Limit: 1}); err != nil {
		if meta.IsNoMatchError(err) || apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("config: ArgoCDConfiguration CRD unavailable: %w", err)
		}
		// Other errors (RBAC, connectivity) still attempt the informer path below.
		log.WithError(err).Debug("ArgoCDConfiguration list probe failed; continuing to informer sync")
	}
	src := &InformerCRDSource{}
	factory := appinformer.NewSharedInformerFactoryWithOptions(
		client,
		time.Minute,
		appinformer.WithNamespace(namespace),
	)
	informer := factory.Argoproj().V1alpha1().ArgoCDConfigurations().Informer()

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj any) { src.store(obj) },
		UpdateFunc: func(_, newObj any) { src.store(newObj) },
		DeleteFunc: func(obj any) { src.delete(obj) },
	})
	if err != nil {
		return nil, fmt.Errorf("config: add ArgoCDConfiguration event handler: %w", err)
	}

	factory.Start(ctx.Done())
	src.synced = informer.HasSynced
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return nil, fmt.Errorf("config: timed out waiting for ArgoCDConfiguration informer sync")
	}
	return src, nil
}

// NewOptionalInformerCRDSource starts an ArgoCDConfiguration informer and returns
// it as a CRDSource. On failure (e.g. CRD not installed), logs a warning and
// returns nil so callers keep the legacy-only Provider path.
func NewOptionalInformerCRDSource(ctx context.Context, client appclientset.Interface, namespace string) CRDSource {
	src, err := NewInformerCRDSource(ctx, client, namespace)
	if err != nil {
		log.WithError(err).Warn("ArgoCDConfiguration informer unavailable; continuing without CRD config source")
		return nil
	}
	return src
}

func (s *InformerCRDSource) store(obj any) {
	cfg := asArgoCDConfiguration(obj)
	if cfg == nil || cfg.Name != application.ArgoCDConfigurationName {
		return
	}
	s.mu.Lock()
	s.current = cfg.DeepCopy()
	s.mu.Unlock()
	s.notifySubscribers()
}

func (s *InformerCRDSource) delete(obj any) {
	cfg := asArgoCDConfiguration(obj)
	if cfg == nil || cfg.Name != application.ArgoCDConfigurationName {
		return
	}
	s.mu.Lock()
	s.current = nil
	s.mu.Unlock()
	s.notifySubscribers()
}

func (s *InformerCRDSource) get() *appv1.ArgoCDConfiguration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current
}

// Subscribe registers a channel for ArgoCDConfiguration add/update/delete events.
func (s *InformerCRDSource) Subscribe(subCh chan<- struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscribers = append(s.subscribers, subCh)
	log.Infof("%v subscribed to ArgoCDConfiguration updates", subCh)
}

// Unsubscribe unregisters a channel from ArgoCDConfiguration updates.
func (s *InformerCRDSource) Unsubscribe(subCh chan<- struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, ch := range s.subscribers {
		if ch == subCh {
			s.subscribers = append(s.subscribers[:i], s.subscribers[i+1:]...)
			log.Infof("%v unsubscribed from ArgoCDConfiguration updates", subCh)
			return
		}
	}
}

func (s *InformerCRDSource) notifySubscribers() {
	s.mu.RLock()
	if len(s.subscribers) == 0 {
		s.mu.RUnlock()
		return
	}
	subscribers := make([]chan<- struct{}, len(s.subscribers))
	copy(subscribers, s.subscribers)
	s.mu.RUnlock()
	go func() {
		log.Infof("Notifying %d ArgoCDConfiguration subscribers: %v", len(subscribers), subscribers)
		for _, sub := range subscribers {
			select {
			case sub <- struct{}{}:
			default:
				// Drop if subscriber is slow; next event or settings change will retry.
			}
		}
	}()
}

func asArgoCDConfiguration(obj any) *appv1.ArgoCDConfiguration {
	if cfg, ok := obj.(*appv1.ArgoCDConfiguration); ok {
		return cfg
	}
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		if cfg, ok := tombstone.Obj.(*appv1.ArgoCDConfiguration); ok {
			return cfg
		}
	}
	return nil
}

// StaticCRDSource is a test/helper CRDSource backed by a fixed object (or nil).
type StaticCRDSource struct {
	Object *appv1.ArgoCDConfiguration
}

func (s StaticCRDSource) get() *appv1.ArgoCDConfiguration {
	return s.Object
}
