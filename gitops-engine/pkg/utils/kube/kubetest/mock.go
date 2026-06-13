package kubetest

import (
	"context"

	"k8s.io/kubectl/pkg/cmd/util"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/diff"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/util/openapi"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube"
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

	convertToVersionFunc           *func(obj *unstructured.Unstructured, group, version string) (*unstructured.Unstructured, error)
	getResourceFunc                *func(ctx context.Context, config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string) (*unstructured.Unstructured, error)
	loadOpenAPISchemaFunc          *func(config *rest.Config) (openapi.Resources, *managedfields.GvkParser, error)
	manageServerSideDiffDryRunFunc *func(config *rest.Config) (diff.KubeApplier, func(), error)
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

// WithLoadOpenAPISchemaFunc overrides the default LoadOpenAPISchema behavior.
func (k *MockKubectlCmd) WithLoadOpenAPISchemaFunc(loadOpenAPISchemaFunc func(*rest.Config) (openapi.Resources, *managedfields.GvkParser, error)) *MockKubectlCmd {
	k.loadOpenAPISchemaFunc = &loadOpenAPISchemaFunc
	return k
}

// WithManageServerSideDiffDryRunFunc overrides the default ManageServerSideDiffDryRuns behavior.
func (k *MockKubectlCmd) WithManageServerSideDiffDryRunFunc(manageServerSideDiffDryRunFunc func(*rest.Config) (diff.KubeApplier, func(), error)) *MockKubectlCmd {
	k.manageServerSideDiffDryRunFunc = &manageServerSideDiffDryRunFunc
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

func (k *MockKubectlCmd) LoadOpenAPISchema(config *rest.Config) (openapi.Resources, *managedfields.GvkParser, error) {
	if k.loadOpenAPISchemaFunc != nil {
		return (*k.loadOpenAPISchemaFunc)(config)
	}
	return nil, nil, nil
}

func (k *MockKubectlCmd) SetOnKubectlRun(_ kube.OnKubectlRunFunc) {
}

func (k *MockKubectlCmd) ManageResources(_ *rest.Config) (kube.ResourceOperations, func(), error) {
	return &MockResourceOps{}, func() {
	}, nil
}

func (k *MockKubectlCmd) ManageServerSideDiffDryRuns(config *rest.Config) (diff.KubeApplier, func(), error) {
	if k.manageServerSideDiffDryRunFunc != nil {
		return (*k.manageServerSideDiffDryRunFunc)(config)
	}
	return &MockKubeApplier{}, func() {}, nil
}

// MockKubeApplier is a mock implementation of diff.KubeApplier for testing
type MockKubeApplier struct {
	// ApplyResourceFunc allows custom override behavior
	ApplyResourceFunc func(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy util.DryRunStrategy, force, validate, serverSideApply bool, manager string) (string, error)
}

func (m *MockKubeApplier) ApplyResource(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy util.DryRunStrategy, force, validate, serverSideApply bool, manager string) (string, error) {
	if m.ApplyResourceFunc != nil {
		return m.ApplyResourceFunc(ctx, obj, dryRunStrategy, force, validate, serverSideApply, manager)
	}
	return "", nil
}
