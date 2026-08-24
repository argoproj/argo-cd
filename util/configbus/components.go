package configbus

// Component name constants for Provider.ForComponent and shared-key Get switch.
// Keep these in sync with ForComponent call sites across binaries.
const (
	ComponentController     = "controller"
	ComponentServer         = "server"
	ComponentReposerver     = "reposerver"
	ComponentApplicationset = "applicationset"
	ComponentNotifications  = "notifications"
	ComponentCommitserver   = "commitserver"
	ComponentShared         = "shared"
)
