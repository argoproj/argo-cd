package kubetest

import (
	"context"
	"sync"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
)

type MockResourceOps struct {
	Commands      map[string]KubectlOutput
	Events        chan watch.Event
	DynamicClient dynamic.Interface

	lastCommandPerResource map[kube.ResourceKey]string
	lastValidate           bool
	serverSideApply        bool
	serverSideApplyManager string
	lastForce              bool

	recordLock sync.RWMutex

	getResourceFunc *func(ctx context.Context, config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string) (*unstructured.Unstructured, error)
}

// WithGetResourceFunc overrides the default ConvertToVersion behavior.
func (r *MockResourceOps) WithGetResourceFunc(getResourcefunc func(context.Context, *rest.Config, schema.GroupVersionKind, string, string) (*unstructured.Unstructured, error)) *MockResourceOps {
	r.getResourceFunc = &getResourcefunc
	return r
}

func (r *MockResourceOps) SetLastValidate(validate bool) {
	r.recordLock.Lock()
	r.lastValidate = validate
	r.recordLock.Unlock()
}

func (r *MockResourceOps) GetLastValidate() bool {
	r.recordLock.RLock()
	validate := r.lastValidate
	r.recordLock.RUnlock()
	return validate
}

func (r *MockResourceOps) SetLastServerSideApply(serverSideApply bool) {
	r.recordLock.Lock()
	r.serverSideApply = serverSideApply
	r.recordLock.Unlock()
}

func (r *MockResourceOps) GetLastServerSideApplyManager() string {
	r.recordLock.Lock()
	manager := r.serverSideApplyManager
	r.recordLock.Unlock()
	return manager
}

func (r *MockResourceOps) GetLastServerSideApply() bool {
	r.recordLock.RLock()
	serverSideApply := r.serverSideApply
	r.recordLock.RUnlock()
	return serverSideApply
}

func (r *MockResourceOps) SetLastServerSideApplyManager(manager string) {
	r.recordLock.Lock()
	r.serverSideApplyManager = manager
	r.recordLock.Unlock()
}

func (r *MockResourceOps) SetLastForce(force bool) {
	r.recordLock.Lock()
	r.lastForce = force
	r.recordLock.Unlock()
}

func (r *MockResourceOps) GetLastForce() bool {
	r.recordLock.RLock()
	force := r.lastForce
	r.recordLock.RUnlock()
	return force
}

func (r *MockResourceOps) SetLastResourceCommand(key kube.ResourceKey, cmd string) {
	r.recordLock.Lock()
	if r.lastCommandPerResource == nil {
		r.lastCommandPerResource = map[kube.ResourceKey]string{}
	}
	r.lastCommandPerResource[key] = cmd
	r.recordLock.Unlock()
}

func (r *MockResourceOps) GetLastResourceCommand(key kube.ResourceKey) string {
	r.recordLock.Lock()
	defer r.recordLock.Unlock()
	if r.lastCommandPerResource == nil {
		return ""
	}
	return r.lastCommandPerResource[key]
}

func (r *MockResourceOps) ApplyResource(_ context.Context, obj *unstructured.Unstructured, _ cmdutil.DryRunStrategy, force bool, validate bool, serverSideApply bool, manager string) (string, error) {
	r.SetLastValidate(validate)
	r.SetLastServerSideApply(serverSideApply)
	r.SetLastServerSideApplyManager(manager)
	r.SetLastForce(force)
	r.SetLastResourceCommand(kube.GetResourceKey(obj), "apply")
	command, ok := r.Commands[obj.GetName()]
	if !ok {
		return "", nil
	}

	return command.Output, command.Err
}

func (r *MockResourceOps) ReplaceResource(_ context.Context, obj *unstructured.Unstructured, _ cmdutil.DryRunStrategy, force bool) (string, error) {
	r.SetLastForce(force)
	command, ok := r.Commands[obj.GetName()]
	r.SetLastResourceCommand(kube.GetResourceKey(obj), "replace")
	if !ok {
		return "", nil
	}

	return command.Output, command.Err
}

func (r *MockResourceOps) UpdateResource(_ context.Context, obj *unstructured.Unstructured, _ cmdutil.DryRunStrategy) (*unstructured.Unstructured, error) {
	r.SetLastResourceCommand(kube.GetResourceKey(obj), "update")
	command, ok := r.Commands[obj.GetName()]
	if !ok {
		return obj, nil
	}
	return obj, command.Err
}

func (r *MockResourceOps) CreateResource(_ context.Context, obj *unstructured.Unstructured, _ cmdutil.DryRunStrategy, _ bool) (string, error) {
	r.SetLastResourceCommand(kube.GetResourceKey(obj), "create")
	command, ok := r.Commands[obj.GetName()]
	if !ok {
		return "", nil
	}
	return command.Output, command.Err
}

/*func (r *MockResourceOps) ConvertToVersion(obj *unstructured.Unstructured, group, version string) (*unstructured.Unstructured, error) {
	if r.convertToVersionFunc != nil {
		return (*r.convertToVersionFunc)(obj, group, version)
	}

	return obj, nil
}*/
