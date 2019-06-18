package app

import (
	"fmt"

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

func (a *Actions) PatchFile(file string, jsonPath string) *Actions {
	fixture.Patch(a.context.path+"/"+file, jsonPath)
	return a
}

func (a *Actions) DeleteFile(file string) *Actions {
	fixture.Delete(a.context.path + "/" + file)
	return a
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

	args = append(args, "--project", a.context.project)

	if a.context.namePrefix != "" {
		args = append(args, "--nameprefix", a.context.namePrefix)
	}

	if a.context.configManagementPlugin != "" {
		args = append(args, "--config-management-plugin", a.context.configManagementPlugin)
	}

	a.runCli(args...)

	return a
}

func (a *Actions) Declarative(filename string) *Actions {
	values := map[string]interface{}{
		"ArgoCDNamespace":     fixture.ArgoCDNamespace,
		"DeploymentNamespace": fixture.DeploymentNamespace(),
		"Name":                a.context.name,
		"Path":                a.context.path,
		"Project":             a.context.project,
		"RepoURL":             fixture.RepoURL(),
	}
	a.lastOutput, a.lastError = fixture.Declarative(filename, values)
	return a
}

func (a *Actions) PatchApp(patch string) *Actions {
	a.runCli("app", "patch", a.context.name, "--patch", patch)
	return a
}

func (a *Actions) Sync() *Actions {
	args := []string{"app", "sync", a.context.name, "--timeout", fmt.Sprintf("%v", a.context.timeout)}

	if a.context.async {
		args = append(args, "--async")
	}

	if a.context.prune {
		args = append(args, "--prune")
	}

	if a.context.resource != "" {
		args = append(args, "--resource", a.context.resource)
	}

	if a.context.localPath != "" {
		args = append(args, "--local", a.context.localPath)
	}

	a.runCli(args...)
	return a
}

func (a *Actions) TerminateOp() *Actions {
	a.runCli("app", "terminate-op", a.context.name)
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
	a.runCli("app", "delete", a.context.name, fmt.Sprintf("--cascade=%v", cascade))
	return a
}

func (a *Actions) And(block func()) *Actions {
	block()
	return a
}

func (a *Actions) Then() *Consequences {
	return &Consequences{a.context, a}
}

func (a *Actions) runCli(args ...string) {
	a.lastOutput, a.lastError = fixture.RunCli(args...)
}
