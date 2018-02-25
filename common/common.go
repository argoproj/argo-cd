package common

const (
	// MetadataPrefix is the prefix used for our labels and annotations
	MetadataPrefix = "argocd.argoproj.io"

	// SecretTypeRepository indicates the data type which argocd stores as a k8s secret
	SecretTypeRepository = "repository"
	// DefaultControllerDeploymentName is the default deployment name of the application controller
	DefaultControllerDeploymentName = "application-controller"

	// DefaultControllerNamespace is the default namespace where the application controller is installed
	DefaultControllerNamespace = "kube-system"
)

var (
	// LabelKeyAppInstance refers to the application instance resource name
	LabelKeyAppInstance = MetadataPrefix + "/app-instance"

	// LabelKeySecretType contains the type of argocd secret (currently this is just 'repo')
	LabelKeySecretType = MetadataPrefix + "/secret-type"
)
