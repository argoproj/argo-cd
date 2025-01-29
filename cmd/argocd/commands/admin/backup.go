package admin

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v3/cmd/argocd/commands/utils"
	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application"
	"github.com/argoproj/argo-cd/v3/util/cli"
	"github.com/argoproj/argo-cd/v3/util/errors"
	"github.com/argoproj/argo-cd/v3/util/localconfig"
	secutil "github.com/argoproj/argo-cd/v3/util/security"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
)

// NewExportCommand defines a new command for exporting Kubernetes and Argo CD resources.
func NewExportCommand() *cobra.Command {
	var (
		clientConfig             clientcmd.ClientConfig
		out                      string
		applicationNamespaces    []string
		applicationsetNamespaces []string
	)
	command := cobra.Command{
		Use:   "export",
		Short: "Export all Argo CD data to stdout (default) or a file",
		Run: func(c *cobra.Command, _ []string) {
			ctx := c.Context()

			config, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)
			tt := time.Now()

			fmt.Printf("backup process started %s\n", namespace)

			var writer io.Writer
			if out == "-" {
				writer = os.Stdout
			} else {
				f, err := os.Create(out)
				errors.CheckError(err)
				bw := bufio.NewWriter(f)
				writer = bw
				defer func() {
					err = bw.Flush()
					errors.CheckError(err)
					err = f.Close()
					errors.CheckError(err)
				}()
			}

			acdClients := newArgoCDClientsets(config, namespace)
			acdConfigMap, err := acdClients.configMaps.Get(ctx, common.ArgoCDConfigMapName, metav1.GetOptions{})
			errors.CheckError(err)
			export(writer, *acdConfigMap, namespace)
			acdRBACConfigMap, err := acdClients.configMaps.Get(ctx, common.ArgoCDRBACConfigMapName, metav1.GetOptions{})
			errors.CheckError(err)
			export(writer, *acdRBACConfigMap, namespace)
			acdKnownHostsConfigMap, err := acdClients.configMaps.Get(ctx, common.ArgoCDKnownHostsConfigMapName, metav1.GetOptions{})
			errors.CheckError(err)
			export(writer, *acdKnownHostsConfigMap, namespace)
			acdTLSCertsConfigMap, err := acdClients.configMaps.Get(ctx, common.ArgoCDTLSCertsConfigMapName, metav1.GetOptions{})
			errors.CheckError(err)
			export(writer, *acdTLSCertsConfigMap, namespace)

			referencedSecrets := getReferencedSecrets(*acdConfigMap)
			secrets, err := acdClients.secrets.List(ctx, metav1.ListOptions{})
			errors.CheckError(err)
			for _, secret := range secrets.Items {
				if isArgoCDSecret(referencedSecrets, secret) {
					export(writer, secret, namespace)
				}
			}
			projects, err := acdClients.projects.List(ctx, metav1.ListOptions{})
			errors.CheckError(err)
			for _, proj := range projects.Items {
				export(writer, proj, namespace)
			}

			additionalNamespaces := getAdditionalNamespaces(ctx, acdClients)

			if len(applicationNamespaces) == 0 {
				applicationNamespaces = additionalNamespaces.applicationNamespaces
			}
			if len(applicationsetNamespaces) == 0 {
				applicationsetNamespaces = additionalNamespaces.applicationsetNamespaces
			}

			applications, err := acdClients.applications.List(ctx, metav1.ListOptions{})
			errors.CheckError(err)
			for _, app := range applications.Items {
				// Export application only if it is in one of the enabled namespaces
				if secutil.IsNamespaceEnabled(app.GetNamespace(), namespace, applicationNamespaces) {
					export(writer, app, namespace)
				}
			}
			applicationSets, err := acdClients.applicationSets.List(ctx, metav1.ListOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				if apierrors.IsForbidden(err) {
					log.Warn(err)
				} else {
					errors.CheckError(err)
				}
			}
			if applicationSets != nil {
				for _, appSet := range applicationSets.Items {
					if secutil.IsNamespaceEnabled(appSet.GetNamespace(), namespace, applicationsetNamespaces) {
						export(writer, appSet, namespace)
					}
				}
			}
			duration := time.Since(tt)
			fmt.Printf("backup process completed successfully in namespace %s at %s, duration: %s\n", namespace, time.Now().Format(time.RFC3339), duration)
		},
	}

	clientConfig = cli.AddKubectlFlagsToCmd(&command)
	command.Flags().StringVarP(&out, "out", "o", "-", "Output to the specified file instead of stdout")
	command.Flags().StringSliceVarP(&applicationNamespaces, "application-namespaces", "", []string{}, fmt.Sprintf("Comma separated list of namespace globs to export applications from. If not provided value from '%s' in %s will be used,if it's not defined only applications from Argo CD namespace will be exported", applicationNamespacesCmdParamsKey, common.ArgoCDCmdParamsConfigMapName))
	command.Flags().StringSliceVarP(&applicationsetNamespaces, "applicationset-namespaces", "", []string{}, fmt.Sprintf("Comma separated list of namespace globs to export applicationsets from. If not provided value from '%s' in %s will be used,if it's not defined only applicationsets from Argo CD namespace will be exported", applicationsetNamespacesCmdParamsKey, common.ArgoCDCmdParamsConfigMapName))
	return &command
}

// NewImportCommand defines a new command for exporting Kubernetes and Argo CD resources.
func NewImportCommand() *cobra.Command {
	var (
		clientConfig             clientcmd.ClientConfig
		prune                    bool
		dryRun                   bool
		verbose                  bool
		stopOperation            bool
		ignoreTracking           bool
		overrideOnConflict       bool
		promptsEnabled           bool
		skipResourcesWithLabels  []string
		applicationNamespaces    []string
		applicationsetNamespaces []string
	)
	command := cobra.Command{
		Use:   "import SOURCE",
		Short: "Import Argo CD data from stdin (specify `-') or a file",
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			config, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			config.QPS = 100
			config.Burst = 50
			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)
			acdClients := newArgoCDClientsets(config, namespace)
			client, err := dynamic.NewForConfig(config)
			errors.CheckError(err)
			fmt.Printf("import process started %s\n", namespace)
			tt := time.Now()
			var input []byte
			if in := args[0]; in == "-" {
				input, err = io.ReadAll(os.Stdin)
			} else {
				input, err = os.ReadFile(in)
			}
			errors.CheckError(err)
			var dryRunMsg string
			if dryRun {
				dryRunMsg = " (dry run)"
			}

			additionalNamespaces := getAdditionalNamespaces(ctx, acdClients)
			if len(applicationNamespaces) == 0 {
				applicationNamespaces = additionalNamespaces.applicationNamespaces
			}
			if len(applicationsetNamespaces) == 0 {
				applicationsetNamespaces = additionalNamespaces.applicationsetNamespaces
			}

			// pruneObjects tracks live objects and it's current resource version. any remaining
			// items in this map indicates the resource should be pruned since it no longer appears
			// in the backup

			pruneObjects := make(map[kube.ResourceKey]unstructured.Unstructured)
			configMaps, err := acdClients.configMaps.List(ctx, metav1.ListOptions{})

			errors.CheckError(err)
			// referencedSecrets holds any secrets referenced in the argocd-cm configmap. These
			// secrets need to be imported too
			var referencedSecrets map[string]bool
			for _, cm := range configMaps.Items {
				if isArgoCDConfigMap(cm.GetName()) {
					pruneObjects[kube.ResourceKey{Group: "", Kind: "ConfigMap", Name: cm.GetName(), Namespace: cm.GetNamespace()}] = cm
				}
				if cm.GetName() == common.ArgoCDConfigMapName {
					referencedSecrets = getReferencedSecrets(cm)
				}
			}

			secrets, err := acdClients.secrets.List(ctx, metav1.ListOptions{})
			errors.CheckError(err)
			for _, secret := range secrets.Items {
				if isArgoCDSecret(referencedSecrets, secret) {
					pruneObjects[kube.ResourceKey{Group: "", Kind: "Secret", Name: secret.GetName(), Namespace: secret.GetNamespace()}] = secret
				}
			}
			applications, err := acdClients.applications.List(ctx, metav1.ListOptions{})
			errors.CheckError(err)
			for _, app := range applications.Items {
				if secutil.IsNamespaceEnabled(app.GetNamespace(), namespace, applicationNamespaces) {
					pruneObjects[kube.ResourceKey{Group: application.Group, Kind: application.ApplicationKind, Name: app.GetName(), Namespace: app.GetNamespace()}] = app
				}
			}
			projects, err := acdClients.projects.List(ctx, metav1.ListOptions{})
			errors.CheckError(err)
			for _, proj := range projects.Items {
				pruneObjects[kube.ResourceKey{Group: application.Group, Kind: application.AppProjectKind, Name: proj.GetName(), Namespace: proj.GetNamespace()}] = proj
			}
			applicationSets, err := acdClients.applicationSets.List(ctx, metav1.ListOptions{})
			if apierrors.IsForbidden(err) || apierrors.IsNotFound(err) {
				log.Warnf("argoproj.io/ApplicationSet: %v\n", err)
			} else {
				errors.CheckError(err)
			}
			if applicationSets != nil {
				for _, appSet := range applicationSets.Items {
					if secutil.IsNamespaceEnabled(appSet.GetNamespace(), namespace, applicationsetNamespaces) {
						pruneObjects[kube.ResourceKey{Group: application.Group, Kind: application.ApplicationSetKind, Name: appSet.GetName(), Namespace: appSet.GetNamespace()}] = appSet
					}
				}
			}
			// Create or replace existing object
			backupObjects, err := kube.SplitYAML(input)

			errors.CheckError(err)
			for _, bakObj := range backupObjects {
				if isLabelMatches(bakObj, skipResourcesWithLabels) {
					fmt.Printf("Skipping %s/%s %s in namespace %s\n", bakObj.GroupVersionKind().Group, bakObj.GroupVersionKind().Kind, bakObj.GetName(), bakObj.GetNamespace())
					continue
				}
				gvk := bakObj.GroupVersionKind()
				// For objects without namespace, assume they belong in ArgoCD namespace
				if bakObj.GetNamespace() == "" {
					bakObj.SetNamespace(namespace)
				}
				key := kube.ResourceKey{Group: gvk.Group, Kind: gvk.Kind, Name: bakObj.GetName(), Namespace: bakObj.GetNamespace()}
				liveObj, exists := pruneObjects[key]
				delete(pruneObjects, key)
				var dynClient dynamic.ResourceInterface
				switch bakObj.GetKind() {
				case "Secret":
					dynClient = client.Resource(secretResource).Namespace(bakObj.GetNamespace())
				case "ConfigMap":
					dynClient = client.Resource(configMapResource).Namespace(bakObj.GetNamespace())
				case application.AppProjectKind:
					dynClient = client.Resource(appprojectsResource).Namespace(bakObj.GetNamespace())
				case application.ApplicationKind:
					dynClient = client.Resource(applicationsResource).Namespace(bakObj.GetNamespace())
					// If application is not in one of the allowed namespaces do not import it
					if !secutil.IsNamespaceEnabled(bakObj.GetNamespace(), namespace, applicationNamespaces) {
						continue
					}
				case application.ApplicationSetKind:
					dynClient = client.Resource(appplicationSetResource).Namespace(bakObj.GetNamespace())
					// If applicationset is not in one of the allowed namespaces do not import it
					if !secutil.IsNamespaceEnabled(bakObj.GetNamespace(), namespace, applicationsetNamespaces) {
						continue
					}
				}

				// If there is a live object, remove the tracking annotations/label that might conflict
				// when argo is managed with an application.
				if ignoreTracking && exists {
					updateTracking(bakObj, &liveObj)
				}

				switch {
				case !exists:
					isForbidden := false
					if !dryRun {
						_, err = dynClient.Create(ctx, bakObj, metav1.CreateOptions{})
						if apierrors.IsForbidden(err) || apierrors.IsNotFound(err) {
							isForbidden = true
							log.Warnf("%s/%s %s: %v", gvk.Group, gvk.Kind, bakObj.GetName(), err)
						} else {
							errors.CheckError(err)
						}
					}
					if !isForbidden {
						fmt.Printf("%s/%s %s in namespace %s created%s\n", gvk.Group, gvk.Kind, bakObj.GetName(), bakObj.GetNamespace(), dryRunMsg)
					}
				case specsEqual(*bakObj, liveObj) && checkAppHasNoNeedToStopOperation(liveObj, stopOperation):
					if verbose {
						fmt.Printf("%s/%s %s unchanged%s\n", gvk.Group, gvk.Kind, bakObj.GetName(), dryRunMsg)
					}
				default:
					isForbidden := false
					if !dryRun {
						newLive := updateLive(bakObj, &liveObj, stopOperation)
						_, err = dynClient.Update(ctx, newLive, metav1.UpdateOptions{})
						if apierrors.IsConflict(err) {
							fmt.Printf("failed to update %s/%s %s in namespace %s: %v\n", gvk.Group, gvk.Kind, bakObj.GetName(), bakObj.GetNamespace(), err)
							if overrideOnConflict {
								err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
									fmt.Printf("resource conflict: retry update for Group: %s, Kind: %s, Name: %s, Namespace: %s\n", gvk.Group, gvk.Kind, bakObj.GetName(), bakObj.GetNamespace())
									liveObj, getErr := dynClient.Get(ctx, newLive.GetName(), metav1.GetOptions{})
									newLive.SetResourceVersion(liveObj.GetResourceVersion())
									annotations := newLive.GetAnnotations()
									newLive.SetAnnotations(annotations)
									if getErr != nil {
										errors.CheckError(getErr)
									}
									_, err = dynClient.Update(ctx, newLive, metav1.UpdateOptions{})
									return err
								})
							}
						}
						if apierrors.IsForbidden(err) || apierrors.IsNotFound(err) {
							isForbidden = true
							log.Warnf("%s/%s %s: %v", gvk.Group, gvk.Kind, bakObj.GetName(), err)
						} else {
							errors.CheckError(err)
						}
					}
					if !isForbidden {
						fmt.Printf("%s/%s %s in namespace %s updated%s\n", gvk.Group, gvk.Kind, bakObj.GetName(), bakObj.GetNamespace(), dryRunMsg)
					}
				}
			}

			promptUtil := utils.NewPrompt(promptsEnabled)

			// Delete objects not in backup
			for key, liveObj := range pruneObjects {
				if prune {
					var dynClient dynamic.ResourceInterface
					switch key.Kind {
					case "Secret":
						dynClient = client.Resource(secretResource).Namespace(liveObj.GetNamespace())
					case application.AppProjectKind:
						dynClient = client.Resource(appprojectsResource).Namespace(liveObj.GetNamespace())
					case application.ApplicationKind:
						dynClient = client.Resource(applicationsResource).Namespace(liveObj.GetNamespace())
						if !dryRun {
							if finalizers := liveObj.GetFinalizers(); len(finalizers) > 0 {
								newLive := liveObj.DeepCopy()
								newLive.SetFinalizers(nil)
								_, err = dynClient.Update(ctx, newLive, metav1.UpdateOptions{})
								if err != nil && !apierrors.IsNotFound(err) {
									errors.CheckError(err)
								}
							}
						}
					case application.ApplicationSetKind:
						dynClient = client.Resource(appplicationSetResource).Namespace(liveObj.GetNamespace())
					default:
						log.Fatalf("Unexpected kind '%s' in prune list", key.Kind)
					}
					isForbidden := false

					if !dryRun {
						canPrune := promptUtil.Confirm(fmt.Sprintf("Are you sure you want to prune %s/%s %s ? [y/n]", key.Group, key.Kind, key.Name))
						if canPrune {
							err = dynClient.Delete(ctx, key.Name, metav1.DeleteOptions{})
							if apierrors.IsForbidden(err) || apierrors.IsNotFound(err) {
								isForbidden = true
								log.Warnf("%s/%s %s: %v\n", key.Group, key.Kind, key.Name, err)
							} else {
								errors.CheckError(err)
							}
						} else {
							fmt.Printf("The command to prune %s/%s %s was cancelled.\n", key.Group, key.Kind, key.Name)
						}
					}
					if !isForbidden {
						fmt.Printf("%s/%s %s pruned%s\n", key.Group, key.Kind, key.Name, dryRunMsg)
					}
				} else {
					fmt.Printf("%s/%s %s needs pruning\n", key.Group, key.Kind, key.Name)
				}
			}
			duration := time.Since(tt)
			fmt.Printf("import process completed successfully in namespace %s at %s, duration: %s\n", namespace, time.Now().Format(time.RFC3339), duration)
		},
	}

	clientConfig = cli.AddKubectlFlagsToCmd(&command)
	command.Flags().BoolVar(&dryRun, "dry-run", false, "Print what will be performed")
	command.Flags().BoolVar(&prune, "prune", false, "Prune secrets, applications and projects which do not appear in the backup")
	command.Flags().BoolVar(&ignoreTracking, "ignore-tracking", false, "Do not update the tracking annotation if the resource is already tracked")
	command.Flags().BoolVar(&overrideOnConflict, "override-on-conflict", false, "Override the resource on conflict when updating resources")
	command.Flags().BoolVar(&verbose, "verbose", false, "Verbose output (versus only changed output)")
	command.Flags().BoolVar(&stopOperation, "stop-operation", false, "Stop any existing operations")
	command.Flags().StringSliceVarP(&skipResourcesWithLabels, "skip-resources-with-labels", "", []string{}, "Skip importing resources based on the labels e.g. '--skip-resources-with-labels my-label/example.io, my-other-label/example.io'")
	command.Flags().StringSliceVarP(&applicationNamespaces, "application-namespaces", "", []string{}, fmt.Sprintf("Comma separated list of namespace globs to which import of applications is allowed. If not provided value from '%s' in %s will be used,if it's not defined only applications without an explicit namespace will be imported to the Argo CD namespace", applicationNamespacesCmdParamsKey, common.ArgoCDCmdParamsConfigMapName))
	command.Flags().StringSliceVarP(&applicationsetNamespaces, "applicationset-namespaces", "", []string{}, fmt.Sprintf("Comma separated list of namespace globs which import of applicationsets is allowed. If not provided value from '%s' in %s will be used,if it's not defined only applicationsets without an explicit namespace will be imported to the Argo CD namespace", applicationsetNamespacesCmdParamsKey, common.ArgoCDCmdParamsConfigMapName))
	command.PersistentFlags().BoolVar(&promptsEnabled, "prompts-enabled", localconfig.GetPromptsEnabled(true), "Force optional interactive prompts to be enabled or disabled, overriding local configuration. If not specified, the local configuration value will be used, which is false by default.")
	return &command
}

// check app has no need to stop operation.
func checkAppHasNoNeedToStopOperation(liveObj unstructured.Unstructured, stopOperation bool) bool {
	if !stopOperation {
		return true
	}
	if liveObj.GetKind() == application.ApplicationKind {
		return liveObj.Object["operation"] == nil
	}
	return true
}

// export writes the unstructured object and removes extraneous cruft from output before writing
func export(w io.Writer, un unstructured.Unstructured, argocdNamespace string) {
	name := un.GetName()
	finalizers := un.GetFinalizers()
	apiVersion := un.GetAPIVersion()
	kind := un.GetKind()
	labels := un.GetLabels()
	annotations := un.GetAnnotations()
	namespace := un.GetNamespace()
	unstructured.RemoveNestedField(un.Object, "metadata")
	un.SetName(name)
	un.SetFinalizers(finalizers)
	un.SetAPIVersion(apiVersion)
	un.SetKind(kind)
	un.SetLabels(labels)
	un.SetAnnotations(annotations)
	if namespace != argocdNamespace {
		un.SetNamespace(namespace)
	}
	data, err := yaml.Marshal(un.Object)
	errors.CheckError(err)
	_, err = w.Write(data)
	errors.CheckError(err)
	_, err = w.Write([]byte(yamlSeparator))
	errors.CheckError(err)
}

// updateLive replaces the live object's finalizers, spec, annotations, labels, and data from the
// backup object but leaves all other fields intact (status, other metadata, etc...)
func updateLive(bak, live *unstructured.Unstructured, stopOperation bool) *unstructured.Unstructured {
	newLive := live.DeepCopy()
	newLive.SetAnnotations(bak.GetAnnotations())
	newLive.SetLabels(bak.GetLabels())
	newLive.SetFinalizers(bak.GetFinalizers())
	switch live.GetKind() {
	case "Secret", "ConfigMap":
		newLive.Object["data"] = bak.Object["data"]
	case application.AppProjectKind:
		newLive.Object["spec"] = bak.Object["spec"]
	case application.ApplicationKind:
		newLive.Object["spec"] = bak.Object["spec"]
		if _, ok := bak.Object["status"]; ok {
			newLive.Object["status"] = bak.Object["status"]
		}
		if stopOperation {
			newLive.Object["operation"] = nil
		}

	case "ApplicationSet":
		newLive.Object["spec"] = bak.Object["spec"]
	}
	return newLive
}

// updateTracking will update the tracking label and annotation in the bak resources to the
// value of the live resource.
func updateTracking(bak, live *unstructured.Unstructured) {
	// update the common annotation
	bakAnnotations := bak.GetAnnotations()
	liveAnnotations := live.GetAnnotations()
	if liveAnnotations != nil && bakAnnotations != nil {
		if v, ok := liveAnnotations[common.AnnotationKeyAppInstance]; ok {
			if _, ok := bakAnnotations[common.AnnotationKeyAppInstance]; ok {
				bakAnnotations[common.AnnotationKeyAppInstance] = v
				bak.SetAnnotations(bakAnnotations)
			}
		}
	}

	// update the common label
	// A custom label can be set, but it is impossible to know which instance is managing the application
	bakLabels := bak.GetLabels()
	liveLabels := live.GetLabels()
	if liveLabels != nil && bakLabels != nil {
		if v, ok := liveLabels[common.LabelKeyAppInstance]; ok {
			if _, ok := bakLabels[common.LabelKeyAppInstance]; ok {
				bakLabels[common.LabelKeyAppInstance] = v
				bak.SetLabels(bakLabels)
			}
		}
	}
}

// skip resource   if any of the specified label exists.
func isLabelMatches(bak *unstructured.Unstructured, skipResourcesWithLabels []string) bool {
	if bak == nil {
		return false
	}
	labels := bak.GetLabels()
	for _, key := range skipResourcesWithLabels {
		if _, exists := labels[key]; exists {
			return true
		}
	}
	return false
}