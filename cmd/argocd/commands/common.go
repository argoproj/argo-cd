package commands

const (
	// cliName is the name of the CLI
	cliName = "argocd"

	// defaultArgoCDConfigMap is the default name of the argocd configmap
	defaultArgoCDConfigMap = "argocd-configmap"
)

var (
	// Parts of the image for installation
	// These values may be overridden by the link flags during build
	imageNamespace = "argoproj"
	imageTag       = "latest"
)
