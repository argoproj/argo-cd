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

	// AuthCookieName is the HTTP cookie name where we store our auth token
	AuthCookieName = "argocd.argoproj.io/auth-token"
)

const (
	ArgoCDAdminUsername = "admin"
	ArgoCDSecretName    = "argocd-secret"
	ArgoCDConfigMapName = "argocd-cm"
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

	// AnnotationKeyRefresh is the annotation key in the application which is updated with an
	// arbitrary value (i.e. timestamp) on a git event, to  force the controller to wake up and
	// re-evaluate the application
	AnnotationKeyRefresh = application.ApplicationFullName + "/refresh"
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
