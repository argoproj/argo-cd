package commands

import (
	"reflect"

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	cmdutil "github.com/argoproj/argo-cd/cmd/util"
	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/errors"
	"github.com/argoproj/argo-cd/util/settings"
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
		pathOpts = clientcmd.NewDefaultPathOptions()
	)

	var command = &cobra.Command{
		Use:               cliName,
		Short:             "argocd-util tools used by Argo CD",
		Long:              "argocd-util has internal utility tools used by Argo CD",
		DisableAutoGenTag: true,
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}

	command.AddCommand(cli.NewVersionCmd(cliName))
	command.AddCommand(NewClusterCommand(pathOpts))
	command.AddCommand(NewProjectsCommand())
	command.AddCommand(NewSettingsCommand())
	command.AddCommand(NewAppCommand())
	command.AddCommand(NewRepoCommand())
	command.AddCommand(NewImportCommand())
	command.AddCommand(NewExportCommand())

	command.Flags().StringVar(&cmdutil.LogFormat, "logformat", "text", "Set the logging format. One of: text|json")
	command.Flags().StringVar(&cmdutil.LogLevel, "loglevel", "info", "Set the logging level. One of: debug|info|warn|error")
	return command
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
