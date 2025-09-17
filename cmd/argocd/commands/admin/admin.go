package admin

import (
	"context"
	"reflect"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	cmdutil "github.com/argoproj/argo-cd/v3/cmd/util"
	"github.com/argoproj/argo-cd/v3/common"
	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application"
	"github.com/argoproj/argo-cd/v3/util/errors"
)

const (
	// YamlSeparator separates sections of a YAML file
	yamlSeparator = "---\n"

	applicationsetNamespacesCmdParamsKey = "applicationsetcontroller.namespaces"
	applicationNamespacesCmdParamsKey    = "application.namespaces"
)

var (
	configMapResource       = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	secretResource          = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
	applicationsResource    = schema.GroupVersionResource{Group: application.Group, Version: "v1alpha1", Resource: application.ApplicationPlural}
	appprojectsResource     = schema.GroupVersionResource{Group: application.Group, Version: "v1alpha1", Resource: application.AppProjectPlural}
	appplicationSetResource = schema.GroupVersionResource{Group: application.Group, Version: "v1alpha1", Resource: application.ApplicationSetPlural}
)

type argocdAdditionalNamespaces struct {
	applicationNamespaces    []string
	applicationsetNamespaces []string
}

type argoCDClientsets struct {
	configMaps      dynamic.ResourceInterface
	secrets         dynamic.ResourceInterface
	applications    dynamic.ResourceInterface
	projects        dynamic.ResourceInterface
	applicationSets dynamic.ResourceInterface
}

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

	command.Flags().StringVar(&cmdutil.LogFormat, "logformat", "json", "Set the logging format. One of: json|text")
	command.Flags().StringVar(&cmdutil.LogLevel, "loglevel", "info", "Set the logging level. One of: debug|info|warn|error")
	return command
}

func newArgoCDClientsets(config *rest.Config, namespace string) *argoCDClientsets {
	dynamicIf, err := dynamic.NewForConfig(config)
	errors.CheckError(err)

	return &argoCDClientsets{
		configMaps:      dynamicIf.Resource(configMapResource).Namespace(namespace),
		secrets:         dynamicIf.Resource(secretResource).Namespace(namespace),
		applications:    dynamicIf.Resource(applicationsResource).Namespace(namespace),
		projects:        dynamicIf.Resource(appprojectsResource).Namespace(namespace),
		applicationSets: dynamicIf.Resource(appplicationSetResource).Namespace(namespace),
	}
}

// isArgoCDSecret returns whether or not the given secret is a part of Argo CD configuration
// (e.g. argocd-secret, repo credentials, or cluster credentials)
func isArgoCDSecret(un unstructured.Unstructured) bool {
	secretName := un.GetName()
	if secretName == common.ArgoCDSecretName {
		return true
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
	delete(leftAnnotation, corev1.LastAppliedConfigAnnotation)
	delete(rightAnnotation, corev1.LastAppliedConfigAnnotation)
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

// Get additional namespaces from argocd-cmd-params
func getAdditionalNamespaces(ctx context.Context, configMapsClient dynamic.ResourceInterface) *argocdAdditionalNamespaces {
	applicationNamespaces := make([]string, 0)
	applicationsetNamespaces := make([]string, 0)

	un, err := configMapsClient.Get(ctx, common.ArgoCDCmdParamsConfigMapName, metav1.GetOptions{})
	errors.CheckError(err)
	var cm corev1.ConfigMap
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

	return &argocdAdditionalNamespaces{
		applicationNamespaces:    applicationNamespaces,
		applicationsetNamespaces: applicationsetNamespaces,
	}
}
