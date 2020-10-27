package kube

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/go-logr/logr"
	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/apply"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/kubernetes/pkg/kubectl/cmd/auth"

	"github.com/argoproj/gitops-engine/pkg/diff"
	"github.com/argoproj/gitops-engine/pkg/utils/io"
	"github.com/argoproj/gitops-engine/pkg/utils/tracing"
)

type CleanupFunc func()

type OnKubectlRunFunc func(command string) (CleanupFunc, error)

type Kubectl interface {
	ApplyResource(ctx context.Context, config *rest.Config, obj *unstructured.Unstructured, namespace string, dryRunStrategy cmdutil.DryRunStrategy, force, validate bool) (string, error)
	ConvertToVersion(obj *unstructured.Unstructured, group, version string) (*unstructured.Unstructured, error)
	DeleteResource(ctx context.Context, config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string, forceDelete bool) error
	GetResource(ctx context.Context, config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string) (*unstructured.Unstructured, error)
	PatchResource(ctx context.Context, config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string, patchType types.PatchType, patchBytes []byte) (*unstructured.Unstructured, error)
	GetAPIResources(config *rest.Config, resourceFilter ResourceFilter) ([]APIResourceInfo, error)
	GetAPIGroups(config *rest.Config) ([]metav1.APIGroup, error)
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

func (k *KubectlCmd) filterAPIResources(config *rest.Config, resourceFilter ResourceFilter, filter filterFunc) ([]APIResourceInfo, error) {
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}

	serverResources, err := disco.ServerPreferredResources()
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

// isSupportedVerb returns whether or not a APIResource supports a specific verb
func isSupportedVerb(apiResource *metav1.APIResource, verb string) bool {
	for _, v := range apiResource.Verbs {
		if v == verb {
			return true
		}
	}
	return false
}

func (k *KubectlCmd) GetAPIGroups(config *rest.Config) ([]metav1.APIGroup, error) {
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}
	serverGroupList, err := disco.ServerGroups()
	if err != nil {
		return nil, err
	}
	return serverGroupList.Groups, nil
}

func (k *KubectlCmd) GetAPIResources(config *rest.Config, resourceFilter ResourceFilter) ([]APIResourceInfo, error) {
	span := k.Tracer.StartSpan("GetAPIResources")
	defer span.Finish()
	apiResIfs, err := k.filterAPIResources(config, resourceFilter, func(apiResource *metav1.APIResource) bool {
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
	apiResource, err := ServerResourceForGroupVersionKind(disco, gvk)
	if err != nil {
		return nil, err
	}
	resource := gvk.GroupVersion().WithResource(apiResource.Name)
	resourceIf := ToResourceInterface(dynamicIf, apiResource, resource, namespace)
	return resourceIf.Get(ctx, name, metav1.GetOptions{})
}

// PatchResource patches resource
func (k *KubectlCmd) PatchResource(ctx context.Context, config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string, patchType types.PatchType, patchBytes []byte) (*unstructured.Unstructured, error) {
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
	apiResource, err := ServerResourceForGroupVersionKind(disco, gvk)
	if err != nil {
		return nil, err
	}
	resource := gvk.GroupVersion().WithResource(apiResource.Name)
	resourceIf := ToResourceInterface(dynamicIf, apiResource, resource, namespace)
	return resourceIf.Patch(ctx, name, patchType, patchBytes, metav1.PatchOptions{})
}

// DeleteResource deletes resource
func (k *KubectlCmd) DeleteResource(ctx context.Context, config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string, forceDelete bool) error {
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
	apiResource, err := ServerResourceForGroupVersionKind(disco, gvk)
	if err != nil {
		return err
	}
	resource := gvk.GroupVersion().WithResource(apiResource.Name)
	resourceIf := ToResourceInterface(dynamicIf, apiResource, resource, namespace)
	propagationPolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{PropagationPolicy: &propagationPolicy}
	if forceDelete {
		propagationPolicy = metav1.DeletePropagationBackground
		zeroGracePeriod := int64(0)
		deleteOptions.GracePeriodSeconds = &zeroGracePeriod
	}

	return resourceIf.Delete(ctx, name, deleteOptions)
}

// ApplyResource performs an apply of a unstructured resource
func (k *KubectlCmd) ApplyResource(ctx context.Context, config *rest.Config, obj *unstructured.Unstructured, namespace string, dryRunStrategy cmdutil.DryRunStrategy, force, validate bool) (string, error) {
	span := k.Tracer.StartSpan("ApplyResource")
	span.SetBaggageItem("kind", obj.GetKind())
	span.SetBaggageItem("name", obj.GetName())
	defer span.Finish()
	k.Log.Info(fmt.Sprintf("Applying resource %s/%s in cluster: %s, namespace: %s", obj.GetKind(), obj.GetName(), config.Host, namespace))
	f, err := ioutil.TempFile(io.TempDir, "")
	if err != nil {
		return "", fmt.Errorf("Failed to generate temp file for kubeconfig: %v", err)
	}
	_ = f.Close()
	err = WriteKubeConfig(config, namespace, f.Name())
	if err != nil {
		return "", fmt.Errorf("Failed to write kubeconfig: %v", err)
	}
	defer io.DeleteFile(f.Name())
	manifestBytes, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	manifestFile, err := ioutil.TempFile(io.TempDir, "")
	if err != nil {
		return "", fmt.Errorf("Failed to generate temp file for manifest: %v", err)
	}
	if _, err = manifestFile.Write(manifestBytes); err != nil {
		return "", fmt.Errorf("Failed to write manifest: %v", err)
	}
	if err = manifestFile.Close(); err != nil {
		return "", fmt.Errorf("Failed to close manifest: %v", err)
	}
	defer io.DeleteFile(manifestFile.Name())

	// log manifest
	if k.Log.V(1).Enabled() {
		var obj unstructured.Unstructured
		err := json.Unmarshal(manifestBytes, &obj)
		if err != nil {
			return "", err
		}
		redacted, _, err := diff.HideSecretData(&obj, nil)
		if err != nil {
			return "", err
		}
		redactedBytes, err := json.Marshal(redacted)
		if err != nil {
			return "", err
		}
		k.Log.V(1).Info(string(redactedBytes))
	}

	var out []string
	if obj.GetAPIVersion() == "rbac.authorization.k8s.io/v1" {
		// If it is an RBAC resource, run `kubectl auth reconcile`. This is preferred over
		// `kubectl apply`, which cannot tolerate changes in roleRef, which is an immutable field.
		// See: https://github.com/kubernetes/kubernetes/issues/66353
		// `auth reconcile` will delete and recreate the resource if necessary
		outReconcile, err := func() (string, error) {
			cleanup, err := k.processKubectlRun("auth")
			if err != nil {
				return "", err
			}
			defer cleanup()
			return k.authReconcile(ctx, config, f.Name(), manifestFile.Name(), namespace, dryRunStrategy)
		}()
		if err != nil {
			return "", err
		}
		out = append(out, outReconcile)
		// We still want to fallthrough and run `kubectl apply` in order set the
		// last-applied-configuration annotation in the object.
	}

	cleanup, err := k.processKubectlRun("apply")
	if err != nil {
		return "", err
	}
	defer cleanup()

	// Run kubectl apply
	fact, ioStreams := kubeCmdFactory(f.Name(), namespace)
	applyOpts, err := newApplyOptions(config, fact, ioStreams, manifestFile.Name(), namespace, validate, force, dryRunStrategy)
	if err != nil {
		return "", err
	}
	err = applyOpts.Run()
	if err != nil {
		return "", errors.New(cleanKubectlOutput(err.Error()))
	}
	if buf := strings.TrimSpace(ioStreams.Out.(*bytes.Buffer).String()); len(buf) > 0 {
		out = append(out, buf)
	}
	if buf := strings.TrimSpace(ioStreams.ErrOut.(*bytes.Buffer).String()); len(buf) > 0 {
		out = append(out, buf)
	}
	return strings.Join(out, ". "), nil
}

func kubeCmdFactory(kubeconfig, ns string) (cmdutil.Factory, genericclioptions.IOStreams) {
	kubeConfigFlags := genericclioptions.NewConfigFlags(true)
	if ns != "" {
		kubeConfigFlags.Namespace = &ns
	}
	kubeConfigFlags.KubeConfig = &kubeconfig
	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)
	f := cmdutil.NewFactory(matchVersionKubeConfigFlags)
	ioStreams := genericclioptions.IOStreams{
		In:     &bytes.Buffer{},
		Out:    &bytes.Buffer{},
		ErrOut: &bytes.Buffer{},
	}
	return f, ioStreams
}

func newApplyOptions(config *rest.Config, f cmdutil.Factory, ioStreams genericclioptions.IOStreams, fileName string, namespace string, validate bool, force bool, dryRunStrategy cmdutil.DryRunStrategy) (*apply.ApplyOptions, error) {
	o := apply.NewApplyOptions(ioStreams)
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	o.DynamicClient = dynamicClient
	o.DeleteOptions = o.DeleteFlags.ToOptions(dynamicClient, o.IOStreams)
	o.OpenAPISchema, _ = f.OpenAPISchema()
	o.Validator, err = f.Validator(validate)
	if err != nil {
		return nil, err
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}
	o.DryRunVerifier = resource.NewDryRunVerifier(dynamicClient, discoveryClient)
	o.Builder = f.NewBuilder()
	o.Mapper, err = f.ToRESTMapper()
	if err != nil {
		return nil, err
	}

	o.ToPrinter = func(operation string) (printers.ResourcePrinter, error) {
		o.PrintFlags.NamePrintFlags.Operation = operation
		switch o.DryRunStrategy {
		case cmdutil.DryRunClient:
			err = o.PrintFlags.Complete("%s (dry run)")
			if err != nil {
				return nil, err
			}
		case cmdutil.DryRunServer:
			err = o.PrintFlags.Complete("%s (server dry run)")
			if err != nil {
				return nil, err
			}
		}
		return o.PrintFlags.ToPrinter()
	}
	o.DeleteOptions.FilenameOptions.Filenames = []string{fileName}
	o.Namespace = namespace
	o.DeleteOptions.ForceDeletion = force
	o.DryRunStrategy = dryRunStrategy
	return o, nil
}

func newReconcileOptions(f cmdutil.Factory, kubeClient *kubernetes.Clientset, fileName string, ioStreams genericclioptions.IOStreams, namespace string, dryRunStrategy cmdutil.DryRunStrategy) (*auth.ReconcileOptions, error) {
	o := auth.NewReconcileOptions(ioStreams)
	o.RBACClient = kubeClient.RbacV1()
	o.NamespaceClient = kubeClient.CoreV1()
	o.FilenameOptions.Filenames = []string{fileName}
	o.DryRun = dryRunStrategy != cmdutil.DryRunNone

	r := f.NewBuilder().
		WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
		NamespaceParam(namespace).DefaultNamespace().
		FilenameParam(false, o.FilenameOptions).
		Flatten().
		Do()
	o.Visitor = r

	if o.DryRun {
		err := o.PrintFlags.Complete("%s (dry run)")
		if err != nil {
			return nil, err
		}
	}
	printer, err := o.PrintFlags.ToPrinter()
	if err != nil {
		return nil, err
	}
	o.PrintObject = printer.PrintObj
	return o, nil
}

func (k *KubectlCmd) authReconcile(ctx context.Context, config *rest.Config, kubeconfigPath string, manifestFile string, namespace string, dryRunStrategy cmdutil.DryRunStrategy) (string, error) {
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", err
	}
	// `kubectl auth reconcile` has a side effect of auto-creating namespaces if it doesn't exist.
	// See: https://github.com/kubernetes/kubernetes/issues/71185. This is behavior which we do
	// not want. We need to check if the namespace exists, before know if it is safe to run this
	// command. Skip this for dryRuns.
	if dryRunStrategy == cmdutil.DryRunNone && namespace != "" {
		_, err = kubeClient.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
	}
	fact, ioStreams := kubeCmdFactory(kubeconfigPath, namespace)
	reconcileOpts, err := newReconcileOptions(fact, kubeClient, manifestFile, ioStreams, namespace, dryRunStrategy)
	if err != nil {
		return "", err
	}

	err = reconcileOpts.Validate()
	if err != nil {
		return "", errors.New(cleanKubectlOutput(err.Error()))
	}
	err = reconcileOpts.RunReconcile()
	if err != nil {
		return "", errors.New(cleanKubectlOutput(err.Error()))
	}

	var out []string
	if buf := strings.TrimSpace(ioStreams.Out.(*bytes.Buffer).String()); len(buf) > 0 {
		out = append(out, buf)
	}
	if buf := strings.TrimSpace(ioStreams.ErrOut.(*bytes.Buffer).String()); len(buf) > 0 {
		out = append(out, buf)
	}
	return strings.Join(out, ". "), nil
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

func (k *KubectlCmd) processKubectlRun(cmd string) (CleanupFunc, error) {
	if k.OnKubectlRun != nil {
		return k.OnKubectlRun(cmd)
	}
	return func() {}, nil
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
