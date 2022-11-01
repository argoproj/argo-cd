package imageUpdater

import (
	"context"
	"fmt"
	"os"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture/applicationsets/utils"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/grpc"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// this implements the "when" part of given/when/then
//
// none of the func implement error checks, and that is complete intended, you should check for errors
// using the Then()
type Actions struct {
	context        *Context
	lastOutput     string
	lastError      error
	describeAction string
	ignoreErrors   bool
}

// IgnoreErrors sets whether to ignore
func (a *Actions) IgnoreErrors() *Actions {
	a.ignoreErrors = true
	return a
}

func (a *Actions) DoNotIgnoreErrors() *Actions {
	a.ignoreErrors = false
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

// Update retrieves the latest copy the Application, then allows the caller to mutate it via 'toUpdate', with
// the result applied back to the cluster resource
func (a *Actions) Update(toUpdate func(*v1alpha1.Application)) *Actions {
	a.context.t.Helper()

	timeout := 30 * time.Second

	var mostRecentError error

	for start := time.Now(); time.Since(start) < timeout; time.Sleep(3 * time.Second) {

		app, err := a.get()
		mostRecentError = err
		if err == nil {
			// Keep trying to update until it succeeds, or the test times out
			toUpdate(app)
			a.describeAction = fmt.Sprintf("updating Application '%s'", app.Name)

			fixtureClient := utils.GetE2EFixtureK8sClient()
			_, err := fixtureClient.AppClientset.ArgoprojV1alpha1().Applications(a.context.appNamespace).Update(context.TODO(), app, metav1.UpdateOptions{})

			if err != nil {
				mostRecentError = err
			} else {
				mostRecentError = nil
				break
			}
		}
	}

	a.lastOutput, a.lastError = "", mostRecentError
	a.verifyAction()

	return a
}

// get retrieves the Application (by name) that was created by an earlier Create action
func (a *Actions) get() (*v1alpha1.Application, error) {

	fixtureClient := utils.GetE2EFixtureK8sClient()
	app, err := fixtureClient.AppClientset.ArgoprojV1alpha1().Applications(a.context.appNamespace).Get(context.TODO(), a.context.name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return app, nil
}

func (a *Actions) verifyAction() {
	a.context.t.Helper()

	if a.describeAction != "" {
		log.Infof("action: %s", a.describeAction)
		a.describeAction = ""
	}

	if !a.ignoreErrors {
		a.Then().Expect(Success(""))
	}

}

func (a *Actions) CreateApp(args ...string) *Actions {
	args = a.prepareCreateAppArgs(args)
	args = append(args, "--dest-namespace", fixture.DeploymentNamespace())

	//  are you adding new context values? if you only use them for this func, then use args instead
	a.runCli(args...)

	return a
}

func (a *Actions) runCli(args ...string) {
	a.context.t.Helper()
	a.lastOutput, a.lastError = fixture.RunCli(args...)
	a.verifyAction()
}

func (a *Actions) prepareCreateAppArgs(args []string) []string {
	a.context.t.Helper()
	args = append([]string{
		"app", "create", a.context.AppQualifiedName(),
		"--repo", fixture.RepoURL(a.context.repoURLType),
	}, args...)

	if a.context.destName != "" {
		args = append(args, "--dest-name", a.context.destName)
	} else {
		args = append(args, "--dest-server", a.context.destServer)
	}
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
	if a.context.helmPassCredentials {
		args = append(args, "--helm-pass-credentials")
	}
	if a.context.helmSkipCrds {
		args = append(args, "--helm-skip-crds")
	}
	return args
}

func (a *Actions) Sync(args ...string) *Actions {
	a.context.t.Helper()
	args = append([]string{"app", "sync"}, args...)
	if a.context.name != "" {
		args = append(args, a.context.AppQualifiedName())
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

	if a.context.replace {
		args = append(args, "--replace")
	}

	//  are you adding new context values? if you only use them for this func, then use args instead

	a.runCli(args...)

	return a
}

func (a *Actions) CreateFromFile(handler func(app *v1alpha1.Application), flags ...string) *Actions {
	a.context.t.Helper()
	app := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      a.context.AppName(),
			Namespace: a.context.AppNamespace(),
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: a.context.project,
			Source: v1alpha1.ApplicationSource{
				RepoURL: fixture.RepoURL(a.context.repoURLType),
				Path:    a.context.path,
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    a.context.destServer,
				Namespace: fixture.DeploymentNamespace(),
			},
		},
	}
	if a.context.namePrefix != "" || a.context.nameSuffix != "" {
		app.Spec.Source.Kustomize = &v1alpha1.ApplicationSourceKustomize{
			NamePrefix: a.context.namePrefix,
			NameSuffix: a.context.nameSuffix,
		}
	}
	if a.context.configManagementPlugin != "" {
		app.Spec.Source.Plugin = &v1alpha1.ApplicationSourcePlugin{
			Name: a.context.configManagementPlugin,
		}
	}

	if len(a.context.parameters) > 0 {
		log.Fatal("Application parameters or json tlas are not supported")
	}

	if a.context.directoryRecurse {
		app.Spec.Source.Directory = &v1alpha1.ApplicationSourceDirectory{Recurse: true}
	}

	handler(app)
	data := grpc.MustMarshal(app)
	tmpFile, err := os.CreateTemp("", "")
	errors.CheckError(err)
	_, err = tmpFile.Write(data)
	errors.CheckError(err)

	args := append([]string{
		"app", "create",
		"-f", tmpFile.Name(),
	}, flags...)
	defer tmpFile.Close()
	a.runCli(args...)
	return a
}
