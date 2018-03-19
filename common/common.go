package common

import (
	"github.com/argoproj/argo-cd/pkg/apis/application"
	rbacv1 "k8s.io/api/rbac/v1"
)

const (
	// MetadataPrefix is the prefix used for our labels and annotations
	MetadataPrefix = "argocd.argoproj.io"

	// SecretTypeRepository indicates a secret type of repository
	SecretTypeRepository = "repository"

	// SecretTypeCluster indicates a secret type of cluster
	SecretTypeCluster = "cluster"

	// SecretTypePassword indicates a password
	SecretTypePassword = "password"
)

var (
	// LabelKeyAppInstance refers to the application instance resource name
	LabelKeyAppInstance = MetadataPrefix + "/app-instance"

	// LabelKeySecretType contains the type of argocd secret (either 'cluster' or 'repo')
	LabelKeySecretType = MetadataPrefix + "/secret-type"

	// LabelKeyApplicationControllerInstanceID is the label which allows to separate application among multiple running application controllers.
	LabelKeyApplicationControllerInstanceID = application.ApplicationFullName + "/controller-instanceid"
	// LabelApplicationName is the label which indicates that resource belongs to application with the specified name
	LabelApplicationName = application.ApplicationFullName + "/app-name"
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
