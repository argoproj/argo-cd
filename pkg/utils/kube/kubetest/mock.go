package kubetest

import (
	"context"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/openapi"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
)

type KubectlOutput struct {
	Output string
	Err    error
}

type MockKubectlCmd struct {
	APIResources  []kube.APIResourceInfo
	Commands      map[string]KubectlOutput
	Events        chan watch.Event
	Version       string
	DynamicClient dynamic.Interface

	lastCommandPerResource map[kube.ResourceKey]string
	lastValidate           bool
	serverSideApply        bool
	recordLock             sync.RWMutex

	convertToVersionFunc *func(obj *unstructured.Unstructured, group, version string) (*unstructured.Unstructured, error)
	getResourceFunc      *func(ctx context.Context, config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string) (*unstructured.Unstructured, error)
}

// WithConvertToVersionFunc overrides the default ConvertToVersion behavior.
func (k *MockKubectlCmd) WithConvertToVersionFunc(convertToVersionFunc func(*unstructured.Unstructured, string, string) (*unstructured.Unstructured, error)) *MockKubectlCmd {
	k.convertToVersionFunc = &convertToVersionFunc
	return k
}

// WithGetResourceFunc overrides the default ConvertToVersion behavior.
func (k *MockKubectlCmd) WithGetResourceFunc(getResourcefunc func(context.Context, *rest.Config, schema.GroupVersionKind, string, string) (*unstructured.Unstructured, error)) *MockKubectlCmd {
	k.getResourceFunc = &getResourcefunc
	return k
}

func (k *MockKubectlCmd) GetLastResourceCommand(key kube.ResourceKey) string {
	k.recordLock.Lock()
	defer k.recordLock.Unlock()
	if k.lastCommandPerResource == nil {
		return ""
	}
	return k.lastCommandPerResource[key]
}

func (k *MockKubectlCmd) SetLastResourceCommand(key kube.ResourceKey, cmd string) {
	k.recordLock.Lock()
	if k.lastCommandPerResource == nil {
		k.lastCommandPerResource = map[kube.ResourceKey]string{}
	}
	k.lastCommandPerResource[key] = cmd
	k.recordLock.Unlock()
}

func (k *MockKubectlCmd) SetLastValidate(validate bool) {
	k.recordLock.Lock()
	k.lastValidate = validate
	k.recordLock.Unlock()
}

func (k *MockKubectlCmd) GetLastValidate() bool {
	k.recordLock.RLock()
	validate := k.lastValidate
	k.recordLock.RUnlock()
	return validate
}

func (k *MockKubectlCmd) SetLastServerSideApply(serverSideApply bool) {
	k.recordLock.Lock()
	k.serverSideApply = serverSideApply
	k.recordLock.Unlock()
}

func (k *MockKubectlCmd) GetLastServerSideApply() bool {
	k.recordLock.RLock()
	serverSideApply := k.serverSideApply
	k.recordLock.RUnlock()
	return serverSideApply
}

func (k *MockKubectlCmd) NewDynamicClient(config *rest.Config) (dynamic.Interface, error) {
	return k.DynamicClient, nil
}

func (k *MockKubectlCmd) GetAPIResources(config *rest.Config, preferred bool, resourceFilter kube.ResourceFilter) ([]kube.APIResourceInfo, error) {
	return k.APIResources, nil
}

func (k *MockKubectlCmd) GetResource(ctx context.Context, config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string) (*unstructured.Unstructured, error) {
	if k.getResourceFunc != nil {
		return (*k.getResourceFunc)(ctx, config, gvk, name, namespace)
	}

	return nil, nil
}

func (k *MockKubectlCmd) PatchResource(ctx context.Context, config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string, patchType types.PatchType, patchBytes []byte, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (k *MockKubectlCmd) DeleteResource(ctx context.Context, config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string, deleteOptions metav1.DeleteOptions) error {
	command, ok := k.Commands[name]
	if !ok {
		return nil
	}
	return command.Err
}

func (k *MockKubectlCmd) CreateResource(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy, validate bool) (string, error) {
	k.SetLastResourceCommand(kube.GetResourceKey(obj), "create")
	command, ok := k.Commands[obj.GetName()]
	if !ok {
		return "", nil
	}
	return command.Output, command.Err
}

func (k *MockKubectlCmd) UpdateResource(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy) (*unstructured.Unstructured, error) {
	k.SetLastResourceCommand(kube.GetResourceKey(obj), "update")
	command, ok := k.Commands[obj.GetName()]
	if !ok {
		return obj, nil
	}
	return obj, command.Err
}

func (k *MockKubectlCmd) ApplyResource(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy, force, validate, serverSideApply bool) (string, error) {
	k.SetLastValidate(validate)
	k.SetLastServerSideApply(serverSideApply)
	k.SetLastResourceCommand(kube.GetResourceKey(obj), "apply")
	command, ok := k.Commands[obj.GetName()]
	if !ok {
		return "", nil
	}
	return command.Output, command.Err
}

func (k *MockKubectlCmd) ReplaceResource(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy, force bool) (string, error) {
	command, ok := k.Commands[obj.GetName()]
	k.SetLastResourceCommand(kube.GetResourceKey(obj), "replace")
	if !ok {
		return "", nil
	}
	return command.Output, command.Err
}

// ConvertToVersion converts an unstructured object into the specified group/version
func (k *MockKubectlCmd) ConvertToVersion(obj *unstructured.Unstructured, group, version string) (*unstructured.Unstructured, error) {
	if k.convertToVersionFunc != nil {
		return (*k.convertToVersionFunc)(obj, group, version)
	}

	return obj, nil
}

func (k *MockKubectlCmd) GetServerVersion(config *rest.Config) (string, error) {
	return k.Version, nil
}

func (k *MockKubectlCmd) LoadOpenAPISchema(config *rest.Config) (openapi.Resources, *managedfields.GvkParser, error) {
	return nil, nil, nil
}

func (k *MockKubectlCmd) SetOnKubectlRun(onKubectlRun kube.OnKubectlRunFunc) {
}

func (k *MockKubectlCmd) ManageResources(config *rest.Config, openAPISchema openapi.Resources) (kube.ResourceOperations, func(), error) {
	return k, func() {

	}, nil
}
