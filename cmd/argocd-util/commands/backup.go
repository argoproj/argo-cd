package commands

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/ghodss/yaml"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/errors"
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
			acdConfigMap, err := acdClients.configMaps.Get(context.Background(), common.ArgoCDConfigMapName, v1.GetOptions{})
			errors.CheckError(err)
			export(writer, *acdConfigMap)
			acdRBACConfigMap, err := acdClients.configMaps.Get(context.Background(), common.ArgoCDRBACConfigMapName, v1.GetOptions{})
			errors.CheckError(err)
			export(writer, *acdRBACConfigMap)
			acdKnownHostsConfigMap, err := acdClients.configMaps.Get(context.Background(), common.ArgoCDKnownHostsConfigMapName, v1.GetOptions{})
			errors.CheckError(err)
			export(writer, *acdKnownHostsConfigMap)
			acdTLSCertsConfigMap, err := acdClients.configMaps.Get(context.Background(), common.ArgoCDTLSCertsConfigMapName, v1.GetOptions{})
			errors.CheckError(err)
			export(writer, *acdTLSCertsConfigMap)

			referencedSecrets := getReferencedSecrets(*acdConfigMap)
			secrets, err := acdClients.secrets.List(context.Background(), v1.ListOptions{})
			errors.CheckError(err)
			for _, secret := range secrets.Items {
				if isArgoCDSecret(referencedSecrets, secret) {
					export(writer, secret)
				}
			}
			projects, err := acdClients.projects.List(context.Background(), v1.ListOptions{})
			errors.CheckError(err)
			for _, proj := range projects.Items {
				export(writer, proj)
			}
			applications, err := acdClients.applications.List(context.Background(), v1.ListOptions{})
			errors.CheckError(err)
			for _, app := range applications.Items {
				export(writer, app)
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
		clientConfig clientcmd.ClientConfig
		prune        bool
		dryRun       bool
		verbose      bool
	)
	var command = cobra.Command{
		Use:   "import SOURCE",
		Short: "Import Argo CD data from stdin (specify `-') or a file",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			config, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			config.QPS = 100
			config.Burst = 50
			errors.CheckError(err)
			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)
			acdClients := newArgoCDClientsets(config, namespace)

			var input []byte
			if in := args[0]; in == "-" {
				input, err = ioutil.ReadAll(os.Stdin)
			} else {
				input, err = ioutil.ReadFile(in)
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
			configMaps, err := acdClients.configMaps.List(context.Background(), v1.ListOptions{})
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

			secrets, err := acdClients.secrets.List(context.Background(), v1.ListOptions{})
			errors.CheckError(err)
			for _, secret := range secrets.Items {
				if isArgoCDSecret(referencedSecrets, secret) {
					pruneObjects[kube.ResourceKey{Group: "", Kind: "Secret", Name: secret.GetName()}] = secret
				}
			}
			applications, err := acdClients.applications.List(context.Background(), v1.ListOptions{})
			errors.CheckError(err)
			for _, app := range applications.Items {
				pruneObjects[kube.ResourceKey{Group: "argoproj.io", Kind: "Application", Name: app.GetName()}] = app
			}
			projects, err := acdClients.projects.List(context.Background(), v1.ListOptions{})
			errors.CheckError(err)
			for _, proj := range projects.Items {
				pruneObjects[kube.ResourceKey{Group: "argoproj.io", Kind: "AppProject", Name: proj.GetName()}] = proj
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
				case "AppProject":
					dynClient = acdClients.projects
				case "Application":
					dynClient = acdClients.applications
				}
				if !exists {
					if !dryRun {
						_, err = dynClient.Create(context.Background(), bakObj, v1.CreateOptions{})
						errors.CheckError(err)
					}
					fmt.Printf("%s/%s %s created%s\n", gvk.Group, gvk.Kind, bakObj.GetName(), dryRunMsg)
				} else if specsEqual(*bakObj, liveObj) {
					if verbose {
						fmt.Printf("%s/%s %s unchanged%s\n", gvk.Group, gvk.Kind, bakObj.GetName(), dryRunMsg)
					}
				} else {
					if !dryRun {
						newLive := updateLive(bakObj, &liveObj)
						_, err = dynClient.Update(context.Background(), newLive, v1.UpdateOptions{})
						errors.CheckError(err)
					}
					fmt.Printf("%s/%s %s updated%s\n", gvk.Group, gvk.Kind, bakObj.GetName(), dryRunMsg)
				}
			}

			// Delete objects not in backup
			for key, liveObj := range pruneObjects {
				if prune {
					var dynClient dynamic.ResourceInterface
					switch key.Kind {
					case "Secret":
						dynClient = acdClients.secrets
					case "AppProject":
						dynClient = acdClients.projects
					case "Application":
						dynClient = acdClients.applications
						if !dryRun {
							if finalizers := liveObj.GetFinalizers(); len(finalizers) > 0 {
								newLive := liveObj.DeepCopy()
								newLive.SetFinalizers(nil)
								_, err = dynClient.Update(context.Background(), newLive, v1.UpdateOptions{})
								if err != nil && !apierr.IsNotFound(err) {
									errors.CheckError(err)
								}
							}
						}
					default:
						logrus.Fatalf("Unexpected kind '%s' in prune list", key.Kind)
					}
					if !dryRun {
						err = dynClient.Delete(context.Background(), key.Name, v1.DeleteOptions{})
						if err != nil && !apierr.IsNotFound(err) {
							errors.CheckError(err)
						}
					}
					fmt.Printf("%s/%s %s pruned%s\n", key.Group, key.Kind, key.Name, dryRunMsg)
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

	return &command
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
func updateLive(bak, live *unstructured.Unstructured) *unstructured.Unstructured {
	newLive := live.DeepCopy()
	newLive.SetAnnotations(bak.GetAnnotations())
	newLive.SetLabels(bak.GetLabels())
	newLive.SetFinalizers(bak.GetFinalizers())
	switch live.GetKind() {
	case "Secret", "ConfigMap":
		newLive.Object["data"] = bak.Object["data"]
	case "AppProject":
		newLive.Object["spec"] = bak.Object["spec"]
	case "Application":
		newLive.Object["spec"] = bak.Object["spec"]
		if _, ok := bak.Object["status"]; ok {
			newLive.Object["status"] = bak.Object["status"]
		}
	}
	return newLive
}
