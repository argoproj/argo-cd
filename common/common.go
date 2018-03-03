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

	// DefaultServerDeploymentName is the default deployment name of the api server
	DefaultServerDeploymentName = "argocd-server"

	// DefaultServerServiceName is the default service name of the api server
	DefaultServerServiceName = "argocd-server"

	// DefaultArgoCDNamespace is the default namespace where Argo CD will be installed
	DefaultArgoCDNamespace = "argocd"
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

const (
	ArgoCDServerServiceAccount = "argocd-server"
	ArgoCDServerRole           = "argocd-server-role"
	ArgoCDServerRoleBinding    = "argocd-server-role-binding"
)

var ArgoCDServerPolicyRules = []rbacv1.PolicyRule{
	{
		APIGroups: []string{""},
		Resources: []string{"pods", "pods/exec", "pods/log"},
		Verbs:     []string{"get", "list", "watch"},
	},
	{
		APIGroups: []string{""},
		Resources: []string{"secrets"},
		Verbs:     []string{"create", "get", "list", "watch", "update", "patch", "delete"},
	},
	{
		APIGroups: []string{"argoproj.io"},
		Resources: []string{"applications"},
		Verbs:     []string{"create", "get", "list", "watch", "update", "patch", "delete"},
	},
}

const (
	ApplicationControllerServiceAccount = "application-controller"
	ApplicationControllerRole           = "application-controller-role"
	ApplicationControllerRoleBinding    = "application-controller-role-binding"
)

var ApplicationControllerPolicyRules = []rbacv1.PolicyRule{
	{
		APIGroups: []string{""},
		Resources: []string{"secrets"},
		Verbs:     []string{"get"},
	},
	{
		APIGroups: []string{"argoproj.io"},
		Resources: []string{"applications"},
		Verbs:     []string{"create", "get", "list", "watch", "update", "patch", "delete"},
	},
}
