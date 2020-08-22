/*
The package provides high-level interface that leverages "pkg/cache", "pkg/sync", "pkg/health" and "pkg/diff" packages
and "implements" GitOps.

Example

The https://github.com/argoproj/gitops-engine/tree/master/agent demonstrates how to use the engine.
*/

package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/diff"
	"github.com/argoproj/gitops-engine/pkg/sync"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
	ioutil "github.com/argoproj/gitops-engine/pkg/utils/io"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
)

const (
	operationRefreshTimeout = time.Second * 1
)

type GitOpsEngine interface {
	// Run initializes engine
	Run() (io.Closer, error)
	// Synchronizes resources in the cluster
	Sync(ctx context.Context, resources []*unstructured.Unstructured, isManaged func(r *cache.Resource) bool, revision string, namespace string, opts ...sync.SyncOpt) ([]common.ResourceSyncResult, error)
}

type gitOpsEngine struct {
	config  *rest.Config
	cache   cache.ClusterCache
	kubectl kube.Kubectl
}

// NewEngine creates new instances of the GitOps engine
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
		e.cache.Invalidate()
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
	diffRes, err := diff.DiffArray(result.Target, result.Live, diff.GetNoopNormalizer(), diff.GetDefaultDiffOptions())
	if err != nil {
		return nil, err
	}
	opts = append(opts, sync.WithSkipHooks(!diffRes.Modified))
	syncCtx, err := sync.NewSyncContext(revision, result, e.config, e.config, e.kubectl, namespace, log.NewEntry(log.New()), opts...)
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
