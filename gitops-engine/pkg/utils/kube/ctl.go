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
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/kube-openapi/pkg/util/proto"
	"sigs.k8s.io/structured-merge-diff/v6/typed"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/diff"
	utils "github.com/argoproj/argo-cd/gitops-engine/pkg/utils/io"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube/scheme"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/tracing"
)

type CleanupFunc func()

type OnKubectlRunFunc func(command string) (CleanupFunc, error)

type Kubectl interface {
	ManageResources(config *rest.Config) (ResourceOperations, func(), error)
	LoadOpenAPISchema(config *rest.Config) (scheme.GVKParser, error)
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
	Log            logr.Logger
	Tracer         tracing.Tracer
	OnKubectlRun   OnKubectlRunFunc
	UseOpenAPIV3   bool
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
		return nil, fmt.Errorf("failed to create discovery client: %w", err)
	}

	var serverResources []*metav1.APIResourceList
	if preferred {
		serverResources, err = disco.ServerPreferredResources()
	} else {
		_, serverResources, err = disco.ServerGroupsAndResources()
	}

	if err != nil {
		if len(serverResources) == 0 {
			return nil, fmt.Errorf("failed to discover server resources, zero resources returned: %w", err)
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
					return nil, fmt.Errorf("failed to parse group version %q: %w", apiResourcesList.GroupVersion, err)
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
// and return a GvkParser used for building a ParseableType to be used in
// structured-merge-diffs. If UseOpenAPIV3 is enabled, schemas are fetched
// per-GroupVersion using the OpenAPI v3 discovery endpoint; otherwise, the
// monolithic OpenAPI v2 document is used.
func (k *KubectlCmd) LoadOpenAPISchema(config *rest.Config) (scheme.GVKParser, error) {
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %w", err)
	}

	if k.UseOpenAPIV3 {
		return k.loadLazyGVKParser(disco)
	}
	return k.loadGVKParserV2(disco)
}

func (k *KubectlCmd) loadLazyGVKParser(disco *discovery.DiscoveryClient) (scheme.GVKParser, error) {
	client := disco.OpenAPIV3()
	paths, err := client.Paths()
	if err != nil {
		return nil, fmt.Errorf("error getting openapi v3 paths: %w", err)
	}
	return newLazyGVKParser(paths, k.Log), nil
}

// eagerGVKParser wraps a managedfields.GvkParser to satisfy scheme.GVKParser.
// Since the eager (v2) parser loads all schemas upfront, Type() never errors.
type eagerGVKParser struct {
	parser *managedfields.GvkParser
}

func (e *eagerGVKParser) Type(gvk schema.GroupVersionKind) (*typed.ParseableType, error) {
	return e.parser.Type(gvk), nil
}

func (k *KubectlCmd) loadGVKParserV2(disco discovery.OpenAPISchemaInterface) (*eagerGVKParser, error) {
	doc, err := disco.OpenAPISchema()
	if err != nil {
		return nil, fmt.Errorf("error getting openapi v2 schema: %w", err)
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
		return nil, fmt.Errorf("failed to create GVK parser: %w", err)
	}
	return &eagerGVKParser{parser: gvkParser}, nil
}


// normalizeV3Extensions fixes a compatibility issue between OpenAPI v3 proto
// models and managedfields.NewGVKParser. The v3 path (proto.NewOpenAPIV3Data)
// produces map[string]interface{} for nested values in schema extensions like
// x-kubernetes-group-version-kind and x-kubernetes-unions, but the upstream
// GVK parser and schema converter expect map[interface{}]interface{} (which is
// what the v2 proto path produces). This function normalizes all extension
// values in-place.
func normalizeV3Extensions(models proto.Models) {
	for _, name := range models.ListModels() {
		m := models.LookupModel(name)
		if m == nil {
			continue
		}
		exts := m.GetExtensions()
		for key, val := range exts {
			exts[key] = deepConvertStringKeysToInterface(val)
		}
	}
}

// deepConvertStringKeysToInterface recursively converts map[string]interface{}
// to map[interface{}]interface{} within a value tree.
func deepConvertStringKeysToInterface(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		result := make(map[interface{}]interface{}, len(val))
		for k, v := range val {
			result[k] = deepConvertStringKeysToInterface(v)
		}
		return result
	case []interface{}:
		for i, item := range val {
			val[i] = deepConvertStringKeysToInterface(item)
		}
		return val
	default:
		return v
	}
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
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %w", err)
	}
	apiResource, err := ServerResourceForGroupVersionKind(disco, gvk, "get")
	if err != nil {
		return nil, fmt.Errorf("failed to get api resource: %w", err)
	}
	resource := gvk.GroupVersion().WithResource(apiResource.Name)
	resourceIf := ToResourceInterface(dynamicIf, apiResource, resource, namespace)
	//nolint:wrapcheck // wrapped message would be same as calling method's wrapped error
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
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %w", err)
	}
	apiResource, err := ServerResourceForGroupVersionKind(disco, gvk, "create")
	if err != nil {
		return nil, fmt.Errorf("failed to get api resource: %w", err)
	}
	resource := gvk.GroupVersion().WithResource(apiResource.Name)
	resourceIf := ToResourceInterface(dynamicIf, apiResource, resource, namespace)
	//nolint:wrapcheck // wrapped message would be same as calling method's wrapped error
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
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %w", err)
	}
	apiResource, err := ServerResourceForGroupVersionKind(disco, gvk, "patch")
	if err != nil {
		return nil, fmt.Errorf("failed to get api resource: %w", err)
	}
	resource := gvk.GroupVersion().WithResource(apiResource.Name)
	resourceIf := ToResourceInterface(dynamicIf, apiResource, resource, namespace)
	//nolint:wrapcheck // wrapped message would be same as calling method's wrapped error
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
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create discovery client: %w", err)
	}
	apiResource, err := ServerResourceForGroupVersionKind(disco, gvk, "delete")
	if err != nil {
		return fmt.Errorf("failed to get api resource: %w", err)
	}
	resource := gvk.GroupVersion().WithResource(apiResource.Name)
	resourceIf := ToResourceInterface(dynamicIf, apiResource, resource, namespace)

	if deleteOptions.PropagationPolicy == nil {
		propagationPolicy := metav1.DeletePropagationForeground
		deleteOptions = metav1.DeleteOptions{PropagationPolicy: &propagationPolicy}
	}
	//nolint:wrapcheck // wrapped message would be same as calling method's wrapped error
	return resourceIf.Delete(ctx, name, deleteOptions)
}

func (k *KubectlCmd) ManageResources(config *rest.Config) (ResourceOperations, func(), error) {
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
		config:       config,
		fact:         fact,
		tracer:       k.Tracer,
		log:          k.Log,
		onKubectlRun: k.OnKubectlRun,
	}, cleanup, nil
}

func ManageServerSideDiffDryRuns(config *rest.Config, tracer tracing.Tracer, log logr.Logger, onKubectlRun OnKubectlRunFunc) (diff.KubeApplier, func(), error) {
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
		config:       config,
		fact:         fact,
		tracer:       tracer,
		log:          log,
		onKubectlRun: onKubectlRun,
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
		return "", fmt.Errorf("failed to create discovery client: %w", err)
	}
	v, err := client.ServerVersion()
	if err != nil {
		return "", fmt.Errorf("failed to get server version: %w", err)
	}

	ver, err := version.ParseGeneric(v.GitVersion)
	if err != nil {
		return "", fmt.Errorf("failed to parse server version: %w", err)
	}
	// ParseGeneric removes the leading "v" and any vendor-specific suffix (e.g. "-gke.100", "-eks-123", "+k3s1").
	// Helm expects a semver-like Kubernetes version with a "v" prefix for capability checks, so we normalize the
	// version to "v<major>.<minor>.<patch>".
	return "v" + ver.String(), nil
}

func (k *KubectlCmd) NewDynamicClient(config *rest.Config) (dynamic.Interface, error) {
	//nolint:wrapcheck // wrapped error message would be the same as the caller's wrapped message
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
	//nolint:wrapcheck // don't wrap message from utility function
	return g.Wait()
}
