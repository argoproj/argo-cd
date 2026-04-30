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
	"k8s.io/cli-runtime/pkg/genericiooptions"
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

	"github.com/argoproj/argo-cd/gitops-engine/pkg/diff"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/io"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/tracing"
)

type outputMode int

const (
	outputModeLog  outputMode = iota // Return log messages (normal apply)
	outputModeJSON                   // Return JSON object (server-side diff)
)

// ResourceOperations provides methods to manage k8s resources
type ResourceOperations interface {
	ApplyResource(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy, force, validate, serverSideApply bool, manager string) (string, error)
	ReplaceResource(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy, force bool) (string, error)
	CreateResource(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy, validate bool) (string, error)
	UpdateResource(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy) (*unstructured.Unstructured, error)
}

// kubectlCommandFacade abstracts the execution of kubectl command options
// Handles the different Run() method signatures and kubectl execution hooks
type KubectlCommandFacade interface {
	Apply(opts *apply.ApplyOptions) error
	Create(opts *create.CreateOptions, fact cmdutil.Factory, cmd *cobra.Command) error
	Replace(opts *replace.ReplaceOptions, fact cmdutil.Factory) error
	AuthReconcile(opts *auth.ReconcileOptions) error
}

// realKubectlCommandFacade is the production implementation of kubectlCommandFacade
type realKubectlCommandFacade struct {
	onKubectlRun OnKubectlRunFunc
}

func (f *realKubectlCommandFacade) Apply(opts *apply.ApplyOptions) error {
	cleanup, err := f.processKubectlRun("apply")
	if err != nil {
		return err
	}
	defer cleanup()
	return opts.Run()
}

func (f *realKubectlCommandFacade) Create(opts *create.CreateOptions, fact cmdutil.Factory, cmd *cobra.Command) error {
	cleanup, err := f.processKubectlRun("create")
	if err != nil {
		return err
	}
	defer cleanup()
	return opts.RunCreate(fact, cmd)
}

func (f *realKubectlCommandFacade) Replace(opts *replace.ReplaceOptions, fact cmdutil.Factory) error {
	cleanup, err := f.processKubectlRun("replace")
	if err != nil {
		return err
	}
	defer cleanup()
	return opts.Run(fact)
}

func (f *realKubectlCommandFacade) AuthReconcile(opts *auth.ReconcileOptions) error {
	cleanup, err := f.processKubectlRun("auth")
	if err != nil {
		return err
	}
	defer cleanup()
	return opts.RunReconcile()
}

func (f *realKubectlCommandFacade) processKubectlRun(cmd string) (CleanupFunc, error) {
	if f.onKubectlRun != nil {
		return f.onKubectlRun(cmd)
	}
	return func() {}, nil
}

// This is a generic implementation for doing most kubectl operations. Implements the ResourceOperations interface.
type kubectlResourceOperations struct {
	config        *rest.Config
	getClientFunc func() (kubernetes.Interface, error)
	log           logr.Logger
	tracer        tracing.Tracer
	fact          cmdutil.Factory
	commandFacade KubectlCommandFacade
	outputMode    outputMode
}

type commandExecutor func(ioStreams genericiooptions.IOStreams, fileName string) error

func maybeLogManifest(manifestBytes []byte, log logr.Logger) error {
	// log manifest
	if log.V(1).Enabled() {
		var obj unstructured.Unstructured
		err := json.Unmarshal(manifestBytes, &obj)
		if err != nil {
			return fmt.Errorf("failed to unmarshal object: %w", err)
		}
		redacted, _, err := diff.HideSecretData(&obj, nil, nil)
		if err != nil {
			return fmt.Errorf("failed to hide secret data: %w", err)
		}
		redactedBytes, err := json.Marshal(redacted)
		if err != nil {
			return fmt.Errorf("failed to marshal redacted object: %w", err)
		}
		log.V(1).Info(string(redactedBytes))
	}
	return nil
}

func createManifestFile(obj *unstructured.Unstructured, log logr.Logger) (*os.File, error) {
	manifestBytes, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal object: %w", err)
	}
	manifestFile, err := os.CreateTemp(io.TempDir, "")
	if err != nil {
		return nil, fmt.Errorf("failed to generate temp file for manifest: %w", err)
	}
	if _, err = manifestFile.Write(manifestBytes); err != nil {
		return nil, fmt.Errorf("failed to write manifest: %w", err)
	}
	if err = manifestFile.Close(); err != nil {
		return nil, fmt.Errorf("failed to close manifest: %w", err)
	}

	err = maybeLogManifest(manifestBytes, log)
	if err != nil {
		return nil, err
	}
	return manifestFile, nil
}

func (k *kubectlResourceOperations) handleJSONOutput(stdout, stderr string) (string, error) {
	if stderr != "" && stdout == "" {
		err := fmt.Errorf("command output had non-empty stderr: %s", stderr)
		k.log.Error(err, "error running command")
		return "", err
	}
	if stderr != "" {
		k.log.Info("Warning: Command output had non-empty stderr: %s", stderr)
	}
	return stdout, nil
}

func (k *kubectlResourceOperations) handleLogOutput(stdout, stderr string) (string, error) {
	var out []string
	if buf := strings.TrimSpace(stdout); buf != "" {
		out = append(out, buf)
	}
	if buf := strings.TrimSpace(stderr); buf != "" {
		out = append(out, buf)
	}
	return strings.Join(out, ". "), nil
}

func (k *kubectlResourceOperations) runResourceCommand(_ context.Context, obj *unstructured.Unstructured, executor commandExecutor) (string, error) {
	manifestFile, err := createManifestFile(obj, k.log)
	if err != nil {
		return "", err
	}
	defer io.DeleteFile(manifestFile.Name())

	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}

	// Run kubectl command
	ioStreams := genericiooptions.IOStreams{
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

	// Delegate to appropriate handler based on output mode
	if k.outputMode == outputModeJSON {
		return k.handleJSONOutput(stdout, stderr)
	}
	return k.handleLogOutput(stdout, stderr)
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
func (k *kubectlResourceOperations) rbacReconcile(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy) (string, error) {
	gvk := obj.GroupVersionKind()
	span := k.tracer.StartSpan("AuthReconcile")
	span.SetBaggageItem("kind", gvk.Kind)
	span.SetBaggageItem("name", obj.GetName())
	defer span.Finish()
	return k.runResourceCommand(ctx, obj, func(ioStreams genericiooptions.IOStreams, fileName string) error {
		kubeClient, err := k.getClientFunc()
		if err != nil {
			return fmt.Errorf("error creating kube client: %w", err)
		}

		// `kubectl auth reconcile` has a side effect of auto-creating namespaces if it doesn't exist.
		// See: https://github.com/kubernetes/kubernetes/issues/71185. This is behavior which we do
		// not want. We need to check if the namespace exists, before know if it is safe to run this
		// command. Skip this for dryRuns.

		// When an Argo CD Application specifies destination.namespace, that namespace
		// may be propagated even for cluster-scoped resources. Passing a namespace in
		// this case causes `kubectl auth reconcile` to fail with:
		//   "namespaces <ns> not found"
		// or may trigger unintended namespace handling behavior.
		// Therefore, we skip namespace existence checks for cluster-scoped RBAC
		// resources and allow reconcile to run without a namespace.
		//
		// https://github.com/argoproj/argo-cd/issues/24833
		clusterScoped := obj.GetKind() == "ClusterRole" || obj.GetKind() == "ClusterRoleBinding"
		if dryRunStrategy == cmdutil.DryRunNone && obj.GetNamespace() != "" && !clusterScoped {
			_, err = kubeClient.CoreV1().Namespaces().Get(ctx, obj.GetNamespace(), metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("error getting namespace %s: %w", obj.GetNamespace(), err)
			}
		}

		authReconcileOptions, err := newReconcileOptions(k.fact, kubeClient, fileName, ioStreams, obj.GetNamespace(), dryRunStrategy)
		if err != nil {
			return err
		}

		return k.commandFacade.AuthReconcile(authReconcileOptions)
	})
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
	return k.runResourceCommand(ctx, obj, func(ioStreams genericiooptions.IOStreams, fileName string) error {
		replaceOptions, err := k.newReplaceOptions(k.config, k.fact, ioStreams, fileName, obj.GetNamespace(), force, dryRunStrategy)
		if err != nil {
			return err
		}

		return k.commandFacade.Replace(replaceOptions, k.fact)
	})
}

func (k *kubectlResourceOperations) CreateResource(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy, validate bool) (string, error) {
	gvk := obj.GroupVersionKind()
	span := k.tracer.StartSpan("CreateResource")
	span.SetBaggageItem("kind", gvk.Kind)
	span.SetBaggageItem("name", obj.GetName())
	defer span.Finish()
	return k.runResourceCommand(ctx, obj, func(ioStreams genericiooptions.IOStreams, fileName string) error {
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

		return k.commandFacade.Create(createOptions, k.fact, command)
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
		return nil, fmt.Errorf("error creating dynamic client for config: %w", err)
	}
	disco, err := discovery.NewDiscoveryClientForConfig(k.config)
	if err != nil {
		return nil, fmt.Errorf("error creating discovery client for config: %w", err)
	}
	apiResource, err := ServerResourceForGroupVersionKind(disco, gvk, "update")
	if err != nil {
		return nil, fmt.Errorf("error creating discovery client for config: %w", err)
	}
	resource := gvk.GroupVersion().WithResource(apiResource.Name)
	resourceIf := ToResourceInterface(dynamicIf, apiResource, resource, obj.GetNamespace())

	updateOptions := metav1.UpdateOptions{}
	switch dryRunStrategy {
	case cmdutil.DryRunClient, cmdutil.DryRunServer:
		updateOptions.DryRun = []string{metav1.DryRunAll}
	}
	//nolint:wrapcheck // wrapped error message would be same as caller's wrapped message
	return resourceIf.Update(ctx, obj, updateOptions)
}

// ApplyResource performs an apply of a unstructured resource
func (k *kubectlResourceOperations) ApplyResource(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy, force, validate, serverSideApply bool, manager string) (string, error) {
	span := k.tracer.StartSpan("ApplyResource")
	span.SetBaggageItem("kind", obj.GetKind())
	span.SetBaggageItem("name", obj.GetName())
	defer span.Finish()

	logWithLevel := k.log.V(0)
	if dryRunStrategy != cmdutil.DryRunNone {
		logWithLevel = logWithLevel.V(1)
	}
	logWithLevel.WithValues(
		"dry-run", [...]string{"none", "client", "server"}[dryRunStrategy],
		"manager", manager,
		"serverSideApply", serverSideApply).Info(fmt.Sprintf("Applying resource %s/%s in cluster: %s, namespace: %s", obj.GetKind(), obj.GetName(), k.config.Host, obj.GetNamespace()))

	// rbac resources are first applied with auth reconcile kubectl feature.
	// This is not supported with server-side apply/diff.
	// Server-side apply correctly handles the RBAC resources.
	var outReconcile string
	if !serverSideApply && obj.GetAPIVersion() == "rbac.authorization.k8s.io/v1" {
		out, err := k.rbacReconcile(ctx, obj, dryRunStrategy)
		if err != nil {
			return "", fmt.Errorf("error running kubectl auth reconcile: %w", err)
		}
		outReconcile = out
	}

	return k.runResourceCommand(ctx, obj, func(ioStreams genericiooptions.IOStreams, fileName string) error {
		applyOpts, err := k.newApplyOptions(ioStreams, obj, fileName, validate, force, serverSideApply, dryRunStrategy, manager)
		if err != nil {
			return err
		}
		_, err = ioStreams.Out.Write([]byte(outReconcile))
		if err != nil {
			return fmt.Errorf("error writing reconcile output to stdout: %w", err)
		}
		return k.commandFacade.Apply(applyOpts)
	})
}

func newApplyOptionsCommon(config *rest.Config, fact cmdutil.Factory, ioStreams genericiooptions.IOStreams, obj *unstructured.Unstructured, fileName string, validate bool, force, serverSideApply bool, dryRunStrategy cmdutil.DryRunStrategy, manager string) (*apply.ApplyOptions, error) {
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
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}
	o.DynamicClient = dynamicClient
	o.DeleteOptions, err = delete.NewDeleteFlags("").ToOptions(dynamicClient, ioStreams)
	if err != nil {
		return nil, fmt.Errorf("failed to create delete flags: %w", err)
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
		return nil, fmt.Errorf("failed to create validator: %w", err)
	}
	o.Builder = fact.NewBuilder()
	o.Mapper, err = fact.ToRESTMapper()
	if err != nil {
		return nil, fmt.Errorf("failed to create restmapper: %w", err)
	}

	o.DeleteOptions.Filenames = []string{fileName}
	o.Namespace = obj.GetNamespace()
	o.DeleteOptions.ForceDeletion = force
	o.DryRunStrategy = dryRunStrategy
	if manager != "" {
		o.FieldManager = manager
	}
	return o, nil
}

func (k *kubectlResourceOperations) newApplyOptions(ioStreams genericiooptions.IOStreams, obj *unstructured.Unstructured, fileName string, validate bool, force, serverSideApply bool, dryRunStrategy cmdutil.DryRunStrategy, manager string) (*apply.ApplyOptions, error) {
	o, err := newApplyOptionsCommon(k.config, k.fact, ioStreams, obj, fileName, validate, force, serverSideApply, dryRunStrategy, manager)
	if err != nil {
		return nil, err
	}

	o.ToPrinter = func(operation string) (printers.ResourcePrinter, error) {
		o.PrintFlags.NamePrintFlags.Operation = operation

		// For server-side diff (outputModeJSON), use JSON printer
		if k.outputMode == outputModeJSON {
			if o.DryRunStrategy != cmdutil.DryRunServer {
				return nil, fmt.Errorf("invalid dry run strategy used with JSON output. : %d, expected %d", o.DryRunStrategy, cmdutil.DryRunServer)
			}
			if !serverSideApply {
				return nil, errors.New("invalid Apply strategy used with JSON output. Must use server-side apply")
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

		// For normal output mode (outputModeLog), use name printer
		switch o.DryRunStrategy {
		case cmdutil.DryRunClient:
			err = o.PrintFlags.Complete("%s (dry run)")
			if err != nil {
				return nil, fmt.Errorf("error configuring client dryrun printer: %w", err)
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

	if err := o.Validate(); err != nil {
		return nil, fmt.Errorf("error validating options: %w", err)
	}
	return o, nil
}

func (k *kubectlResourceOperations) newCreateOptions(ioStreams genericiooptions.IOStreams, fileName string, dryRunStrategy cmdutil.DryRunStrategy) (*create.CreateOptions, error) {
	o := create.NewCreateOptions(ioStreams)

	recorder, err := o.RecordFlags.ToRecorder()
	if err != nil {
		return nil, fmt.Errorf("error configuring recorder: %w", err)
	}
	o.Recorder = recorder

	switch dryRunStrategy {
	case cmdutil.DryRunClient:
		err = o.PrintFlags.Complete("%s (dry run)")
		if err != nil {
			return nil, fmt.Errorf("error configuring client dryrun printer: %w", err)
		}
	case cmdutil.DryRunServer:
		err = o.PrintFlags.Complete("%s (server dry run)")
		if err != nil {
			return nil, fmt.Errorf("error configuring server dryrun printer: %w", err)
		}
	}
	o.DryRunStrategy = dryRunStrategy

	printer, err := o.PrintFlags.ToPrinter()
	if err != nil {
		return nil, fmt.Errorf("error configuring printer: %w", err)
	}
	o.PrintObj = func(obj runtime.Object) error {
		return printer.PrintObj(obj, o.Out)
	}
	o.FilenameOptions.Filenames = []string{fileName}

	if err := o.Validate(); err != nil {
		return nil, fmt.Errorf("error validating options: %w", err)
	}
	return o, nil
}

func (k *kubectlResourceOperations) newReplaceOptions(config *rest.Config, f cmdutil.Factory, ioStreams genericiooptions.IOStreams, fileName string, namespace string, force bool, dryRunStrategy cmdutil.DryRunStrategy) (*replace.ReplaceOptions, error) {
	o := replace.NewReplaceOptions(ioStreams)

	recorder, err := o.RecordFlags.ToRecorder()
	if err != nil {
		return nil, fmt.Errorf("error configuring recorder: %w", err)
	}
	o.Recorder = recorder

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error configuring dynamic client: %w", err)
	}

	o.DeleteOptions, err = o.DeleteFlags.ToOptions(dynamicClient, o.IOStreams)
	if err != nil {
		return nil, fmt.Errorf("error configuring delete: %w", err)
	}

	o.Builder = func() *resource.Builder {
		return f.NewBuilder()
	}

	switch dryRunStrategy {
	case cmdutil.DryRunClient:
		err = o.PrintFlags.Complete("%s (dry run)")
		if err != nil {
			return nil, fmt.Errorf("error configuring client dryrun printer: %w", err)
		}
	case cmdutil.DryRunServer:
		err = o.PrintFlags.Complete("%s (server dry run)")
		if err != nil {
			return nil, fmt.Errorf("error configuring server dryrun printer: %w", err)
		}
	}
	o.DryRunStrategy = dryRunStrategy

	printer, err := o.PrintFlags.ToPrinter()
	if err != nil {
		return nil, fmt.Errorf("error configuring printer: %w", err)
	}
	o.PrintObj = func(obj runtime.Object) error {
		return printer.PrintObj(obj, o.Out)
	}

	o.DeleteOptions.Filenames = []string{fileName}
	o.Namespace = namespace

	if dryRunStrategy == cmdutil.DryRunNone {
		o.DeleteOptions.ForceDeletion = force
	}

	if err := o.Validate(); err != nil {
		return nil, fmt.Errorf("error validating options: %w", err)
	}
	return o, nil
}

func newReconcileOptions(f cmdutil.Factory, kubeClient kubernetes.Interface, fileName string, ioStreams genericiooptions.IOStreams, namespace string, dryRunStrategy cmdutil.DryRunStrategy) (*auth.ReconcileOptions, error) {
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
			return nil, fmt.Errorf("error configuring client dryrun printer: %w", err)
		}
	}
	printer, err := o.PrintFlags.ToPrinter()
	if err != nil {
		return nil, fmt.Errorf("error configuring printer: %w", err)
	}
	o.PrintObject = printer.PrintObj

	if err := o.Validate(); err != nil {
		return nil, fmt.Errorf("error validating options: %w", err)
	}
	return o, nil
}
