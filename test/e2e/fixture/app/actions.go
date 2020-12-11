package app

import (
	"fmt"
	"io/ioutil"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/test/e2e/fixture"
	"github.com/argoproj/argo-cd/util/errors"
	"github.com/argoproj/argo-cd/util/grpc"
)

// this implements the "when" part of given/when/then
//
// none of the func implement error checks, and that is complete intended, you should check for errors
// using the Then()
type Actions struct {
	context      *Context
	lastOutput   string
	lastError    error
	ignoreErrors bool
}

func (a *Actions) IgnoreErrors() *Actions {
	a.ignoreErrors = true
	return a
}

func (a *Actions) DoNotIgnoreErrors() *Actions {
	a.ignoreErrors = false
	return a
}

func (a *Actions) PatchFile(file string, jsonPath string) *Actions {
	a.context.t.Helper()
	fixture.Patch(a.context.path+"/"+file, jsonPath)
	return a
}

func (a *Actions) DeleteFile(file string) *Actions {
	a.context.t.Helper()
	fixture.Delete(a.context.path + "/" + file)
	return a
}

func (a *Actions) WriteFile(fileName, fileContents string) *Actions {
	a.context.t.Helper()
	fixture.WriteFile(a.context.path+"/"+fileName, fileContents)
	return a
}

func (a *Actions) AddFile(fileName, fileContents string) *Actions {
	a.context.t.Helper()
	fixture.AddFile(a.context.path+"/"+fileName, fileContents)
	return a
}

func (a *Actions) AddSignedFile(fileName, fileContents string) *Actions {
	a.context.t.Helper()
	fixture.AddSignedFile(a.context.path+"/"+fileName, fileContents)
	return a
}

func (a *Actions) CreateFromPartialFile(data string, flags ...string) *Actions {
	a.context.t.Helper()
	tmpFile, err := ioutil.TempFile("", "")
	errors.CheckError(err)
	_, err = tmpFile.Write([]byte(data))
	errors.CheckError(err)

	args := append([]string{
		"app", "create",
		"-f", tmpFile.Name(),
		"--name", a.context.name,
		"--repo", fixture.RepoURL(a.context.repoURLType),
		"--dest-server", a.context.destServer,
		"--dest-namespace", fixture.DeploymentNamespace(),
	}, flags...)

	a.runCli(args...)
	return a
}
func (a *Actions) CreateFromFile(handler func(app *Application), flags ...string) *Actions {
	a.context.t.Helper()
	app := &Application{
		ObjectMeta: v1.ObjectMeta{
			Name: a.context.name,
		},
		Spec: ApplicationSpec{
			Project: a.context.project,
			Source: ApplicationSource{
				RepoURL: fixture.RepoURL(a.context.repoURLType),
				Path:    a.context.path,
			},
			Destination: ApplicationDestination{
				Server:    a.context.destServer,
				Namespace: fixture.DeploymentNamespace(),
			},
		},
	}
	if a.context.env != "" {
		app.Spec.Source.Ksonnet = &ApplicationSourceKsonnet{
			Environment: a.context.env,
		}
	}
	if a.context.namePrefix != "" || a.context.nameSuffix != "" {
		app.Spec.Source.Kustomize = &ApplicationSourceKustomize{
			NamePrefix: a.context.namePrefix,
			NameSuffix: a.context.nameSuffix,
		}
	}
	if a.context.configManagementPlugin != "" {
		app.Spec.Source.Plugin = &ApplicationSourcePlugin{
			Name: a.context.configManagementPlugin,
		}
	}

	if len(a.context.parameters) > 0 {
		log.Fatal("Application parameters or json tlas are not supported")
	}

	if a.context.directoryRecurse {
		app.Spec.Source.Directory = &ApplicationSourceDirectory{Recurse: true}
	}

	handler(app)
	data := grpc.MustMarshal(app)
	tmpFile, err := ioutil.TempFile("", "")
	errors.CheckError(err)
	_, err = tmpFile.Write(data)
	errors.CheckError(err)

	args := append([]string{
		"app", "create",
		"-f", tmpFile.Name(),
	}, flags...)

	a.runCli(args...)
	return a
}

func (a *Actions) CreateWithNoNameSpace(args ...string) *Actions {
	args = a.prepareCreateArgs(args)
	//  are you adding new context values? if you only use them for this func, then use args instead
	a.runCli(args...)
	return a
}

func (a *Actions) Create(args ...string) *Actions {
	args = a.prepareCreateArgs(args)
	args = append(args, "--dest-namespace", fixture.DeploymentNamespace())

	//  are you adding new context values? if you only use them for this func, then use args instead
	a.runCli(args...)

	return a
}

func (a *Actions) prepareCreateArgs(args []string) []string {
	a.context.t.Helper()
	args = append([]string{
		"app", "create", a.context.name,
		"--repo", fixture.RepoURL(a.context.repoURLType),
		"--dest-server", a.context.destServer,
	}, args...)

	if a.context.path != "" {
		args = append(args, "--path", a.context.path)
	}

	if a.context.chart != "" {
		args = append(args, "--helm-chart", a.context.chart)
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

	if a.context.nameSuffix != "" {
		args = append(args, "--namesuffix", a.context.nameSuffix)
	}

	if a.context.configManagementPlugin != "" {
		args = append(args, "--config-management-plugin", a.context.configManagementPlugin)
	}

	if a.context.revision != "" {
		args = append(args, "--revision", a.context.revision)
	}
	return args
}

func (a *Actions) Declarative(filename string) *Actions {
	a.context.t.Helper()
	return a.DeclarativeWithCustomRepo(filename, fixture.RepoURL(a.context.repoURLType))
}

func (a *Actions) DeclarativeWithCustomRepo(filename string, repoURL string) *Actions {
	a.context.t.Helper()
	values := map[string]interface{}{
		"ArgoCDNamespace":     fixture.ArgoCDNamespace,
		"DeploymentNamespace": fixture.DeploymentNamespace(),
		"Name":                a.context.name,
		"Path":                a.context.path,
		"Project":             a.context.project,
		"RepoURL":             repoURL,
	}
	a.lastOutput, a.lastError = fixture.Declarative(filename, values)
	a.verifyAction()
	return a
}

func (a *Actions) PatchApp(patch string) *Actions {
	a.context.t.Helper()
	a.runCli("app", "patch", a.context.name, "--patch", patch)
	return a
}

func (a *Actions) AppSet(flags ...string) *Actions {
	a.context.t.Helper()
	args := []string{"app", "set", a.context.name}
	args = append(args, flags...)
	a.runCli(args...)
	return a
}

func (a *Actions) AppUnSet(flags ...string) *Actions {
	a.context.t.Helper()
	args := []string{"app", "unset", a.context.name}
	args = append(args, flags...)
	a.runCli(args...)
	return a
}

func (a *Actions) Sync(args ...string) *Actions {
	a.context.t.Helper()
	args = append([]string{"app", "sync"}, args...)
	if a.context.name != "" {
		args = append(args, a.context.name)
	}
	args = append(args, "--timeout", fmt.Sprintf("%v", a.context.timeout))

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

	if a.context.force {
		args = append(args, "--force")
	}

	//  are you adding new context values? if you only use them for this func, then use args instead

	a.runCli(args...)

	return a
}

func (a *Actions) TerminateOp() *Actions {
	a.context.t.Helper()
	a.runCli("app", "terminate-op", a.context.name)
	return a
}

func (a *Actions) Refresh(refreshType RefreshType) *Actions {
	a.context.t.Helper()
	flag := map[RefreshType]string{
		RefreshTypeNormal: "--refresh",
		RefreshTypeHard:   "--hard-refresh",
	}[refreshType]

	a.runCli("app", "get", a.context.name, flag)

	return a
}

func (a *Actions) Delete(cascade bool) *Actions {
	a.context.t.Helper()
	a.runCli("app", "delete", a.context.name, fmt.Sprintf("--cascade=%v", cascade), "--yes")
	return a
}

func (a *Actions) And(block func()) *Actions {
	a.context.t.Helper()
	block()
	return a
}

func (a *Actions) Then() *Consequences {
	a.context.t.Helper()
	return &Consequences{a.context, a}
}

func (a *Actions) runCli(args ...string) {
	a.context.t.Helper()
	a.lastOutput, a.lastError = fixture.RunCli(args...)
	a.verifyAction()
}

func (a *Actions) verifyAction() {
	a.context.t.Helper()
	if !a.ignoreErrors {
		a.Then().Expect(Success(""))
	}
}
