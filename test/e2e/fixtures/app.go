package fixtures

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/common"
	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

type App struct {
	fixture *Fixture
	t       *testing.T
	path    string
	Name    string
}

func (f *Fixture) NewApp(t *testing.T, path string) *App {

	f.EnsureCleanState()

	name := strings.ReplaceAll(path, "/", "-")

	_, _ = f.RunCli("app", "create", name,
		"--repo", f.RepoURL(),
		"--path", path,
		"--dest-server", common.KubernetesInternalAPIServerAddr,
		"--dest-namespace", f.DeploymentNamespace)

	return &App{f, t, path, name}
}

func (a *App) Sync() *App {
	_, _ = a.fixture.RunCli("app", "sync", a.Name, "--timeout", "5")
	return a
}

func (a *App) TerminateOp() *App {
	_, _ = a.fixture.RunCli("app", "terminate-op", a.Name)
	return a
}

func (a *App) Expect(e Expectation) *App {
	WaitUntil(a.t, func() (done bool, err error) {
		done, message := e(a)
		if done {
			return true, nil
		} else {
			return false, errors.New(message)
		}
	})
	return a
}

func (a *App) Patch(file string, jsonPath string) *App {
	a.fixture.Patch(a.path+"/"+file, jsonPath)
	return a
}

func (a *App) get() Application {
	app, err := a.fixture.AppClientset.ArgoprojV1alpha1().Applications(a.fixture.ArgoCDNamespace).Get(a.Name, v1.GetOptions{})
	assert.NoError(a.t, err)
	return *app
}

func (a *App) resource(name string) ResourceStatus {
	for _, r := range a.get().Status.Resources {
		if r.Name == name {
			return r
		}
	}
	return ResourceStatus{
		Health: &HealthStatus{Status: HealthStatusUnknown},
	}
}
