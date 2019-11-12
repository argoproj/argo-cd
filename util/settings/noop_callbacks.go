package settings

import (
	"github.com/argoproj/argo-cd/engine/pkg"
	"github.com/argoproj/argo-cd/engine/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/engine/util/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type noop_callbacks struct {
}

func (n noop_callbacks) OnClusterInitialized(server string) {
}

func (n noop_callbacks) OnResourceUpdated(un *unstructured.Unstructured) {
}

func (n noop_callbacks) OnResourceRemoved(key kube.ResourceKey) {

}

func (n noop_callbacks) OnBeforeSync(appName string, tasks []pkg.SyncTaskInfo) ([]pkg.SyncTaskInfo, error) {
	return tasks, nil
}

func (n noop_callbacks) OnSyncCompleted(appName string, state v1alpha1.OperationState) error {
	return nil
}

func NewNoOpCallbacks() pkg.Callbacks {
	return &noop_callbacks{}
}
