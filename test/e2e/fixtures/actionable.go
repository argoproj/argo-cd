package fixtures

import (
	"strconv"
	"strings"
)

type Actionable struct {
	context *Context
}

func (a *Actionable) Create() *Actionable {

	if a.context.name == "" {
		a.context.name = strings.ReplaceAll(a.context.path, "/", "-")
	}

	args := []string{
		"app", "create", a.context.name,
		"--repo", a.context.fixture.RepoURL(),
		"--path", a.context.path,
		"--dest-server", a.context.destServer,
		"--dest-namespace", a.context.fixture.DeploymentNamespace,
		"--env", a.context.env,
	}

	for _, parameter := range a.context.parameters {
		args = append(args, "--parameter", parameter)
	}

	_, _ = a.runCli(args...)

	return a
}

func (a *Actionable) Sync() *Actionable {
	_, _ = a.runCli("app", "sync", a.context.name, "--timeout", "5")
	return a
}

func (a *Actionable) TerminateOp() *Actionable {
	_, _ = a.runCli("app", "terminate-op", a.context.name)
	return a
}

func (a *Actionable) Patch(file string, jsonPath string) *Actionable {
	a.context.fixture.Patch(a.context.path+"/"+file, jsonPath)
	return a
}

func (a *Actionable) Delete(cascade bool) *Actionable {
	_, _ = a.runCli("app", "delete", a.context.name, "--cascade", strconv.FormatBool(cascade))
	return a
}

func (a *Actionable) runCli(args ...string) (output string, err error) {
	return a.context.fixture.RunCli(args...)
}

func (a *Actionable) Then() *Consequences {
	return &Consequences{a.context, a}
}
