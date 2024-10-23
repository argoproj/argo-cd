package v1alpha1

const (
	// AnnotationKeyRefresh is the annotation key which indicates that app needs to be refreshed. Removed by application controller after app is refreshed.
	// Might take values 'normal'/'hard'. Value 'hard' means manifest cache and target cluster state cache should be invalidated before refresh.
	AnnotationKeyRefresh string = "argocd.argoproj.io/refresh"

	// AnnotationKeyManifestGeneratePaths is an annotation that contains a list of semicolon-separated paths in the
	// manifests repository that affects the manifest generation. Paths might be either relative or absolute. The
	// absolute path means an absolute path within the repository and the relative path is relative to the application
	// source path within the repository.
	AnnotationKeyManifestGeneratePaths = "argocd.argoproj.io/manifest-generate-paths"

	// AnnotationAppRefreshTimeout is an optional annotation for applications to override the default refresh timeout
	// the provided value is evaluated as seconds and after the timeout is reached the app gets soft refreshed
	AnnotationAppRefreshTimeout string = "argocd.argoproj.io/refresh-timeout"

	// AnnotationAppRefreshTimeout is an optional annotation for applications to override the default hard refresh timeout
	// the provided value is evaluated as seconds and after the timeout is reached the app gets hard refreshed
	AnnotationAppHardRefreshTimeout string = "argocd.argoproj.io/hard-refresh-timeout"
)
