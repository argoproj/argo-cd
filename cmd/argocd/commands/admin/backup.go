package admin

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/errors"
)

// NewExportCommand defines a new command for exporting Kubernetes and Argo CD resources.
func NewExportCommand() *cobra.Command {
	var (
		clientConfig clientcmd.ClientConfig
		out          string
	)
	var command = cobra.Command{
		Use:   "export",
		Short: "Export all Argo CD data to stdout (default) or a file",
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			config, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)

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
			acdConfigMap, err := acdClients.configMaps.Get(ctx, common.ArgoCDConfigMapName, v1.GetOptions{})
			errors.CheckError(err)
			export(writer, *acdConfigMap)
			acdRBACConfigMap, err := acdClients.configMaps.Get(ctx, common.ArgoCDRBACConfigMapName, v1.GetOptions{})
			errors.CheckError(err)
			export(writer, *acdRBACConfigMap)
			acdKnownHostsConfigMap, err := acdClients.configMaps.Get(ctx, common.ArgoCDKnownHostsConfigMapName, v1.GetOptions{})
			errors.CheckError(err)
			export(writer, *acdKnownHostsConfigMap)
			acdTLSCertsConfigMap, err := acdClients.configMaps.Get(ctx, common.ArgoCDTLSCertsConfigMapName, v1.GetOptions{})
			errors.CheckError(err)
			export(writer, *acdTLSCertsConfigMap)

			referencedSecrets := getReferencedSecrets(*acdConfigMap)
			secrets, err := acdClients.secrets.List(ctx, v1.ListOptions{})
			errors.CheckError(err)
			for _, secret := range secrets.Items {
				if isArgoCDSecret(referencedSecrets, secret) {
					export(writer, secret)
				}
			}
			projects, err := acdClients.projects.List(ctx, v1.ListOptions{})
			errors.CheckError(err)
			for _, proj := range projects.Items {
				export(writer, proj)
			}
			applications, err := acdClients.applications.List(ctx, v1.ListOptions{})
			errors.CheckError(err)
			for _, app := range applications.Items {
				export(writer, app)
			}
			applicationSets, err := acdClients.applicationSets.List(ctx, v1.ListOptions{})
			if err != nil && !apierr.IsNotFound(err) {
				if apierr.IsForbidden(err) {
					log.Warn(err)
				} else {
					errors.CheckError(err)
				}
			}
			if applicationSets != nil {
				for _, appSet := range applicationSets.Items {
					export(writer, appSet)
				}
			}
		},
	}

	clientConfig = cli.AddKubectlFlagsToCmd(&command)
	command.Flags().StringVarP(&out, "out", "o", "-", "Output to the specified file instead of stdout")

	return &command
}

// NewImportCommand defines a new command for exporting Kubernetes and Argo CD resources.
func NewImportCommand() *cobra.Command {
	var (
		clientConfig  clientcmd.ClientConfig
		prune         bool
		dryRun        bool
		verbose       bool
		stopOperation bool
	)
	var command = cobra.Command{
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

			// pruneObjects tracks live objects and it's current resource version. any remaining
			// items in this map indicates the resource should be pruned since it no longer appears
			// in the backup
			pruneObjects := make(map[kube.ResourceKey]unstructured.Unstructured)
			configMaps, err := acdClients.configMaps.List(ctx, v1.ListOptions{})
			errors.CheckError(err)
			// referencedSecrets holds any secrets referenced in the argocd-cm configmap. These
			// secrets need to be imported too
			var referencedSecrets map[string]bool
			for _, cm := range configMaps.Items {
				if isArgoCDConfigMap(cm.GetName()) {
					pruneObjects[kube.ResourceKey{Group: "", Kind: "ConfigMap", Name: cm.GetName()}] = cm
				}
				if cm.GetName() == common.ArgoCDConfigMapName {
					referencedSecrets = getReferencedSecrets(cm)
				}
			}

			secrets, err := acdClients.secrets.List(ctx, v1.ListOptions{})
			errors.CheckError(err)
			for _, secret := range secrets.Items {
				if isArgoCDSecret(referencedSecrets, secret) {
					pruneObjects[kube.ResourceKey{Group: "", Kind: "Secret", Name: secret.GetName()}] = secret
				}
			}
			applications, err := acdClients.applications.List(ctx, v1.ListOptions{})
			errors.CheckError(err)
			for _, app := range applications.Items {
				pruneObjects[kube.ResourceKey{Group: application.Group, Kind: application.ApplicationKind, Name: app.GetName()}] = app
			}
			projects, err := acdClients.projects.List(ctx, v1.ListOptions{})
			errors.CheckError(err)
			for _, proj := range projects.Items {
				pruneObjects[kube.ResourceKey{Group: application.Group, Kind: application.AppProjectKind, Name: proj.GetName()}] = proj
			}
			applicationSets, err := acdClients.applicationSets.List(ctx, v1.ListOptions{})
			if apierr.IsForbidden(err) || apierr.IsNotFound(err) {
				log.Warnf("argoproj.io/ApplicationSet: %v\n", err)
			} else {
				errors.CheckError(err)
			}
			if applicationSets != nil {
				for _, appSet := range applicationSets.Items {
					pruneObjects[kube.ResourceKey{Group: application.Group, Kind: application.ApplicationSetKind, Name: appSet.GetName()}] = appSet
				}
			}

			// Create or replace existing object
			backupObjects, err := kube.SplitYAML(input)
			errors.CheckError(err)
			for _, bakObj := range backupObjects {
				gvk := bakObj.GroupVersionKind()
				key := kube.ResourceKey{Group: gvk.Group, Kind: gvk.Kind, Name: bakObj.GetName()}
				liveObj, exists := pruneObjects[key]
				delete(pruneObjects, key)
				var dynClient dynamic.ResourceInterface
				switch bakObj.GetKind() {
				case "Secret":
					dynClient = acdClients.secrets
				case "ConfigMap":
					dynClient = acdClients.configMaps
				case application.AppProjectKind:
					dynClient = acdClients.projects
				case application.ApplicationKind:
					dynClient = acdClients.applications
				case application.ApplicationSetKind:
					dynClient = acdClients.applicationSets
				}
				if !exists {
					isForbidden := false
					if !dryRun {
						_, err = dynClient.Create(ctx, bakObj, v1.CreateOptions{})
						if apierr.IsForbidden(err) || apierr.IsNotFound(err) {
							isForbidden = true
							log.Warnf("%s/%s %s: %v", gvk.Group, gvk.Kind, bakObj.GetName(), err)
						} else {
							errors.CheckError(err)
						}
					}
					if !isForbidden {
						fmt.Printf("%s/%s %s created%s\n", gvk.Group, gvk.Kind, bakObj.GetName(), dryRunMsg)
					}

				} else if specsEqual(*bakObj, liveObj) && checkAppHasNoNeedToStopOperation(liveObj, stopOperation) {
					if verbose {
						fmt.Printf("%s/%s %s unchanged%s\n", gvk.Group, gvk.Kind, bakObj.GetName(), dryRunMsg)
					}
				} else {
					isForbidden := false
					if !dryRun {
						newLive := updateLive(bakObj, &liveObj, stopOperation)
						_, err = dynClient.Update(ctx, newLive, v1.UpdateOptions{})
						if apierr.IsForbidden(err) || apierr.IsNotFound(err) {
							isForbidden = true
							log.Warnf("%s/%s %s: %v", gvk.Group, gvk.Kind, bakObj.GetName(), err)
						} else {
							errors.CheckError(err)
						}
					}
					if !isForbidden {
						fmt.Printf("%s/%s %s updated%s\n", gvk.Group, gvk.Kind, bakObj.GetName(), dryRunMsg)
					}
				}
			}

			// Delete objects not in backup
			for key, liveObj := range pruneObjects {
				if prune {
					var dynClient dynamic.ResourceInterface
					switch key.Kind {
					case "Secret":
						dynClient = acdClients.secrets
					case application.AppProjectKind:
						dynClient = acdClients.projects
					case application.ApplicationKind:
						dynClient = acdClients.applications
						if !dryRun {
							if finalizers := liveObj.GetFinalizers(); len(finalizers) > 0 {
								newLive := liveObj.DeepCopy()
								newLive.SetFinalizers(nil)
								_, err = dynClient.Update(ctx, newLive, v1.UpdateOptions{})
								if err != nil && !apierr.IsNotFound(err) {
									errors.CheckError(err)
								}
							}
						}
					case application.ApplicationSetKind:
						dynClient = acdClients.applicationSets
					default:
						log.Fatalf("Unexpected kind '%s' in prune list", key.Kind)
					}
					isForbidden := false
					if !dryRun {
						err = dynClient.Delete(ctx, key.Name, v1.DeleteOptions{})
						if apierr.IsForbidden(err) || apierr.IsNotFound(err) {
							isForbidden = true
							log.Warnf("%s/%s %s: %v\n", key.Group, key.Kind, key.Name, err)
						} else {
							errors.CheckError(err)
						}
					}
					if !isForbidden {
						fmt.Printf("%s/%s %s pruned%s\n", key.Group, key.Kind, key.Name, dryRunMsg)
					}
				} else {
					fmt.Printf("%s/%s %s needs pruning\n", key.Group, key.Kind, key.Name)
				}
			}
		},
	}

	clientConfig = cli.AddKubectlFlagsToCmd(&command)
	command.Flags().BoolVar(&dryRun, "dry-run", false, "Print what will be performed")
	command.Flags().BoolVar(&prune, "prune", false, "Prune secrets, applications and projects which do not appear in the backup")
	command.Flags().BoolVar(&verbose, "verbose", false, "Verbose output (versus only changed output)")
	command.Flags().BoolVar(&stopOperation, "stop-operation", false, "Stop any existing operations")

	return &command
}

// check app has no need to stop operation.
func checkAppHasNoNeedToStopOperation(liveObj unstructured.Unstructured, stopOperation bool) bool {
	if !stopOperation {
		return true
	}
	switch liveObj.GetKind() {
	case application.ApplicationKind:
		return liveObj.Object["operation"] == nil
	}
	return true
}

// export writes the unstructured object and removes extraneous cruft from output before writing
func export(w io.Writer, un unstructured.Unstructured) {
	name := un.GetName()
	finalizers := un.GetFinalizers()
	apiVersion := un.GetAPIVersion()
	kind := un.GetKind()
	labels := un.GetLabels()
	annotations := un.GetAnnotations()
	unstructured.RemoveNestedField(un.Object, "metadata")
	un.SetName(name)
	un.SetFinalizers(finalizers)
	un.SetAPIVersion(apiVersion)
	un.SetKind(kind)
	un.SetLabels(labels)
	un.SetAnnotations(annotations)
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
