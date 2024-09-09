package admin

import (
	"context"
	"reflect"
	"strings"

	"github.com/spf13/cobra"
	apiv1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"

	cmdutil "github.com/argoproj/argo-cd/v2/cmd/util"
	"github.com/argoproj/argo-cd/v2/common"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/settings"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application"
)

const (
	// YamlSeparator separates sections of a YAML file
	yamlSeparator = "---\n"
)

var (
	configMapResource       = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	secretResource          = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
	applicationsResource    = schema.GroupVersionResource{Group: application.Group, Version: "v1alpha1", Resource: application.ApplicationPlural}
	appprojectsResource     = schema.GroupVersionResource{Group: application.Group, Version: "v1alpha1", Resource: application.AppProjectPlural}
	appplicationSetResource = schema.GroupVersionResource{Group: application.Group, Version: "v1alpha1", Resource: application.ApplicationSetPlural}
)

// NewAdminCommand returns a new instance of an argocd command
func NewAdminCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	pathOpts := clientcmd.NewDefaultPathOptions()

	command := &cobra.Command{
		Use:               "admin",
		Short:             "Contains a set of commands useful for Argo CD administrators and requires direct Kubernetes access",
		DisableAutoGenTag: true,
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
		Example: `# Access the Argo CD web UI
$ argocd admin dashboard

# Reset the initial admin password
$ argocd admin initial-password reset
`,
	}

	command.AddCommand(NewClusterCommand(clientOpts, pathOpts))
	command.AddCommand(NewProjectsCommand())
	command.AddCommand(NewSettingsCommand())
	command.AddCommand(NewAppCommand(clientOpts))
	command.AddCommand(NewRepoCommand())
	command.AddCommand(NewImportCommand())
	command.AddCommand(NewExportCommand())
	command.AddCommand(NewDashboardCommand(clientOpts))
	command.AddCommand(NewNotificationsCommand())
	command.AddCommand(NewInitialPasswordCommand())
	command.AddCommand(NewRedisInitialPasswordCommand())

	command.Flags().StringVar(&cmdutil.LogFormat, "logformat", "text", "Set the logging format. One of: text|json")
	command.Flags().StringVar(&cmdutil.LogLevel, "loglevel", "info", "Set the logging level. One of: debug|info|warn|error")
	return command
}

type argoCDClientsets struct {
	configMaps      dynamic.ResourceInterface
	secrets         dynamic.ResourceInterface
	applications    dynamic.ResourceInterface
	projects        dynamic.ResourceInterface
	applicationSets dynamic.ResourceInterface
}

func newArgoCDClientsets(config *rest.Config, namespace string) *argoCDClientsets {
	dynamicIf, err := dynamic.NewForConfig(config)
	errors.CheckError(err)
	return &argoCDClientsets{
		configMaps: dynamicIf.Resource(configMapResource).Namespace(namespace),
		secrets:    dynamicIf.Resource(secretResource).Namespace(namespace),
		// To support applications and applicationsets in any namespace we will watch all namespaces and filter them afterwards
		applications:    dynamicIf.Resource(applicationsResource),
		projects:        dynamicIf.Resource(appprojectsResource).Namespace(namespace),
		applicationSets: dynamicIf.Resource(appplicationSetResource),
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
	leftAnnotation := left.GetAnnotations()
	rightAnnotation := right.GetAnnotations()
	delete(leftAnnotation, apiv1.LastAppliedConfigAnnotation)
	delete(rightAnnotation, apiv1.LastAppliedConfigAnnotation)
	if !reflect.DeepEqual(leftAnnotation, rightAnnotation) {
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
	case application.AppProjectKind:
		leftSpec, _, _ := unstructured.NestedMap(left.Object, "spec")
		rightSpec, _, _ := unstructured.NestedMap(right.Object, "spec")
		return reflect.DeepEqual(leftSpec, rightSpec)
	case application.ApplicationKind:
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

type argocdAdditonalNamespaces struct {
	applicationNamespaces    []string
	applicationsetNamespaces []string
}

const (
	applicationsetNamespacesCmdParamsKey = "applicationsetcontroller.namespaces"
	applicationNamespacesCmdParamsKey    = "application.namespaces"
)

// Get additional namespaces from argocd-cmd-params
func getAdditionalNamespaces(ctx context.Context, argocdClientsets *argoCDClientsets) *argocdAdditonalNamespaces {
	applicationNamespaces := make([]string, 0)
	applicationsetNamespaces := make([]string, 0)

	un, err := argocdClientsets.configMaps.Get(ctx, common.ArgoCDCmdParamsConfigMapName, v1.GetOptions{})
	errors.CheckError(err)
	var cm apiv1.ConfigMap
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(un.Object, &cm)
	errors.CheckError(err)

	namespacesListFromString := func(namespaces string) []string {
		listOfNamespaces := []string{}

		ss := strings.Split(namespaces, ",")

		for _, namespace := range ss {
			if namespace != "" {
				listOfNamespaces = append(listOfNamespaces, strings.TrimSpace(namespace))
			}
		}

		return listOfNamespaces
	}

	if strNamespaces, ok := cm.Data[applicationNamespacesCmdParamsKey]; ok {
		applicationNamespaces = namespacesListFromString(strNamespaces)
	}

	if strNamespaces, ok := cm.Data[applicationsetNamespacesCmdParamsKey]; ok {
		applicationsetNamespaces = namespacesListFromString(strNamespaces)
	}

	return &argocdAdditonalNamespaces{
		applicationNamespaces:    applicationNamespaces,
		applicationsetNamespaces: applicationsetNamespaces,
	}
}
