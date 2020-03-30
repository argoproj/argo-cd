package kubetest

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/kube"
)

type KubectlOutput struct {
	Output string
	Err    error
}

type MockKubectlCmd struct {
	APIResources  []kube.APIResourceInfo
	Commands      map[string]KubectlOutput
	Events        chan watch.Event
	LastValidate  bool
	Version       string
	DynamicClient dynamic.Interface
	APIGroups     []metav1.APIGroup
}

func (k *MockKubectlCmd) NewDynamicClient(config *rest.Config) (dynamic.Interface, error) {
	return k.DynamicClient, nil
}

func (k *MockKubectlCmd) GetAPIResources(config *rest.Config, resourceFilter kube.ResourceFilter) ([]kube.APIResourceInfo, error) {
	return k.APIResources, nil
}

func (k *MockKubectlCmd) GetResource(config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (k *MockKubectlCmd) PatchResource(config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string, patchType types.PatchType, patchBytes []byte) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (k *MockKubectlCmd) DeleteResource(config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string, forceDelete bool) error {
	command, ok := k.Commands[name]
	if !ok {
		return nil
	}
	return command.Err
}

func (k *MockKubectlCmd) ApplyResource(config *rest.Config, obj *unstructured.Unstructured, namespace string, dryRun, force, validate bool) (string, error) {
	k.LastValidate = validate
	command, ok := k.Commands[obj.GetName()]
	if !ok {
		return "", nil
	}
	return command.Output, command.Err
}

// ConvertToVersion converts an unstructured object into the specified group/version
func (k *MockKubectlCmd) ConvertToVersion(obj *unstructured.Unstructured, group, version string) (*unstructured.Unstructured, error) {
	return obj, nil
}

func (k *MockKubectlCmd) GetServerVersion(config *rest.Config) (string, error) {
	return k.Version, nil
}

func (k *MockKubectlCmd) GetAPIGroups(config *rest.Config) ([]metav1.APIGroup, error) {
	return k.APIGroups, nil
}

func (k *MockKubectlCmd) SetOnKubectlRun(onKubectlRun func(command string) (util.Closer, error)) {
}
