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
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/apply"
	"k8s.io/kubectl/pkg/cmd/auth"
	"k8s.io/kubectl/pkg/cmd/create"
	"k8s.io/kubectl/pkg/cmd/delete"
	"k8s.io/kubectl/pkg/cmd/replace"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/kubectl/pkg/util/openapi"

	"github.com/argoproj/gitops-engine/pkg/diff"
	"github.com/argoproj/gitops-engine/pkg/utils/io"
	"github.com/argoproj/gitops-engine/pkg/utils/tracing"
)

// ResourceOperations provides methods to manage k8s resources
type ResourceOperations interface {
	ApplyResource(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy, force, validate, serverSideApply bool) (string, error)
	ReplaceResource(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy, force bool) (string, error)
	CreateResource(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy, validate bool) (string, error)
	UpdateResource(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy) (*unstructured.Unstructured, error)
}

type kubectlResourceOperations struct {
	config        *rest.Config
	log           logr.Logger
	tracer        tracing.Tracer
	onKubectlRun  OnKubectlRunFunc
	fact          cmdutil.Factory
	openAPISchema openapi.Resources
}

type commandExecutor func(f cmdutil.Factory, ioStreams genericclioptions.IOStreams, fileName string) error

func (k *kubectlResourceOperations) runResourceCommand(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy, executor commandExecutor) (string, error) {
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
	if k.log.V(1).Enabled() {
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
		k.log.V(1).Info(string(redactedBytes))
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
			return k.authReconcile(ctx, obj, manifestFile.Name(), dryRunStrategy)
		}()
		if err != nil {
			return "", err
		}
		out = append(out, outReconcile)
		// We still want to fallthrough and run `kubectl apply` in order set the
		// last-applied-configuration annotation in the object.
	}

	// Run kubectl apply
	ioStreams := genericclioptions.IOStreams{
		In:     &bytes.Buffer{},
		Out:    &bytes.Buffer{},
		ErrOut: &bytes.Buffer{},
	}
	err = executor(k.fact, ioStreams, manifestFile.Name())
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

func kubeCmdFactory(kubeconfig, ns string, config *rest.Config) cmdutil.Factory {
	kubeConfigFlags := genericclioptions.NewConfigFlags(true)
	if ns != "" {
		kubeConfigFlags.Namespace = &ns
	}
	kubeConfigFlags.KubeConfig = &kubeconfig
	kubeConfigFlags.WithDiscoveryBurst(config.Burst)
	kubeConfigFlags.WithDiscoveryQPS(config.QPS)
	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)
	return cmdutil.NewFactory(matchVersionKubeConfigFlags)
}

func (k *kubectlResourceOperations) ReplaceResource(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy, force bool) (string, error) {
	span := k.tracer.StartSpan("ReplaceResource")
	span.SetBaggageItem("kind", obj.GetKind())
	span.SetBaggageItem("name", obj.GetName())
	defer span.Finish()
	k.log.Info(fmt.Sprintf("Replacing resource %s/%s in cluster: %s, namespace: %s", obj.GetKind(), obj.GetName(), k.config.Host, obj.GetNamespace()))
	return k.runResourceCommand(ctx, obj, dryRunStrategy, func(f cmdutil.Factory, ioStreams genericclioptions.IOStreams, fileName string) error {
		cleanup, err := k.processKubectlRun("replace")
		if err != nil {
			return err
		}
		defer cleanup()

		replaceOptions, err := newReplaceOptions(k.config, f, ioStreams, fileName, obj.GetNamespace(), force, dryRunStrategy)
		if err != nil {
			return err
		}
		return replaceOptions.Run(f)
	})
}

func (k *kubectlResourceOperations) CreateResource(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy, validate bool) (string, error) {
	gvk := obj.GroupVersionKind()
	span := k.tracer.StartSpan("CreateResource")
	span.SetBaggageItem("kind", gvk.Kind)
	span.SetBaggageItem("name", obj.GetName())
	defer span.Finish()
	return k.runResourceCommand(ctx, obj, dryRunStrategy, func(f cmdutil.Factory, ioStreams genericclioptions.IOStreams, fileName string) error {
		cleanup, err := k.processKubectlRun("create")
		if err != nil {
			return err
		}
		defer cleanup()

		createOptions, err := newCreateOptions(k.config, ioStreams, fileName, dryRunStrategy)
		if err != nil {
			return err
		}
		command := &cobra.Command{}
		saveConfig := false
		command.Flags().BoolVar(&saveConfig, "save-config", false, "")
		val := false
		command.Flags().BoolVar(&val, "validate", false, "")
		if validate {
			_ = command.Flags().Set("validate", "true")
		}

		return createOptions.RunCreate(f, command)
	})
}

func (k *kubectlResourceOperations) UpdateResource(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy) (*unstructured.Unstructured, error) {
	gvk := obj.GroupVersionKind()
	span := k.tracer.StartSpan("UpdateResource")
	span.SetBaggageItem("kind", gvk.Kind)
	span.SetBaggageItem("name", obj.GetName())
	defer span.Finish()
	dynamicIf, err := dynamic.NewForConfig(k.config)
	if err != nil {
		return nil, err
	}
	disco, err := discovery.NewDiscoveryClientForConfig(k.config)
	if err != nil {
		return nil, err
	}
	apiResource, err := ServerResourceForGroupVersionKind(disco, gvk)
	if err != nil {
		return nil, err
	}
	resource := gvk.GroupVersion().WithResource(apiResource.Name)
	resourceIf := ToResourceInterface(dynamicIf, apiResource, resource, obj.GetNamespace())

	updateOptions := metav1.UpdateOptions{}
	switch dryRunStrategy {
	case cmdutil.DryRunClient, cmdutil.DryRunServer:
		updateOptions.DryRun = []string{metav1.DryRunAll}
	}
	return resourceIf.Update(ctx, obj, updateOptions)
}

// ApplyResource performs an apply of a unstructured resource
func (k *kubectlResourceOperations) ApplyResource(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy, force, validate, serverSideApply bool) (string, error) {
	span := k.tracer.StartSpan("ApplyResource")
	span.SetBaggageItem("kind", obj.GetKind())
	span.SetBaggageItem("name", obj.GetName())
	defer span.Finish()
	k.log.Info(fmt.Sprintf("Applying resource %s/%s in cluster: %s, namespace: %s", obj.GetKind(), obj.GetName(), k.config.Host, obj.GetNamespace()))
	return k.runResourceCommand(ctx, obj, dryRunStrategy, func(f cmdutil.Factory, ioStreams genericclioptions.IOStreams, fileName string) error {
		cleanup, err := k.processKubectlRun("apply")
		if err != nil {
			return err
		}
		defer cleanup()

		applyOpts, err := k.newApplyOptions(ioStreams, obj, fileName, validate, force, serverSideApply, dryRunStrategy)
		if err != nil {
			return err
		}
		return applyOpts.Run()
	})
}

func (k *kubectlResourceOperations) newApplyOptions(ioStreams genericclioptions.IOStreams, obj *unstructured.Unstructured, fileName string, validate bool, force, serverSideApply bool, dryRunStrategy cmdutil.DryRunStrategy) (*apply.ApplyOptions, error) {
	flags := apply.NewApplyFlags(k.fact, ioStreams)
	o := &apply.ApplyOptions{
		IOStreams:         ioStreams,
		VisitedUids:       sets.NewString(),
		VisitedNamespaces: sets.NewString(),
		Recorder:          genericclioptions.NoopRecorder{},
		PrintFlags:        flags.PrintFlags,
		Overwrite:         true,
		OpenAPIPatch:      true,
		ServerSideApply:   serverSideApply,
	}
	dynamicClient, err := dynamic.NewForConfig(k.config)
	if err != nil {
		return nil, err
	}
	o.DynamicClient = dynamicClient
	o.DeleteOptions, err = delete.NewDeleteFlags("").ToOptions(dynamicClient, ioStreams)
	if err != nil {
		return nil, err
	}
	o.OpenAPISchema = k.openAPISchema
	o.Validator, err = k.fact.Validator(validate)
	if err != nil {
		return nil, err
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(k.config)
	if err != nil {
		return nil, err
	}
	o.DryRunVerifier = resource.NewDryRunVerifier(dynamicClient, discoveryClient)
	o.Builder = k.fact.NewBuilder()
	o.Mapper, err = k.fact.ToRESTMapper()
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
	o.Namespace = obj.GetNamespace()
	o.DeleteOptions.ForceDeletion = force
	o.DryRunStrategy = dryRunStrategy
	return o, nil
}

func newCreateOptions(config *rest.Config, ioStreams genericclioptions.IOStreams, fileName string, dryRunStrategy cmdutil.DryRunStrategy) (*create.CreateOptions, error) {
	o := create.NewCreateOptions(ioStreams)

	recorder, err := o.RecordFlags.ToRecorder()
	if err != nil {
		return nil, err
	}
	o.Recorder = recorder

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}
	o.DryRunVerifier = resource.NewDryRunVerifier(dynamicClient, discoveryClient)

	switch dryRunStrategy {
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
	o.DryRunStrategy = dryRunStrategy

	printer, err := o.PrintFlags.ToPrinter()
	if err != nil {
		return nil, err
	}
	o.PrintObj = func(obj runtime.Object) error {
		return printer.PrintObj(obj, o.Out)
	}
	o.FilenameOptions.Filenames = []string{fileName}
	return o, nil
}

func newReplaceOptions(config *rest.Config, f cmdutil.Factory, ioStreams genericclioptions.IOStreams, fileName string, namespace string, force bool, dryRunStrategy cmdutil.DryRunStrategy) (*replace.ReplaceOptions, error) {
	o := replace.NewReplaceOptions(ioStreams)

	recorder, err := o.RecordFlags.ToRecorder()
	if err != nil {
		return nil, err
	}
	o.Recorder = recorder

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	o.DeleteOptions, err = o.DeleteFlags.ToOptions(dynamicClient, o.IOStreams)
	if err != nil {
		return nil, err
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}
	o.DryRunVerifier = resource.NewDryRunVerifier(dynamicClient, discoveryClient)
	o.Builder = func() *resource.Builder {
		return f.NewBuilder()
	}

	switch dryRunStrategy {
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
	o.DryRunStrategy = dryRunStrategy

	printer, err := o.PrintFlags.ToPrinter()
	if err != nil {
		return nil, err
	}
	o.PrintObj = func(obj runtime.Object) error {
		return printer.PrintObj(obj, o.Out)
	}

	o.DeleteOptions.FilenameOptions.Filenames = []string{fileName}
	o.Namespace = namespace
	o.DeleteOptions.ForceDeletion = force
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

func (k *kubectlResourceOperations) authReconcile(ctx context.Context, obj *unstructured.Unstructured, manifestFile string, dryRunStrategy cmdutil.DryRunStrategy) (string, error) {
	kubeClient, err := kubernetes.NewForConfig(k.config)
	if err != nil {
		return "", err
	}
	// `kubectl auth reconcile` has a side effect of auto-creating namespaces if it doesn't exist.
	// See: https://github.com/kubernetes/kubernetes/issues/71185. This is behavior which we do
	// not want. We need to check if the namespace exists, before know if it is safe to run this
	// command. Skip this for dryRuns.
	if dryRunStrategy == cmdutil.DryRunNone && obj.GetNamespace() != "" {
		_, err = kubeClient.CoreV1().Namespaces().Get(ctx, obj.GetNamespace(), metav1.GetOptions{})
		if err != nil {
			return "", err
		}
	}
	ioStreams := genericclioptions.IOStreams{
		In:     &bytes.Buffer{},
		Out:    &bytes.Buffer{},
		ErrOut: &bytes.Buffer{},
	}
	reconcileOpts, err := newReconcileOptions(k.fact, kubeClient, manifestFile, ioStreams, obj.GetNamespace(), dryRunStrategy)
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

func (k *kubectlResourceOperations) processKubectlRun(cmd string) (CleanupFunc, error) {
	if k.onKubectlRun != nil {
		return k.onKubectlRun(cmd)
	}
	return func() {}, nil
}
