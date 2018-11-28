package kubetest

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
)

type KubectlOutput struct {
	Output string
	Err    error
}

type MockKubectlCmd struct {
	Resources []*unstructured.Unstructured
	Commands  map[string]KubectlOutput
	Events    chan watch.Event
}

func (k MockKubectlCmd) GetResources(config *rest.Config, namespace string) ([]*unstructured.Unstructured, error) {
	return k.Resources, nil
}

func (k MockKubectlCmd) GetResource(config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (k MockKubectlCmd) WatchResources(
	ctx context.Context, config *rest.Config, namespace string) (chan watch.Event, error) {

	return k.Events, nil
}

func (k MockKubectlCmd) DeleteResource(config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string) error {
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
