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
	AuthCookieName = "argocd.token"
	// ResourcesFinalizerName is a number of application CRD finalizer
	ResourcesFinalizerName = "resources-finalizer." + MetadataPrefix
)

const (
	ArgoCDAdminUsername = "admin"
	ArgoCDSecretName    = "argocd-secret"
	ArgoCDConfigMapName = "argocd-cm"
)

const (
	// DexAPIEndpoint is the endpoint where we serve the Dex API server
	DexAPIEndpoint = "/api/dex"
	// LoginEndpoint is ArgoCD's shorthand login endpoint which redirects to dex's OAuth 2.0 provider's consent page
	LoginEndpoint = "/auth/login"
	// CallbackEndpoint is ArgoCD's final callback endpoint we reach after OAuth 2.0 login flow has been completed
	CallbackEndpoint = "/auth/callback"
	// ArgoCDClientAppName is name of the Oauth client app used when registering our web app to dex
	ArgoCDClientAppName = "ArgoCD"
	// ArgoCDClientAppID is the Oauth client ID we will use when registering our app to dex
	ArgoCDClientAppID = "argo-cd"
	// ArgoCDCLIClientAppName is name of the Oauth client app used when registering our CLI to dex
	ArgoCDCLIClientAppName = "ArgoCD CLI"
	// ArgoCDCLIClientAppID is the Oauth client ID we will use when registering our CLI to dex
	ArgoCDCLIClientAppID = "argo-cd-cli"
	// EnvVarSSODebug is an environment variable to enable additional OAuth debugging in the API server
	EnvVarSSODebug = "ARGOCD_SSO_DEBUG"
)

var (
	// LabelKeyAppInstance refers to the application instance resource name
	LabelKeyAppInstance = MetadataPrefix + "/app-instance"

	// LabelKeySecretType contains the type of argocd secret (either 'cluster' or 'repo')
	LabelKeySecretType = MetadataPrefix + "/secret-type"

	// AnnotationConnectionStatus contains connection state status
	AnnotationConnectionStatus = MetadataPrefix + "/connection-status"
	// AnnotationConnectionMessage contains additional information about connection status
	AnnotationConnectionMessage = MetadataPrefix + "/connection-message"
	// AnnotationConnectionMessage contains timestamp when connection state was collected
	AnnotationConnectionAttemptedAt = MetadataPrefix + "/connection-attempted-at"

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
