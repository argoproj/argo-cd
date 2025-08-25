package kubetest

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
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

func (k *MockKubectlCmd) NewDynamicClient(_ *rest.Config) (dynamic.Interface, error) {
	return k.DynamicClient, nil
}

func (k *MockKubectlCmd) GetAPIResources(_ *rest.Config, _ bool, _ kube.ResourceFilter) ([]kube.APIResourceInfo, error) {
	return k.APIResources, nil
}

func (k *MockKubectlCmd) GetResource(ctx context.Context, config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string) (*unstructured.Unstructured, error) {
	if k.getResourceFunc != nil {
		return (*k.getResourceFunc)(ctx, config, gvk, name, namespace)
	}

	return nil, nil
}

func (k *MockKubectlCmd) PatchResource(_ context.Context, _ *rest.Config, _ schema.GroupVersionKind, _ string, _ string, _ types.PatchType, _ []byte, _ ...string) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (k *MockKubectlCmd) DeleteResource(_ context.Context, _ *rest.Config, _ schema.GroupVersionKind, name string, _ string, _ metav1.DeleteOptions) error {
	command, ok := k.Commands[name]
	if !ok {
		return nil
	}
	return command.Err
}

func (k *MockKubectlCmd) CreateResource(_ context.Context, _ *rest.Config, _ schema.GroupVersionKind, _ string, _ string, _ *unstructured.Unstructured, _ metav1.CreateOptions, _ ...string) (*unstructured.Unstructured, error) {
	return nil, nil
}

// ConvertToVersion converts an unstructured object into the specified group/version
func (k *MockKubectlCmd) ConvertToVersion(obj *unstructured.Unstructured, group, version string) (*unstructured.Unstructured, error) {
	if k.convertToVersionFunc != nil {
		return (*k.convertToVersionFunc)(obj, group, version)
	}

	return obj, nil
}

func (k *MockKubectlCmd) GetServerVersion(_ *rest.Config) (string, error) {
	return k.Version, nil
}

func (k *MockKubectlCmd) LoadOpenAPISchema(_ *rest.Config) (openapi.Resources, *managedfields.GvkParser, error) {
	return nil, nil, nil
}

func (k *MockKubectlCmd) SetOnKubectlRun(_ kube.OnKubectlRunFunc) {
}

func (k *MockKubectlCmd) ManageResources(_ *rest.Config, _ openapi.Resources) (kube.ResourceOperations, func(), error) {
	return &MockResourceOps{}, func() {
	}, nil
}
