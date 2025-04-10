package kube

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
	ApplyResource(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy, force bool, validate bool, serverSideApply bool, manager string) (string, error)
	ReplaceResource(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy, force bool) (string, error)
	CreateResource(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy, validate bool) (string, error)
	UpdateResource(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy) (*unstructured.Unstructured, error)
}

// This is a generic implementation for doing most kubectl operations. Implements the ResourceOperations interface.
type kubectlResourceOperations struct {
	config        *rest.Config
	log           logr.Logger
	tracer        tracing.Tracer
	onKubectlRun  OnKubectlRunFunc
	fact          cmdutil.Factory
	openAPISchema openapi.Resources
}

// This is an implementation specific for doing server-side diff dry runs. Implements the KubeApplier interface.
type kubectlServerSideDiffDryRunApplier struct {
	config        *rest.Config
	log           logr.Logger
	tracer        tracing.Tracer
	onKubectlRun  OnKubectlRunFunc
	fact          cmdutil.Factory
	openAPISchema openapi.Resources
}

type commandExecutor func(ioStreams genericclioptions.IOStreams, fileName string) error

func maybeLogManifest(manifestBytes []byte, log logr.Logger) error {
	// log manifest
	if log.V(1).Enabled() {
		var obj unstructured.Unstructured
		err := json.Unmarshal(manifestBytes, &obj)
		if err != nil {
			return err
		}
		redacted, _, err := diff.HideSecretData(&obj, nil, nil)
		if err != nil {
			return err
		}
		redactedBytes, err := json.Marshal(redacted)
		if err != nil {
			return err
		}
		log.V(1).Info(string(redactedBytes))
	}
	return nil
}

func createManifestFile(obj *unstructured.Unstructured, log logr.Logger) (*os.File, error) {
	manifestBytes, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	manifestFile, err := os.CreateTemp(io.TempDir, "")
	if err != nil {
		return nil, fmt.Errorf("Failed to generate temp file for manifest: %w", err)
	}
	if _, err = manifestFile.Write(manifestBytes); err != nil {
		return nil, fmt.Errorf("Failed to write manifest: %w", err)
	}
	if err = manifestFile.Close(); err != nil {
		return nil, fmt.Errorf("Failed to close manifest: %w", err)
	}

	err = maybeLogManifest(manifestBytes, log)
	if err != nil {
		return nil, err
	}
	return manifestFile, nil
}

func (k *kubectlResourceOperations) runResourceCommand(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy, executor commandExecutor) (string, error) {
	manifestFile, err := createManifestFile(obj, k.log)
	if err != nil {
		return "", err
	}
	defer io.DeleteFile(manifestFile.Name())

	var out []string
	// rbac resouces are first applied with auth reconcile kubectl feature.
	if obj.GetAPIVersion() == "rbac.authorization.k8s.io/v1" {
		outReconcile, err := k.rbacReconcile(ctx, obj, manifestFile.Name(), dryRunStrategy)
		if err != nil {
			return "", fmt.Errorf("error running rbacReconcile: %w", err)
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
	err = executor(ioStreams, manifestFile.Name())
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

func (k *kubectlServerSideDiffDryRunApplier) runResourceCommand(obj *unstructured.Unstructured, executor commandExecutor) (string, error) {
	manifestFile, err := createManifestFile(obj, k.log)
	if err != nil {
		return "", err
	}
	defer io.DeleteFile(manifestFile.Name())

	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}

	// Run kubectl apply
	ioStreams := genericclioptions.IOStreams{
		In:     &bytes.Buffer{},
		Out:    stdoutBuf,
		ErrOut: stderrBuf,
	}
	err = executor(ioStreams, manifestFile.Name())
	if err != nil {
		return "", errors.New(cleanKubectlOutput(err.Error()))
	}
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	if stderr != "" && stdout == "" {
		err := fmt.Errorf("Server-side dry run apply had non-empty stderr: %s", stderr)
		k.log.Error(err, "server-side diff")
		return "", err
	}
	if stderr != "" {
		k.log.Info("Warning: Server-side dry run apply had non-empty stderr: %s", stderr)
	}
	return stdout, nil
}

// rbacReconcile will perform reconciliation for RBAC resources. It will run
// the following command:
//
//	kubectl auth reconcile
//
// This is preferred over `kubectl apply`, which cannot tolerate changes in
// roleRef, which is an immutable field.
// See: https://github.com/kubernetes/kubernetes/issues/66353
// `auth reconcile` will delete and recreate the resource if necessary
func (k *kubectlResourceOperations) rbacReconcile(ctx context.Context, obj *unstructured.Unstructured, fileName string, dryRunStrategy cmdutil.DryRunStrategy) (string, error) {
	cleanup, err := processKubectlRun(k.onKubectlRun, "auth")
	if err != nil {
		return "", fmt.Errorf("error processing kubectl run auth: %w", err)
	}
	defer cleanup()
	outReconcile, err := k.authReconcile(ctx, obj, fileName, dryRunStrategy)
	if err != nil {
		return "", fmt.Errorf("error running kubectl auth reconcile: %w", err)
	}
	return outReconcile, nil
}

func kubeCmdFactory(kubeconfig, ns string, config *rest.Config) cmdutil.Factory {
	kubeConfigFlags := genericclioptions.NewConfigFlags(true)
	if ns != "" {
		kubeConfigFlags.Namespace = &ns
	}
	kubeConfigFlags.KubeConfig = &kubeconfig
	kubeConfigFlags.WithDiscoveryBurst(config.Burst)
	kubeConfigFlags.WithDiscoveryQPS(config.QPS)
	kubeConfigFlags.Impersonate = &config.Impersonate.UserName
	kubeConfigFlags.ImpersonateUID = &config.Impersonate.UID
	kubeConfigFlags.ImpersonateGroup = &config.Impersonate.Groups
	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)
	return cmdutil.NewFactory(matchVersionKubeConfigFlags)
}

func (k *kubectlResourceOperations) ReplaceResource(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy, force bool) (string, error) {
	span := k.tracer.StartSpan("ReplaceResource")
	span.SetBaggageItem("kind", obj.GetKind())
	span.SetBaggageItem("name", obj.GetName())
	defer span.Finish()
	k.log.Info(fmt.Sprintf("Replacing resource %s/%s in cluster: %s, namespace: %s", obj.GetKind(), obj.GetName(), k.config.Host, obj.GetNamespace()))
	return k.runResourceCommand(ctx, obj, dryRunStrategy, func(ioStreams genericclioptions.IOStreams, fileName string) error {
		cleanup, err := processKubectlRun(k.onKubectlRun, "replace")
		if err != nil {
			return err
		}
		defer cleanup()

		replaceOptions, err := k.newReplaceOptions(k.config, k.fact, ioStreams, fileName, obj.GetNamespace(), force, dryRunStrategy)
		if err != nil {
			return err
		}
		return replaceOptions.Run(k.fact)
	})
}

func (k *kubectlResourceOperations) CreateResource(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy, validate bool) (string, error) {
	gvk := obj.GroupVersionKind()
	span := k.tracer.StartSpan("CreateResource")
	span.SetBaggageItem("kind", gvk.Kind)
	span.SetBaggageItem("name", obj.GetName())
	defer span.Finish()
	return k.runResourceCommand(ctx, obj, dryRunStrategy, func(ioStreams genericclioptions.IOStreams, fileName string) error {
		cleanup, err := processKubectlRun(k.onKubectlRun, "create")
		if err != nil {
			return err
		}
		defer cleanup()

		createOptions, err := k.newCreateOptions(ioStreams, fileName, dryRunStrategy)
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

		return createOptions.RunCreate(k.fact, command)
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
	apiResource, err := ServerResourceForGroupVersionKind(disco, gvk, "update")
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
func (k *kubectlServerSideDiffDryRunApplier) ApplyResource(_ context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy, force bool, validate bool, serverSideApply bool, manager string) (string, error) {
	span := k.tracer.StartSpan("ApplyResource")
	span.SetBaggageItem("kind", obj.GetKind())
	span.SetBaggageItem("name", obj.GetName())
	defer span.Finish()
	k.log.V(1).WithValues(
		"dry-run", [...]string{"none", "client", "server"}[dryRunStrategy],
		"manager", manager,
		"serverSideApply", serverSideApply).Info(fmt.Sprintf("Running server-side diff. Dry run applying resource %s/%s in cluster: %s, namespace: %s", obj.GetKind(), obj.GetName(), k.config.Host, obj.GetNamespace()))

	return k.runResourceCommand(obj, func(ioStreams genericclioptions.IOStreams, fileName string) error {
		cleanup, err := processKubectlRun(k.onKubectlRun, "apply")
		if err != nil {
			return err
		}
		defer cleanup()

		applyOpts, err := k.newApplyOptions(ioStreams, obj, fileName, validate, force, serverSideApply, dryRunStrategy, manager)
		if err != nil {
			return err
		}
		return applyOpts.Run()
	})
}

// ApplyResource performs an apply of a unstructured resource
func (k *kubectlResourceOperations) ApplyResource(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy, force, validate, serverSideApply bool, manager string) (string, error) {
	span := k.tracer.StartSpan("ApplyResource")
	span.SetBaggageItem("kind", obj.GetKind())
	span.SetBaggageItem("name", obj.GetName())
	defer span.Finish()
	logWithLevel := k.log
	if dryRunStrategy != cmdutil.DryRunNone {
		logWithLevel = logWithLevel.V(1)
	}
	logWithLevel.WithValues(
		"dry-run", [...]string{"none", "client", "server"}[dryRunStrategy],
		"manager", manager,
		"serverSideApply", serverSideApply,
		"serverSideDiff", true).Info(fmt.Sprintf("Applying resource %s/%s in cluster: %s, namespace: %s", obj.GetKind(), obj.GetName(), k.config.Host, obj.GetNamespace()))

	return k.runResourceCommand(ctx, obj, dryRunStrategy, func(ioStreams genericclioptions.IOStreams, fileName string) error {
		cleanup, err := processKubectlRun(k.onKubectlRun, "apply")
		if err != nil {
			return err
		}
		defer cleanup()

		applyOpts, err := k.newApplyOptions(ioStreams, obj, fileName, validate, force, serverSideApply, dryRunStrategy, manager)
		if err != nil {
			return err
		}
		return applyOpts.Run()
	})
}

func newApplyOptionsCommon(config *rest.Config, fact cmdutil.Factory, ioStreams genericclioptions.IOStreams, obj *unstructured.Unstructured, fileName string, validate bool, force bool, serverSideApply bool, dryRunStrategy cmdutil.DryRunStrategy, manager string) (*apply.ApplyOptions, error) {
	flags := apply.NewApplyFlags(ioStreams)
	o := &apply.ApplyOptions{
		IOStreams:         ioStreams,
		VisitedUids:       sets.Set[types.UID]{},
		VisitedNamespaces: sets.Set[string]{},
		Recorder:          genericclioptions.NoopRecorder{},
		PrintFlags:        flags.PrintFlags,
		Overwrite:         true,
		OpenAPIPatch:      true,
		ServerSideApply:   serverSideApply,
	}
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	o.DynamicClient = dynamicClient
	o.DeleteOptions, err = delete.NewDeleteFlags("").ToOptions(dynamicClient, ioStreams)
	if err != nil {
		return nil, err
	}
	o.OpenAPIGetter = fact
	o.DryRunStrategy = dryRunStrategy
	o.FieldManager = manager
	validateDirective := metav1.FieldValidationIgnore
	if validate {
		validateDirective = metav1.FieldValidationStrict
	}
	o.Validator, err = fact.Validator(validateDirective)
	if err != nil {
		return nil, err
	}
	o.Builder = fact.NewBuilder()
	o.Mapper, err = fact.ToRESTMapper()
	if err != nil {
		return nil, err
	}

	o.DeleteOptions.FilenameOptions.Filenames = []string{fileName}
	o.Namespace = obj.GetNamespace()
	o.DeleteOptions.ForceDeletion = force
	o.DryRunStrategy = dryRunStrategy
	if manager != "" {
		o.FieldManager = manager
	}
	return o, nil
}

func (k *kubectlServerSideDiffDryRunApplier) newApplyOptions(ioStreams genericclioptions.IOStreams, obj *unstructured.Unstructured, fileName string, validate bool, force, serverSideApply bool, dryRunStrategy cmdutil.DryRunStrategy, manager string) (*apply.ApplyOptions, error) {
	o, err := newApplyOptionsCommon(k.config, k.fact, ioStreams, obj, fileName, validate, force, serverSideApply, dryRunStrategy, manager)
	if err != nil {
		return nil, err
	}

	o.ToPrinter = func(operation string) (printers.ResourcePrinter, error) {
		o.PrintFlags.NamePrintFlags.Operation = operation
		if o.DryRunStrategy != cmdutil.DryRunServer {
			return nil, fmt.Errorf("invalid dry run strategy passed to server-side diff dry run applier: %d, expected %d", o.DryRunStrategy, cmdutil.DryRunServer)
		}
		// managedFields are required by server-side diff to identify
		// changes made by mutation webhooks.
		o.PrintFlags.JSONYamlPrintFlags.ShowManagedFields = true
		p, err := o.PrintFlags.JSONYamlPrintFlags.ToPrinter("json")
		if err != nil {
			return nil, fmt.Errorf("error configuring server-side diff printer: %w", err)
		}
		return p, nil
	}

	o.ForceConflicts = true
	return o, nil
}

func (k *kubectlResourceOperations) newApplyOptions(ioStreams genericclioptions.IOStreams, obj *unstructured.Unstructured, fileName string, validate bool, force, serverSideApply bool, dryRunStrategy cmdutil.DryRunStrategy, manager string) (*apply.ApplyOptions, error) {
	o, err := newApplyOptionsCommon(k.config, k.fact, ioStreams, obj, fileName, validate, force, serverSideApply, dryRunStrategy, manager)
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
				return nil, fmt.Errorf("error configuring server dryrun printer: %w", err)
			}
		}
		return o.PrintFlags.ToPrinter()
	}

	if serverSideApply {
		o.ForceConflicts = true
	}
	return o, nil
}

func (k *kubectlResourceOperations) newCreateOptions(ioStreams genericclioptions.IOStreams, fileName string, dryRunStrategy cmdutil.DryRunStrategy) (*create.CreateOptions, error) {
	o := create.NewCreateOptions(ioStreams)

	recorder, err := o.RecordFlags.ToRecorder()
	if err != nil {
		return nil, err
	}
	o.Recorder = recorder

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

func (k *kubectlResourceOperations) newReplaceOptions(config *rest.Config, f cmdutil.Factory, ioStreams genericclioptions.IOStreams, fileName string, namespace string, force bool, dryRunStrategy cmdutil.DryRunStrategy) (*replace.ReplaceOptions, error) {
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
		return "", fmt.Errorf("error calling newReconcileOptions: %w", err)
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

func processKubectlRun(onKubectlRun OnKubectlRunFunc, cmd string) (CleanupFunc, error) {
	if onKubectlRun != nil {
		return onKubectlRun(cmd)
	}
	return func() {}, nil
}
