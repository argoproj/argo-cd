package kube

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os/exec"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/apply"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/kubernetes/pkg/kubectl/cmd/auth"

	"github.com/argoproj/argo-cd/engine/pkg/utils/diff"
	executil "github.com/argoproj/argo-cd/engine/pkg/utils/exec"
	"github.com/argoproj/argo-cd/engine/pkg/utils/io"
	"github.com/argoproj/argo-cd/engine/pkg/utils/tracing"
)

type Kubectl interface {
	ApplyResource(config *rest.Config, obj *unstructured.Unstructured, namespace string, dryRun, force, validate bool) (string, error)
	ConvertToVersion(obj *unstructured.Unstructured, group, version string) (*unstructured.Unstructured, error)
	DeleteResource(config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string, forceDelete bool) error
	GetResource(config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string) (*unstructured.Unstructured, error)
	PatchResource(config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string, patchType types.PatchType, patchBytes []byte) (*unstructured.Unstructured, error)
	GetAPIResources(config *rest.Config, resourceFilter ResourceFilter) ([]APIResourceInfo, error)
	GetAPIGroups(config *rest.Config) ([]metav1.APIGroup, error)
	GetServerVersion(config *rest.Config) (string, error)
	NewDynamicClient(config *rest.Config) (dynamic.Interface, error)
	SetOnKubectlRun(onKubectlRun func(command string) (io.Closer, error))
}

type KubectlCmd struct {
	OnKubectlRun func(command string) (io.Closer, error)
}

type APIResourceInfo struct {
	GroupKind            schema.GroupKind
	Meta                 metav1.APIResource
	GroupVersionResource schema.GroupVersionResource
}

type filterFunc func(apiResource *metav1.APIResource) bool

func filterAPIResources(config *rest.Config, resourceFilter ResourceFilter, filter filterFunc) ([]APIResourceInfo, error) {
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}

	serverResources, err := disco.ServerPreferredResources()
	if err != nil {
		if len(serverResources) == 0 {
			return nil, err
		}
		log.Warnf("Partial success when performing preferred resource discovery: %v", err)
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
	span := tracing.StartSpan("GetAPIResources")
	defer span.Finish()
	apiResIfs, err := filterAPIResources(config, resourceFilter, func(apiResource *metav1.APIResource) bool {
		return isSupportedVerb(apiResource, listVerb) && isSupportedVerb(apiResource, watchVerb)
	})
	if err != nil {
		return nil, err
	}
	return apiResIfs, err
}

// GetResource returns resource
func (k *KubectlCmd) GetResource(config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string) (*unstructured.Unstructured, error) {
	span := tracing.StartSpan("GetResource")
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
	return resourceIf.Get(name, metav1.GetOptions{})
}

// PatchResource patches resource
func (k *KubectlCmd) PatchResource(config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string, patchType types.PatchType, patchBytes []byte) (*unstructured.Unstructured, error) {
	span := tracing.StartSpan("PatchResource")
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
	return resourceIf.Patch(name, patchType, patchBytes, metav1.PatchOptions{})
}

// DeleteResource deletes resource
func (k *KubectlCmd) DeleteResource(config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string, forceDelete bool) error {
	span := tracing.StartSpan("DeleteResource")
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
	deleteOptions := &metav1.DeleteOptions{PropagationPolicy: &propagationPolicy}
	if forceDelete {
		propagationPolicy = metav1.DeletePropagationBackground
		zeroGracePeriod := int64(0)
		deleteOptions.GracePeriodSeconds = &zeroGracePeriod
	}

	return resourceIf.Delete(name, deleteOptions)
}

// ApplyResource performs an apply of a unstructured resource
func (k *KubectlCmd) ApplyResource(config *rest.Config, obj *unstructured.Unstructured, namespace string, dryRun, force, validate bool) (string, error) {
	span := tracing.StartSpan("ApplyResource")
	span.SetBaggageItem("kind", obj.GetKind())
	span.SetBaggageItem("name", obj.GetName())
	defer span.Finish()
	log.Infof("Applying resource %s/%s in cluster: %s, namespace: %s", obj.GetKind(), obj.GetName(), config.Host, namespace)
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
	if log.IsLevelEnabled(log.DebugLevel) {
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
		log.Debug(string(redactedBytes))
	}

	var out []string
	if obj.GetAPIVersion() == "rbac.authorization.k8s.io/v1" {
		// If it is an RBAC resource, run `kubectl auth reconcile`. This is preferred over
		// `kubectl apply`, which cannot tolerate changes in roleRef, which is an immutable field.
		// See: https://github.com/kubernetes/kubernetes/issues/66353
		// `auth reconcile` will delete and recreate the resource if necessary
		closer, err := k.processKubectlRun("auth")
		if err != nil {
			return "", err
		}
		outReconcile, err := k.authReconcile(config, f.Name(), manifestFile.Name(), namespace, dryRun)
		io.Close(closer)
		if err != nil {
			return "", err
		}
		out = append(out, outReconcile)
		// We still want to fallthrough and run `kubectl apply` in order set the
		// last-applied-configuration annotation in the object.
	}

	closer, err := k.processKubectlRun("apply")
	if err != nil {
		return "", err
	}
	defer io.Close(closer)

	// Run kubectl apply
	fact, ioStreams := kubeCmdFactory(f.Name(), namespace)
	applyOpts, err := newApplyOptions(config, fact, ioStreams, manifestFile.Name(), namespace, validate, force, dryRun)
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

func newApplyOptions(config *rest.Config, f cmdutil.Factory, ioStreams genericclioptions.IOStreams, fileName string, namespace string, validate bool, force bool, dryRun bool) (*apply.ApplyOptions, error) {
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
	o.Builder = f.NewBuilder()
	o.Mapper, err = f.ToRESTMapper()
	if err != nil {
		return nil, err
	}

	o.ToPrinter = func(operation string) (printers.ResourcePrinter, error) {
		o.PrintFlags.NamePrintFlags.Operation = operation
		if o.DryRun {
			err = o.PrintFlags.Complete("%s (dry run)")
			if err != nil {
				return nil, err
			}
		}
		if o.ServerDryRun {
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
	o.DryRun = dryRun
	return o, nil
}

func newReconcileOptions(f cmdutil.Factory, kubeClient *kubernetes.Clientset, fileName string, ioStreams genericclioptions.IOStreams, namespace string, dryRun bool) (*auth.ReconcileOptions, error) {
	o := auth.NewReconcileOptions(ioStreams)
	o.RBACClient = kubeClient.RbacV1()
	o.NamespaceClient = kubeClient.CoreV1()
	o.FilenameOptions.Filenames = []string{fileName}
	o.DryRun = dryRun

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

func (k *KubectlCmd) authReconcile(config *rest.Config, kubeconfigPath string, manifestFile string, namespace string, dryRun bool) (string, error) {
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", err
	}
	// `kubectl auth reconcile` has a side effect of auto-creating namespaces if it doesn't exist.
	// See: https://github.com/kubernetes/kubernetes/issues/71185. This is behavior which we do
	// not want. We need to check if the namespace exists, before know if it is safe to run this
	// command. Skip this for dryRuns.
	if !dryRun && namespace != "" {
		_, err = kubeClient.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
	}
	fact, ioStreams := kubeCmdFactory(kubeconfigPath, namespace)
	reconcileOpts, err := newReconcileOptions(fact, kubeClient, manifestFile, ioStreams, namespace, dryRun)
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

func Version() (string, error) {
	span := tracing.StartSpan("Version")
	defer span.Finish()
	cmd := exec.Command("kubectl", "version", "--client")
	out, err := executil.Run(cmd)
	if err != nil {
		return "", fmt.Errorf("could not get kubectl version: %s", err)
	}
	re := regexp.MustCompile(`GitVersion:"([a-zA-Z0-9\.\-]+)"`)
	matches := re.FindStringSubmatch(out)
	if len(matches) != 2 {
		return "", errors.New("could not get kubectl version")
	}
	version := matches[1]
	if version[0] != 'v' {
		version = "v" + version
	}
	return strings.TrimSpace(version), nil
}

// ConvertToVersion converts an unstructured object into the specified group/version
func (k *KubectlCmd) ConvertToVersion(obj *unstructured.Unstructured, group string, version string) (*unstructured.Unstructured, error) {
	span := tracing.StartSpan("ConvertToVersion")
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
	span := tracing.StartSpan("GetServerVersion")
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

func (k *KubectlCmd) processKubectlRun(cmd string) (io.Closer, error) {
	if k.OnKubectlRun != nil {
		return k.OnKubectlRun(cmd)
	}
	return io.NewCloser(func() error {
		return nil
		// do nothing
	}), nil
}

func (k *KubectlCmd) SetOnKubectlRun(onKubectlRun func(command string) (io.Closer, error)) {
	k.OnKubectlRun = onKubectlRun
}
