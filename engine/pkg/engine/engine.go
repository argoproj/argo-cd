package engine

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"

	ioutil "github.com/argoproj/argo-cd/engine/pkg/utils/io"
	"github.com/argoproj/argo-cd/engine/pkg/utils/kube"
	"github.com/argoproj/argo-cd/engine/pkg/utils/kube/cache"
	"github.com/argoproj/argo-cd/engine/pkg/utils/kube/sync"
	"github.com/argoproj/argo-cd/engine/pkg/utils/kube/sync/common"
)

const (
	operationRefreshTimeout = time.Second * 1
)

type GitOpsEngine interface {
	Run() (io.Closer, error)
	Sync(ctx context.Context, resources []*unstructured.Unstructured, isManaged func(r *cache.Resource) bool, revision string, namespace string, opts ...sync.SyncOpt) ([]common.ResourceSyncResult, error)
}

type gitOpsEngine struct {
	config  *rest.Config
	cache   cache.ClusterCache
	kubectl kube.Kubectl
}

func NewEngine(config *rest.Config, clusterCache cache.ClusterCache) GitOpsEngine {
	return &gitOpsEngine{
		config:  config,
		kubectl: &kube.KubectlCmd{},
		cache:   clusterCache,
	}
}

func (e *gitOpsEngine) Run() (io.Closer, error) {
	err := e.cache.EnsureSynced()
	if err != nil {
		return nil, err
	}

	return ioutil.NewCloser(func() error {
		e.cache.Invalidate(func(config *rest.Config, ns []string, settings cache.Settings) (*rest.Config, []string, cache.Settings) {
			return config, ns, settings
		})
		return nil
	}), nil
}

func (e *gitOpsEngine) Sync(ctx context.Context,
	resources []*unstructured.Unstructured,
	isManaged func(r *cache.Resource) bool,
	revision string,
	namespace string,
	opts ...sync.SyncOpt,
) ([]common.ResourceSyncResult, error) {
	managedResources, err := e.cache.GetManagedLiveObjs(resources, isManaged)
	if err != nil {
		return nil, err
	}
	result := sync.Reconcile(resources, managedResources, namespace, e.cache)
	syncCtx, err := sync.NewSyncContext(revision, result, e.config, e.kubectl, namespace, log.NewEntry(log.New()), opts...)
	if err != nil {
		return nil, err
	}

	resUpdated := make(chan bool)
	unsubscribe := e.cache.OnResourceUpdated(func(newRes *cache.Resource, oldRes *cache.Resource, namespaceResources map[kube.ResourceKey]*cache.Resource) {
		var key kube.ResourceKey
		if newRes != nil {
			key = newRes.ResourceKey()
		} else {
			key = oldRes.ResourceKey()
		}
		if _, ok := managedResources[key]; ok {
			resUpdated <- true
		}
	})
	defer unsubscribe()
	for {
		syncCtx.Sync()
		phase, message, resources := syncCtx.GetState()
		if phase.Completed() {
			if phase == common.OperationError {
				err = fmt.Errorf("sync operation failed: %s", message)
			}
			return resources, err
		}
		select {
		case <-ctx.Done():
			syncCtx.Terminate()
			return resources, errors.New("sync operation was terminated")
		case <-time.After(operationRefreshTimeout):
		case <-resUpdated:
		}
	}
}
