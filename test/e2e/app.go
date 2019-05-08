package e2e

import (
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/common"
	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

type App struct {
	fixture *Fixture
	t       *testing.T
	Name    string
}

func (f *Fixture) NewApp(t *testing.T, path string) (app App) {

	f.EnsureCleanState()

	_, err := f.RunCli("app", "create", path,
		"--repo", f.RepoURL(),
		"--path", path,
		"--dest-server", common.KubernetesInternalAPIServerAddr,
		"--dest-namespace", f.DeploymentNamespace,
		"--sync-policy", "automated")

	assert.NoError(t, err)

	return App{f, t, path}
}

func (f *App) get() (*Application, error) {
	app, err := f.fixture.AppClientset.ArgoprojV1alpha1().Applications(f.fixture.ArgoCDNamespace).Get(f.Name, v1.GetOptions{})
	assert.NoError(f.t, err)
	return app, err
}

func (f *App) Is(value func(app *Application) bool, msg string, args ...interface{}) (*testing.T, wait.ConditionFunc, string) {
	return f.t, func() (done bool, err error) {
		app, err := f.get()
		return err == nil && value(app), err
	}, fmt.Sprintf(msg, args...)
}

func (f *App) SyncStatusIs(value SyncStatusCode) (*testing.T, wait.ConditionFunc, string) {
	return f.Is(func(app *Application) bool {
		return app.Status.Sync.Status == value
	}, fmt.Sprintf("app %s's sync status is %s", f.Name, value))
}

func (f *App) HealthIs(value HealthStatusCode) (*testing.T, wait.ConditionFunc, string) {
	return f.Is(func(app *Application) bool {
		return app.Status.Health.Status == value
	}, "app %s's health is %s", f.Name, value)
}

func (f *App) ResourceIs(name string, value func(resource ResourceStatus) bool, msg string, args ...interface{}) (*testing.T, wait.ConditionFunc, string) {
	return f.Is(func(app *Application) bool {
		for _, resource := range app.Status.Resources {
			if resource.Name == name {
				return value(resource)
			}
		}
		return false
	}, fmt.Sprintf(msg, args...))
}
func (f *App) ResourceSyncStatusIs(name string, value SyncStatusCode) (*testing.T, wait.ConditionFunc, string) {
	return f.ResourceIs(name, func(resource ResourceStatus) bool {
		return resource.Status == value
	}, "app %s's resource %s sync status is %s", f.Name, name, value)
}

func (f *App) ResourceHealthIs(name string, value HealthStatusCode) (*testing.T, wait.ConditionFunc, string) {
	return f.ResourceIs(name, func(resource ResourceStatus) bool {
		return resource.Health.Status == value
	}, "app %s's resource %s health is %s", f.Name, name, value)
}
