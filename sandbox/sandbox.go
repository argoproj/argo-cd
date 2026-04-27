package sandbox

type SandboxImpl interface {
	Name() string
	Init(sandboxConfig ArgocdSandboxConfig) error
}
