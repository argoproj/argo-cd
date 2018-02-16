package commands

const (
	cliName = "argocd"
)

var (
	// Parts of the image for installation
	// These values may be overridden by the link flags during build
	imageNamespace = "argoproj"
	imageTag       = "latest"
)
