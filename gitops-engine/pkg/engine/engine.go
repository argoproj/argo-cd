/*
The package provides high-level interface that leverages "pkg/cache", "pkg/sync", "pkg/health" and "pkg/diff" packages
and "implements" GitOps.

Example

The https://github.com/argoproj/gitops-engine/tree/master/agent demonstrates how to use the engine.
*/

package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/diff"
	"github.com/argoproj/gitops-engine/pkg/sync"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
)

const (
	operationRefreshTimeout = time.Second * 1
)

type StopFunc func()

type GitOpsEngine interface {
	// Run initializes engine
	Run() (StopFunc, error)
	// Synchronizes resources in the cluster
	Sync(ctx context.Context, resources []*unstructured.Unstructured, isManaged func(r *cache.Resource) bool, revision string, namespace string, opts ...sync.SyncOpt) ([]common.ResourceSyncResult, error)
}

type gitOpsEngine struct {
	config  *rest.Config
	cache   cache.ClusterCache
	kubectl kube.Kubectl
	log     logr.Logger
}

// NewEngine creates new instances of the GitOps engine
func NewEngine(config *rest.Config, clusterCache cache.ClusterCache, opts ...Option) GitOpsEngine {
	o := applyOptions(opts)
	return &gitOpsEngine{
		config:  config,
		cache:   clusterCache,
		kubectl: o.kubectl,
		log:     o.log,
	}
}

func (e *gitOpsEngine) Run() (StopFunc, error) {
	err := e.cache.EnsureSynced()
	if err != nil {
		return nil, err
	}

	return func() {
		e.cache.Invalidate()
	}, nil
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
	diffRes, err := diff.DiffArray(result.Target, result.Live, diff.WithLogr(e.log))
	if err != nil {
		return nil, err
	}
	opts = append(opts, sync.WithSkipHooks(!diffRes.Modified))
	syncCtx, cleanup, err := sync.NewSyncContext(revision, result, e.config, e.config, e.kubectl, namespace, e.cache.GetOpenAPISchema(), opts...)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	resUpdated := make(chan bool)
	resIgnore := make(chan struct{})
	unsubscribe := e.cache.OnResourceUpdated(func(newRes *cache.Resource, oldRes *cache.Resource, _ map[kube.ResourceKey]*cache.Resource) {
		var key kube.ResourceKey
		if newRes != nil {
			key = newRes.ResourceKey()
		} else {
			key = oldRes.ResourceKey()
		}
		if _, ok := managedResources[key]; ok {
			select {
			case resUpdated <- true:
			case <-resIgnore:
			}
		}
	})
	defer close(resIgnore)
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
			return resources, ctx.Err()
		case <-time.After(operationRefreshTimeout):
		case <-resUpdated:
		}
	}
}
