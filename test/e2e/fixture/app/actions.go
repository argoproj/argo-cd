package app

import (
	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/test/e2e/fixture"
)

// this implements the "when" part of given/when/then
//
// none of the func implement error checks, and that is complete intended, you should check for errors
// using the Then()
type Actions struct {
	context    *Context
	lastOutput string
	lastError  error
}

func (a *Actions) Create() *Actions {

	args := []string{
		"app", "create", a.context.name,
		"--repo", fixture.RepoURL(),
		"--path", a.context.path,
		"--dest-server", a.context.destServer,
		"--dest-namespace", fixture.DeploymentNamespace(),
	}

	if a.context.env != "" {
		args = append(args, "--env", a.context.env)
	}

	for _, parameter := range a.context.parameters {
		args = append(args, "--parameter", parameter)
	}

	if a.context.namePrefix != "" {
		args = append(args, "--nameprefix", a.context.namePrefix)
	}

	a.runCli(args...)

	return a
}
func (a *Actions) PatchApp(patch string) *Actions {
	a.runCli("app", "patch", a.context.name, "--patch", patch)
	return a
}

func (a *Actions) Sync() *Actions {
	a.runCli("app", "sync", a.context.name, "--timeout", "5", "--prune")
	return a
}

func (a *Actions) TerminateOp() *Actions {
	a.runCli("app", "terminate-op", a.context.name)
	return a
}

func (a *Actions) Patch(file string, jsonPath string) *Actions {
	fixture.Patch(a.context.path+"/"+file, jsonPath)
	return a
}

func (a *Actions) Refresh(refreshType RefreshType) *Actions {

	flag := map[RefreshType]string{
		RefreshTypeNormal: "--refresh",
		RefreshTypeHard:   "--hard-refresh",
	}[refreshType]

	a.runCli("app", "get", a.context.name, flag)

	return a
}

func (a *Actions) Delete(cascade bool) *Actions {
	args := []string{"app", "delete", a.context.name}
	if cascade {
		args = append(args, "--cascade")
	}
	a.runCli(args...)
	return a
}

func (a *Actions) Then() *Consequences {
	return &Consequences{a.context, a}
}

func (a *Actions) runCli(args ...string) {
	a.lastOutput, a.lastError = fixture.RunCli(args...)
}
