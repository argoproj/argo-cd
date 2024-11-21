package healthz

const (
	// catches corrupted informer state; see https://github.com/argoproj/argo-cd/issues/4960 for more information
	NotObjectErrMsg string = "object does not implement the Object interfaces"
)
