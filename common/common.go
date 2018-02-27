package common

import "github.com/argoproj/argo-cd/pkg/apis/application"
import rbacv1 "k8s.io/api/rbac/v1"

const (
	// MetadataPrefix is the prefix used for our labels and annotations
	MetadataPrefix = "argocd.argoproj.io"

	// SecretTypeRepository indicates a secret type of repository
	SecretTypeRepository = "repository"

	// SecretTypeCluster indicates a secret type of cluster
	SecretTypeCluster = "cluster"

	// DefaultControllerDeploymentName is the default deployment name of the application controller
	DefaultControllerDeploymentName = "application-controller"

	// DefaultControllerNamespace is the default namespace where the application controller is installed
	DefaultControllerNamespace = "kube-system"
)

var (
	// LabelKeyAppInstance refers to the application instance resource name
	LabelKeyAppInstance = MetadataPrefix + "/app-instance"

	// LabelKeySecretType contains the type of argocd secret (either 'cluster' or 'repo')
	LabelKeySecretType = MetadataPrefix + "/secret-type"
	// LabelKeyApplicationControllerInstanceID is the label which allows to separate application among multiple running application controllers.
	LabelKeyApplicationControllerInstanceID = application.ApplicationFullName + "/controller-instanceid"
)

// ArgoCDManagerServiceAccount is the name of the service account for managing a cluster
const (
	ArgoCDManagerServiceAccount     = "argocd-manager"
	ArgoCDManagerClusterRole        = "argocd-manager-role"
	ArgoCDManagerClusterRoleBinding = "argocd-manager-role-binding"
)

// ArgoCDManagerPolicyRules are the policies to give argocd-manager
var ArgoCDManagerPolicyRules = []rbacv1.PolicyRule{
	{
		APIGroups: []string{"*"},
		Resources: []string{"*"},
		Verbs:     []string{"*"},
	},
}
