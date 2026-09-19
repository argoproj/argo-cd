package configbus

// Component name constants label Argo CD processes in docs, metrics, and logs.
// They are not wired into Provider construction (no ForComponent).
const (
	ComponentController     = "controller"
	ComponentServer         = "server"
	ComponentReposerver     = "reposerver"
	ComponentApplicationset = "applicationset"
	ComponentNotifications  = "notifications"
	ComponentCommitserver   = "commitserver"
	ComponentShared         = "shared"
)
