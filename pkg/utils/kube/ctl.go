package kube

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-logr/logr"
	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/kube-openapi/pkg/util/proto"
	"k8s.io/kubectl/pkg/util/openapi"

	"github.com/argoproj/gitops-engine/pkg/diff"
	utils "github.com/argoproj/gitops-engine/pkg/utils/io"
	"github.com/argoproj/gitops-engine/pkg/utils/tracing"
)

type CleanupFunc func()

type OnKubectlRunFunc func(command string) (CleanupFunc, error)

type Kubectl interface {
	ManageResources(config *rest.Config, openAPISchema openapi.Resources) (ResourceOperations, func(), error)
	LoadOpenAPISchema(config *rest.Config) (openapi.Resources, *managedfields.GvkParser, error)
	ConvertToVersion(obj *unstructured.Unstructured, group, version string) (*unstructured.Unstructured, error)
	DeleteResource(ctx context.Context, config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string, deleteOptions metav1.DeleteOptions) error
	GetResource(ctx context.Context, config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string) (*unstructured.Unstructured, error)
	CreateResource(ctx context.Context, config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string, obj *unstructured.Unstructured, createOptions metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error)
	PatchResource(ctx context.Context, config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string, patchType types.PatchType, patchBytes []byte, subresources ...string) (*unstructured.Unstructured, error)
	GetAPIResources(config *rest.Config, preferred bool, resourceFilter ResourceFilter) ([]APIResourceInfo, error)
	GetServerVersion(config *rest.Config) (string, error)
	NewDynamicClient(config *rest.Config) (dynamic.Interface, error)
	SetOnKubectlRun(onKubectlRun OnKubectlRunFunc)
}

type KubectlCmd struct {
	Log          logr.Logger
	Tracer       tracing.Tracer
	OnKubectlRun OnKubectlRunFunc
}

type APIResourceInfo struct {
	GroupKind            schema.GroupKind
	Meta                 metav1.APIResource
	GroupVersionResource schema.GroupVersionResource
}

type filterFunc func(apiResource *metav1.APIResource) bool

func (k *KubectlCmd) filterAPIResources(config *rest.Config, preferred bool, resourceFilter ResourceFilter, filter filterFunc) ([]APIResourceInfo, error) {
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}

	var serverResources []*metav1.APIResourceList
	if preferred {
		serverResources, err = disco.ServerPreferredResources()
	} else {
		_, serverResources, err = disco.ServerGroupsAndResources()
	}

	if err != nil {
		if len(serverResources) == 0 {
			return nil, err
		}
		k.Log.Error(err, "Partial success when performing preferred resource discovery")
	}
	apiResIfs := make([]APIResourceInfo, 0)
	for _, apiResourcesList := range serverResources {
		gv, err := schema.ParseGroupVersion(apiResourcesList.GroupVersion)
		if err != nil {
			gv = schema.GroupVersion{}
		}
		for _, apiResource := range apiResourcesList.APIResources {
			if resourceFilter.IsExcludedResource(gv.Group, apiResource.Kind, config.Host) {
				continue
			}

			if filter(&apiResource) {
				resource := ToGroupVersionResource(apiResourcesList.GroupVersion, &apiResource)
				gv, err := schema.ParseGroupVersion(apiResourcesList.GroupVersion)
				if err != nil {
					return nil, err
				}
				apiResIf := APIResourceInfo{
					GroupKind:            schema.GroupKind{Group: gv.Group, Kind: apiResource.Kind},
					Meta:                 apiResource,
					GroupVersionResource: resource,
				}
				apiResIfs = append(apiResIfs, apiResIf)
			}
		}
	}
	return apiResIfs, nil
}

// isSupportedVerb returns whether or not a APIResource supports a specific verb.
// The verb will be matched case-insensitive.
func isSupportedVerb(apiResource *metav1.APIResource, verb string) bool {
	if verb == "" || verb == "*" {
		return true
	}
	for _, v := range apiResource.Verbs {
		if strings.EqualFold(v, verb) {
			return true
		}
	}
	return false
}

// LoadOpenAPISchema will load all existing resource schemas from the cluster
// and return:
// - openapi.Resources: used for getting the proto.Schema from a GVK
// - managedfields.GvkParser: used for building a ParseableType to be used in
// structured-merge-diffs
func (k *KubectlCmd) LoadOpenAPISchema(config *rest.Config) (openapi.Resources, *managedfields.GvkParser, error) {
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, nil, err
	}

	oapiGetter := openapi.NewOpenAPIGetter(disco)
	oapiResources, err := openapi.NewOpenAPIParser(oapiGetter).Parse()
	if err != nil {
		return nil, nil, fmt.Errorf("error getting openapi resources: %w", err)
	}
	gvkParser, err := k.newGVKParser(oapiGetter)
	if err != nil {
		return oapiResources, nil, fmt.Errorf("error getting gvk parser: %w", err)
	}
	return oapiResources, gvkParser, nil
}

func (k *KubectlCmd) newGVKParser(oapiGetter discovery.OpenAPISchemaInterface) (*managedfields.GvkParser, error) {
	doc, err := oapiGetter.OpenAPISchema()
	if err != nil {
		return nil, fmt.Errorf("error getting openapi schema: %w", err)
	}
	models, err := proto.NewOpenAPIData(doc)
	if err != nil {
		return nil, fmt.Errorf("error getting openapi data: %w", err)
	}
	var taintedGVKs []schema.GroupVersionKind
	models, taintedGVKs = newUniqueModels(models)
	if len(taintedGVKs) > 0 {
		k.Log.Info("Duplicate GVKs detected in OpenAPI schema. This could cause inaccurate diffs.", "gvks", taintedGVKs)
	}
	gvkParser, err := managedfields.NewGVKParser(models, false)
	if err != nil {
		return nil, err
	}
	return gvkParser, nil
}

func (k *KubectlCmd) GetAPIResources(config *rest.Config, preferred bool, resourceFilter ResourceFilter) ([]APIResourceInfo, error) {
	span := k.Tracer.StartSpan("GetAPIResources")
	defer span.Finish()
	apiResIfs, err := k.filterAPIResources(config, preferred, resourceFilter, func(apiResource *metav1.APIResource) bool {
		return isSupportedVerb(apiResource, listVerb) && isSupportedVerb(apiResource, watchVerb)
	})
	if err != nil {
		return nil, err
	}
	return apiResIfs, err
}

// GetResource returns resource
func (k *KubectlCmd) GetResource(ctx context.Context, config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string) (*unstructured.Unstructured, error) {
	span := k.Tracer.StartSpan("GetResource")
	span.SetBaggageItem("kind", gvk.Kind)
	span.SetBaggageItem("name", name)
	defer span.Finish()
	dynamicIf, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}
	apiResource, err := ServerResourceForGroupVersionKind(disco, gvk, "get")
	if err != nil {
		return nil, err
	}
	resource := gvk.GroupVersion().WithResource(apiResource.Name)
	resourceIf := ToResourceInterface(dynamicIf, apiResource, resource, namespace)
	return resourceIf.Get(ctx, name, metav1.GetOptions{})
}

// CreateResource creates resource
func (k *KubectlCmd) CreateResource(ctx context.Context, config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string, obj *unstructured.Unstructured, createOptions metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	span := k.Tracer.StartSpan("CreateResource")
	span.SetBaggageItem("kind", gvk.Kind)
	span.SetBaggageItem("name", name)
	defer span.Finish()
	dynamicIf, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}
	apiResource, err := ServerResourceForGroupVersionKind(disco, gvk, "create")
	if err != nil {
		return nil, err
	}
	resource := gvk.GroupVersion().WithResource(apiResource.Name)
	resourceIf := ToResourceInterface(dynamicIf, apiResource, resource, namespace)
	return resourceIf.Create(ctx, obj, createOptions, subresources...)
}

// PatchResource patches resource
func (k *KubectlCmd) PatchResource(ctx context.Context, config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string, patchType types.PatchType, patchBytes []byte, subresources ...string) (*unstructured.Unstructured, error) {
	span := k.Tracer.StartSpan("PatchResource")
	span.SetBaggageItem("kind", gvk.Kind)
	span.SetBaggageItem("name", name)
	defer span.Finish()
	dynamicIf, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}
	apiResource, err := ServerResourceForGroupVersionKind(disco, gvk, "patch")
	if err != nil {
		return nil, err
	}
	resource := gvk.GroupVersion().WithResource(apiResource.Name)
	resourceIf := ToResourceInterface(dynamicIf, apiResource, resource, namespace)
	return resourceIf.Patch(ctx, name, patchType, patchBytes, metav1.PatchOptions{}, subresources...)
}

// DeleteResource deletes resource
func (k *KubectlCmd) DeleteResource(ctx context.Context, config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string, deleteOptions metav1.DeleteOptions) error {
	span := k.Tracer.StartSpan("DeleteResource")
	span.SetBaggageItem("kind", gvk.Kind)
	span.SetBaggageItem("name", name)
	defer span.Finish()
	dynamicIf, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return err
	}
	apiResource, err := ServerResourceForGroupVersionKind(disco, gvk, "delete")
	if err != nil {
		return err
	}
	resource := gvk.GroupVersion().WithResource(apiResource.Name)
	resourceIf := ToResourceInterface(dynamicIf, apiResource, resource, namespace)

	if deleteOptions.PropagationPolicy == nil {
		propagationPolicy := metav1.DeletePropagationForeground
		deleteOptions = metav1.DeleteOptions{PropagationPolicy: &propagationPolicy}
	}
	return resourceIf.Delete(ctx, name, deleteOptions)
}

func (k *KubectlCmd) ManageResources(config *rest.Config, openAPISchema openapi.Resources) (ResourceOperations, func(), error) {
	f, err := os.CreateTemp(utils.TempDir, "")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate temp file for kubeconfig: %w", err)
	}
	_ = f.Close()
	err = WriteKubeConfig(config, "", f.Name())
	if err != nil {
		utils.DeleteFile(f.Name())
		return nil, nil, fmt.Errorf("failed to write kubeconfig: %w", err)
	}
	fact := kubeCmdFactory(f.Name(), "", config)
	cleanup := func() {
		utils.DeleteFile(f.Name())
	}
	return &kubectlResourceOperations{
		config:        config,
		fact:          fact,
		openAPISchema: openAPISchema,
		tracer:        k.Tracer,
		log:           k.Log,
		onKubectlRun:  k.OnKubectlRun,
	}, cleanup, nil
}

func ManageServerSideDiffDryRuns(config *rest.Config, openAPISchema openapi.Resources, tracer tracing.Tracer, log logr.Logger, onKubectlRun OnKubectlRunFunc) (diff.KubeApplier, func(), error) {
	f, err := os.CreateTemp(utils.TempDir, "")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate temp file for kubeconfig: %w", err)
	}
	_ = f.Close()
	err = WriteKubeConfig(config, "", f.Name())
	if err != nil {
		utils.DeleteFile(f.Name())
		return nil, nil, fmt.Errorf("failed to write kubeconfig: %w", err)
	}
	fact := kubeCmdFactory(f.Name(), "", config)
	cleanup := func() {
		utils.DeleteFile(f.Name())
	}
	return &kubectlServerSideDiffDryRunApplier{
		config:        config,
		fact:          fact,
		openAPISchema: openAPISchema,
		tracer:        tracer,
		log:           log,
		onKubectlRun:  onKubectlRun,
	}, cleanup, nil
}

// ConvertToVersion converts an unstructured object into the specified group/version
func (k *KubectlCmd) ConvertToVersion(obj *unstructured.Unstructured, group string, version string) (*unstructured.Unstructured, error) {
	span := k.Tracer.StartSpan("ConvertToVersion")
	from := obj.GroupVersionKind().GroupVersion()
	span.SetBaggageItem("from", from.String())
	span.SetBaggageItem("to", schema.GroupVersion{Group: group, Version: version}.String())
	defer span.Finish()
	if from.Group == group && from.Version == version {
		return obj.DeepCopy(), nil
	}
	return convertToVersionWithScheme(obj, group, version)
}

func (k *KubectlCmd) GetServerVersion(config *rest.Config) (string, error) {
	span := k.Tracer.StartSpan("GetServerVersion")
	defer span.Finish()
	client, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return "", err
	}
	v, err := client.ServerVersion()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s", v.Major, v.Minor), nil
}

func (k *KubectlCmd) NewDynamicClient(config *rest.Config) (dynamic.Interface, error) {
	return dynamic.NewForConfig(config)
}

func (k *KubectlCmd) SetOnKubectlRun(onKubectlRun OnKubectlRunFunc) {
	k.OnKubectlRun = onKubectlRun
}

func RunAllAsync(count int, action func(i int) error) error {
	g, ctx := errgroup.WithContext(context.Background())
loop:
	for i := 0; i < count; i++ {
		index := i
		g.Go(func() error {
			return action(index)
		})
		select {
		case <-ctx.Done():
			// Something went wrong already, stop spawning tasks.
			break loop
		default:
		}
	}
	return g.Wait()
}
