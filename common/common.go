package common

const (
	// MetadataPrefix is the prefix used for our labels and annotations
	MetadataPrefix = "argocd.argoproj.io"

	// SecretTypeRepository indicates the data type which argocd stores as a k8s secret
	SecretTypeRepository = "repository"
	// DefaultControllerDeploymentName is the default deployment name of the applicaiton controller
	DefaultControllerDeploymentName = "application-controller"
	// DefaultControllerNamespace is the default namespace where the applicaiton controller is installed
	DefaultControllerNamespace = "kube-system"
)

var (
	// LabelKeySecretType contains the type of argocd secret (currently this is just 'repo')
	LabelKeySecretType = MetadataPrefix + "/secret-type"
)
