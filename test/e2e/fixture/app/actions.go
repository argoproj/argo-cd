package app

import "github.com/argoproj/argo-cd/test/e2e/fixture"

// this implements the "when" part of given/when/then
//
// none of the func implement error checks, and that is complete intended, you should check for errors
// using the Then()
type Actions struct {
	context *Context
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

	_, _ = a.runCli(args...)

	return a
}

func (a *Actions) Sync() *Actions {
	_, _ = a.runCli("app", "sync", a.context.name, "--timeout", "5", "--prune")
	return a
}

func (a *Actions) TerminateOp() *Actions {
	_, _ = a.runCli("app", "terminate-op", a.context.name)
	return a
}

func (a *Actions) Patch(file string, jsonPath string) *Actions {
	fixture.Patch(a.context.path+"/"+file, jsonPath)
	return a
}

func (a *Actions) Delete(cascade bool) *Actions {
	args := []string{"app", "delete", a.context.name}
	if cascade {
		args = append(args, "--cascade")
	}
	_, _ = a.runCli(args...)
	return a
}

func (a *Actions) Then() *Consequences {
	return &Consequences{a.context, a}
}

func (a *Actions) runCli(args ...string) (output string, err error) {
	return fixture.RunCli(args...)
}
