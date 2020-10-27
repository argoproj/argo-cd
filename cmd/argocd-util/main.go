package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"reflect"
	"syscall"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/cmd/argocd-util/commands"
	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/dex"
	"github.com/argoproj/argo-cd/util/errors"
	"github.com/argoproj/argo-cd/util/settings"

	// load the gcp plugin (required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// load the oidc plugin (required to authenticate with OpenID Connect).
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	// load the azure plugin (required to authenticate with AKS clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/azure"
)

const (
	// CLIName is the name of the CLI
	cliName = "argocd-util"
	// YamlSeparator separates sections of a YAML file
	yamlSeparator = "---\n"
)

var (
	configMapResource    = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	secretResource       = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
	applicationsResource = schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}
	appprojectsResource  = schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "appprojects"}
)

// NewCommand returns a new instance of an argocd command
func NewCommand() *cobra.Command {
	var (
		logFormat string
		logLevel  string
	)

	var command = &cobra.Command{
		Use:   cliName,
		Short: "argocd-util has internal tools used by Argo CD",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}

	command.AddCommand(cli.NewVersionCmd(cliName))
	command.AddCommand(NewRunDexCommand())
	command.AddCommand(NewGenDexConfigCommand())
	command.AddCommand(NewImportCommand())
	command.AddCommand(NewExportCommand())
	command.AddCommand(NewClusterConfig())
	command.AddCommand(commands.NewProjectsCommand())
	command.AddCommand(commands.NewSettingsCommand())
	command.AddCommand(commands.NewAppsCommand())

	command.Flags().StringVar(&logFormat, "logformat", "text", "Set the logging format. One of: text|json")
	command.Flags().StringVar(&logLevel, "loglevel", "info", "Set the logging level. One of: debug|info|warn|error")
	return command
}

func NewRunDexCommand() *cobra.Command {
	var (
		clientConfig clientcmd.ClientConfig
	)
	var command = cobra.Command{
		Use:   "rundex",
		Short: "Runs dex generating a config using settings from the Argo CD configmap and secret",
		RunE: func(c *cobra.Command, args []string) error {
			_, err := exec.LookPath("dex")
			errors.CheckError(err)
			config, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)
			kubeClientset := kubernetes.NewForConfigOrDie(config)
			settingsMgr := settings.NewSettingsManager(context.Background(), kubeClientset, namespace)
			prevSettings, err := settingsMgr.GetSettings()
			errors.CheckError(err)
			updateCh := make(chan *settings.ArgoCDSettings, 1)
			settingsMgr.Subscribe(updateCh)

			for {
				var cmd *exec.Cmd
				dexCfgBytes, err := dex.GenerateDexConfigYAML(prevSettings)
				errors.CheckError(err)
				if len(dexCfgBytes) == 0 {
					log.Infof("dex is not configured")
				} else {
					err = ioutil.WriteFile("/tmp/dex.yaml", dexCfgBytes, 0644)
					errors.CheckError(err)
					log.Debug(redactor(string(dexCfgBytes)))
					cmd = exec.Command("dex", "serve", "/tmp/dex.yaml")
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					err = cmd.Start()
					errors.CheckError(err)
				}

				// loop until the dex config changes
				for {
					newSettings := <-updateCh
					newDexCfgBytes, err := dex.GenerateDexConfigYAML(newSettings)
					errors.CheckError(err)
					if string(newDexCfgBytes) != string(dexCfgBytes) {
						prevSettings = newSettings
						log.Infof("dex config modified. restarting dex")
						if cmd != nil && cmd.Process != nil {
							err = cmd.Process.Signal(syscall.SIGTERM)
							errors.CheckError(err)
							_, err = cmd.Process.Wait()
							errors.CheckError(err)
						}
						break
					} else {
						log.Infof("dex config unmodified")
					}
				}
			}
		},
	}

	clientConfig = cli.AddKubectlFlagsToCmd(&command)
	return &command
}

func NewGenDexConfigCommand() *cobra.Command {
	var (
		clientConfig clientcmd.ClientConfig
		out          string
	)
	var command = cobra.Command{
		Use:   "gendexcfg",
		Short: "Generates a dex config from Argo CD settings",
		RunE: func(c *cobra.Command, args []string) error {
			config, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)
			kubeClientset := kubernetes.NewForConfigOrDie(config)
			settingsMgr := settings.NewSettingsManager(context.Background(), kubeClientset, namespace)
			settings, err := settingsMgr.GetSettings()
			errors.CheckError(err)
			dexCfgBytes, err := dex.GenerateDexConfigYAML(settings)
			errors.CheckError(err)
			if len(dexCfgBytes) == 0 {
				log.Infof("dex is not configured")
				return nil
			}
			if out == "" {
				dexCfg := make(map[string]interface{})
				err := yaml.Unmarshal(dexCfgBytes, &dexCfg)
				errors.CheckError(err)
				if staticClientsInterface, ok := dexCfg["staticClients"]; ok {
					if staticClients, ok := staticClientsInterface.([]interface{}); ok {
						for i := range staticClients {
							staticClient := staticClients[i]
							if mappings, ok := staticClient.(map[string]interface{}); ok {
								for key := range mappings {
									if key == "secret" {
										mappings[key] = "******"
									}
								}
								staticClients[i] = mappings
							}
						}
						dexCfg["staticClients"] = staticClients
					}
				}
				errors.CheckError(err)
				maskedDexCfgBytes, err := yaml.Marshal(dexCfg)
				errors.CheckError(err)
				fmt.Print(string(maskedDexCfgBytes))
			} else {
				err = ioutil.WriteFile(out, dexCfgBytes, 0644)
				errors.CheckError(err)
			}
			return nil
		},
	}

	clientConfig = cli.AddKubectlFlagsToCmd(&command)
	command.Flags().StringVarP(&out, "out", "o", "", "Output to the specified file instead of stdout")
	return &command
}

// NewImportCommand defines a new command for exporting Kubernetes and Argo CD resources.
func NewImportCommand() *cobra.Command {
	var (
		clientConfig clientcmd.ClientConfig
		prune        bool
		dryRun       bool
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
			configMaps, err := acdClients.configMaps.List(context.Background(), metav1.ListOptions{})
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

			secrets, err := acdClients.secrets.List(context.Background(), metav1.ListOptions{})
			errors.CheckError(err)
			for _, secret := range secrets.Items {
				if isArgoCDSecret(referencedSecrets, secret) {
					pruneObjects[kube.ResourceKey{Group: "", Kind: "Secret", Name: secret.GetName()}] = secret
				}
			}
			applications, err := acdClients.applications.List(context.Background(), metav1.ListOptions{})
			errors.CheckError(err)
			for _, app := range applications.Items {
				pruneObjects[kube.ResourceKey{Group: "argoproj.io", Kind: "Application", Name: app.GetName()}] = app
			}
			projects, err := acdClients.projects.List(context.Background(), metav1.ListOptions{})
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
						_, err = dynClient.Create(context.Background(), bakObj, metav1.CreateOptions{})
						errors.CheckError(err)
					}
					fmt.Printf("%s/%s %s created%s\n", gvk.Group, gvk.Kind, bakObj.GetName(), dryRunMsg)
				} else if specsEqual(*bakObj, liveObj) {
					fmt.Printf("%s/%s %s unchanged%s\n", gvk.Group, gvk.Kind, bakObj.GetName(), dryRunMsg)
				} else {
					if !dryRun {
						newLive := updateLive(bakObj, &liveObj)
						_, err = dynClient.Update(context.Background(), newLive, metav1.UpdateOptions{})
						errors.CheckError(err)
					}
					fmt.Printf("%s/%s %s updated%s\n", gvk.Group, gvk.Kind, bakObj.GetName(), dryRunMsg)
				}
			}

			// Delete objects not in backup
			for key := range pruneObjects {
				if prune {
					var dynClient dynamic.ResourceInterface
					switch key.Kind {
					case "Secret":
						dynClient = acdClients.secrets
					case "AppProject":
						dynClient = acdClients.projects
					case "Application":
						dynClient = acdClients.applications
					default:
						log.Fatalf("Unexpected kind '%s' in prune list", key.Kind)
					}
					if !dryRun {
						err = dynClient.Delete(context.Background(), key.Name, metav1.DeleteOptions{})
						errors.CheckError(err)
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

	return &command
}

type argoCDClientsets struct {
	configMaps   dynamic.ResourceInterface
	secrets      dynamic.ResourceInterface
	applications dynamic.ResourceInterface
	projects     dynamic.ResourceInterface
}

func newArgoCDClientsets(config *rest.Config, namespace string) *argoCDClientsets {
	dynamicIf, err := dynamic.NewForConfig(config)
	errors.CheckError(err)
	return &argoCDClientsets{
		configMaps:   dynamicIf.Resource(configMapResource).Namespace(namespace),
		secrets:      dynamicIf.Resource(secretResource).Namespace(namespace),
		applications: dynamicIf.Resource(applicationsResource).Namespace(namespace),
		projects:     dynamicIf.Resource(appprojectsResource).Namespace(namespace),
	}
}

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
			acdConfigMap, err := acdClients.configMaps.Get(context.Background(), common.ArgoCDConfigMapName, metav1.GetOptions{})
			errors.CheckError(err)
			export(writer, *acdConfigMap)
			acdRBACConfigMap, err := acdClients.configMaps.Get(context.Background(), common.ArgoCDRBACConfigMapName, metav1.GetOptions{})
			errors.CheckError(err)
			export(writer, *acdRBACConfigMap)
			acdKnownHostsConfigMap, err := acdClients.configMaps.Get(context.Background(), common.ArgoCDKnownHostsConfigMapName, metav1.GetOptions{})
			errors.CheckError(err)
			export(writer, *acdKnownHostsConfigMap)
			acdTLSCertsConfigMap, err := acdClients.configMaps.Get(context.Background(), common.ArgoCDTLSCertsConfigMapName, metav1.GetOptions{})
			errors.CheckError(err)
			export(writer, *acdTLSCertsConfigMap)

			referencedSecrets := getReferencedSecrets(*acdConfigMap)
			secrets, err := acdClients.secrets.List(context.Background(), metav1.ListOptions{})
			errors.CheckError(err)
			for _, secret := range secrets.Items {
				if isArgoCDSecret(referencedSecrets, secret) {
					export(writer, secret)
				}
			}
			projects, err := acdClients.projects.List(context.Background(), metav1.ListOptions{})
			errors.CheckError(err)
			for _, proj := range projects.Items {
				export(writer, proj)
			}
			applications, err := acdClients.applications.List(context.Background(), metav1.ListOptions{})
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

// getReferencedSecrets examines the argocd-cm config for any referenced repo secrets and returns a
// map of all referenced secrets.
func getReferencedSecrets(un unstructured.Unstructured) map[string]bool {
	var cm apiv1.ConfigMap
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(un.Object, &cm)
	errors.CheckError(err)
	referencedSecrets := make(map[string]bool)

	// Referenced repository secrets
	if reposRAW, ok := cm.Data["repositories"]; ok {
		repos := make([]settings.Repository, 0)
		err := yaml.Unmarshal([]byte(reposRAW), &repos)
		errors.CheckError(err)
		for _, cred := range repos {
			if cred.PasswordSecret != nil {
				referencedSecrets[cred.PasswordSecret.Name] = true
			}
			if cred.SSHPrivateKeySecret != nil {
				referencedSecrets[cred.SSHPrivateKeySecret.Name] = true
			}
			if cred.UsernameSecret != nil {
				referencedSecrets[cred.UsernameSecret.Name] = true
			}
			if cred.TLSClientCertDataSecret != nil {
				referencedSecrets[cred.TLSClientCertDataSecret.Name] = true
			}
			if cred.TLSClientCertKeySecret != nil {
				referencedSecrets[cred.TLSClientCertKeySecret.Name] = true
			}
		}
	}

	// Referenced repository credentials secrets
	if reposRAW, ok := cm.Data["repository.credentials"]; ok {
		creds := make([]settings.RepositoryCredentials, 0)
		err := yaml.Unmarshal([]byte(reposRAW), &creds)
		errors.CheckError(err)
		for _, cred := range creds {
			if cred.PasswordSecret != nil {
				referencedSecrets[cred.PasswordSecret.Name] = true
			}
			if cred.SSHPrivateKeySecret != nil {
				referencedSecrets[cred.SSHPrivateKeySecret.Name] = true
			}
			if cred.UsernameSecret != nil {
				referencedSecrets[cred.UsernameSecret.Name] = true
			}
			if cred.TLSClientCertDataSecret != nil {
				referencedSecrets[cred.TLSClientCertDataSecret.Name] = true
			}
			if cred.TLSClientCertKeySecret != nil {
				referencedSecrets[cred.TLSClientCertKeySecret.Name] = true
			}
		}
	}
	return referencedSecrets
}

// isArgoCDSecret returns whether or not the given secret is a part of Argo CD configuration
// (e.g. argocd-secret, repo credentials, or cluster credentials)
func isArgoCDSecret(repoSecretRefs map[string]bool, un unstructured.Unstructured) bool {
	secretName := un.GetName()
	if secretName == common.ArgoCDSecretName {
		return true
	}
	if repoSecretRefs != nil {
		if _, ok := repoSecretRefs[secretName]; ok {
			return true
		}
	}
	if labels := un.GetLabels(); labels != nil {
		if _, ok := labels[common.LabelKeySecretType]; ok {
			return true
		}
	}
	if annotations := un.GetAnnotations(); annotations != nil {
		if annotations[common.AnnotationKeyManagedBy] == common.AnnotationValueManagedByArgoCD {
			return true
		}
	}
	return false
}

// isArgoCDConfigMap returns true if the configmap name is one of argo cd's well known configmaps
func isArgoCDConfigMap(name string) bool {
	switch name {
	case common.ArgoCDConfigMapName, common.ArgoCDRBACConfigMapName, common.ArgoCDKnownHostsConfigMapName, common.ArgoCDTLSCertsConfigMapName:
		return true
	}
	return false

}

// specsEqual returns if the spec, data, labels, annotations, and finalizers of the two
// supplied objects are equal, indicating that no update is necessary during importing
func specsEqual(left, right unstructured.Unstructured) bool {
	if !reflect.DeepEqual(left.GetAnnotations(), right.GetAnnotations()) {
		return false
	}
	if !reflect.DeepEqual(left.GetLabels(), right.GetLabels()) {
		return false
	}
	if !reflect.DeepEqual(left.GetFinalizers(), right.GetFinalizers()) {
		return false
	}
	switch left.GetKind() {
	case "Secret", "ConfigMap":
		leftData, _, _ := unstructured.NestedMap(left.Object, "data")
		rightData, _, _ := unstructured.NestedMap(right.Object, "data")
		return reflect.DeepEqual(leftData, rightData)
	case "AppProject":
		leftSpec, _, _ := unstructured.NestedMap(left.Object, "spec")
		rightSpec, _, _ := unstructured.NestedMap(right.Object, "spec")
		return reflect.DeepEqual(leftSpec, rightSpec)
	case "Application":
		leftSpec, _, _ := unstructured.NestedMap(left.Object, "spec")
		rightSpec, _, _ := unstructured.NestedMap(right.Object, "spec")
		leftStatus, _, _ := unstructured.NestedMap(left.Object, "status")
		rightStatus, _, _ := unstructured.NestedMap(right.Object, "status")
		// reconciledAt and observedAt are constantly changing and we ignore any diff there
		delete(leftStatus, "reconciledAt")
		delete(rightStatus, "reconciledAt")
		delete(leftStatus, "observedAt")
		delete(rightStatus, "observedAt")
		return reflect.DeepEqual(leftSpec, rightSpec) && reflect.DeepEqual(leftStatus, rightStatus)
	}
	return false
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

// NewClusterConfig returns a new instance of `argocd-util kubeconfig` command
func NewClusterConfig() *cobra.Command {
	var (
		clientConfig clientcmd.ClientConfig
	)
	var command = &cobra.Command{
		Use:   "kubeconfig CLUSTER_URL OUTPUT_PATH",
		Short: "Generates kubeconfig for the specified cluster",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			serverUrl := args[0]
			output := args[1]
			conf, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)
			kubeclientset, err := kubernetes.NewForConfig(conf)
			errors.CheckError(err)

			cluster, err := db.NewDB(namespace, settings.NewSettingsManager(context.Background(), kubeclientset, namespace), kubeclientset).GetCluster(context.Background(), serverUrl)
			errors.CheckError(err)
			err = kube.WriteKubeConfig(cluster.RawRestConfig(), namespace, output)
			errors.CheckError(err)
		},
	}
	clientConfig = cli.AddKubectlFlagsToCmd(command)
	return command
}

func iterateStringFields(obj interface{}, callback func(name string, val string) string) {
	if mapField, ok := obj.(map[string]interface{}); ok {
		for field, val := range mapField {
			if strVal, ok := val.(string); ok {
				mapField[field] = callback(field, strVal)
			} else {
				iterateStringFields(val, callback)
			}
		}
	} else if arrayField, ok := obj.([]interface{}); ok {
		for i := range arrayField {
			iterateStringFields(arrayField[i], callback)
		}
	}
}

func redactor(dirtyString string) string {
	config := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(dirtyString), &config)
	errors.CheckError(err)
	iterateStringFields(config, func(name string, val string) string {
		if name == "clientSecret" || name == "secret" || name == "bindPW" {
			return "********"
		} else {
			return val
		}
	})
	data, err := yaml.Marshal(config)
	errors.CheckError(err)
	return string(data)
}

func main() {
	if err := NewCommand().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
