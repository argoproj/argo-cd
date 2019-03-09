package kubetest

import (
	"context"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"

	"github.com/argoproj/argo-cd/util/kube"
)

type KubectlOutput struct {
	Output string
	Err    error
}

type MockKubectlCmd struct {
	APIResources []*v1.APIResourceList
	Resources    []kube.ResourcesBatch
	Commands     map[string]KubectlOutput
	Events       chan kube.WatchEvent
}

func (k MockKubectlCmd) GetAPIResources(config *rest.Config) ([]*v1.APIResourceList, error) {
	return k.APIResources, nil
}

func (k MockKubectlCmd) GetResources(config *rest.Config, resourceFilter kube.ResourceFilter, namespace string) (chan kube.ResourcesBatch, error) {
	res := make(chan kube.ResourcesBatch)
	go func() {
		defer close(res)
		for i := range k.Resources {
			res <- k.Resources[i]
		}
	}()
	return res, nil
}

func (k MockKubectlCmd) GetResource(config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (k MockKubectlCmd) PatchResource(config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string, patchType types.PatchType, patchBytes []byte) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (k MockKubectlCmd) WatchResources(
	ctx context.Context, config *rest.Config, resourceFilter kube.ResourceFilter, versionSource kube.CachedVersionSource) (chan kube.WatchEvent, error) {

	return k.Events, nil
}

func (k MockKubectlCmd) DeleteResource(config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string, forceDelete bool) error {
	command, ok := k.Commands[name]
	if !ok {
		return nil
	}
	return command.Err
}

func (k MockKubectlCmd) ApplyResource(config *rest.Config, obj *unstructured.Unstructured, namespace string, dryRun, force bool) (string, error) {
	command, ok := k.Commands[obj.GetName()]
	if !ok {
		return "", nil
	}
	return command.Output, command.Err
}

// ConvertToVersion converts an unstructured object into the specified group/version
func (k MockKubectlCmd) ConvertToVersion(obj *unstructured.Unstructured, group, version string) (*unstructured.Unstructured, error) {
	return obj, nil
}
