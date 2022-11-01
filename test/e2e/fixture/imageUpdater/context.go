package imageUpdater

import (
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/env"
)

// this implements the "given" part of given/when/then
type Context struct {
	t           *testing.T
	path        string
	chart       string
	repoURLType fixture.RepoURLType
	// seconds
	timeout                int
	name                   string
	appNamespace           string
	destServer             string
	destName               string
	env                    string
	parameters             []string
	namePrefix             string
	nameSuffix             string
	resource               string
	prune                  bool
	configManagementPlugin string
	async                  bool
	localPath              string
	project                string
	revision               string
	force                  bool
	directoryRecurse       bool
	replace                bool
	helmPassCredentials    bool
	helmSkipCrds           bool
	trackingMethod         v1alpha1.TrackingMethod
}

func Given(t *testing.T) *Context {
	fixture.EnsureCleanState(t)
	return GivenWithSameState(t)
}

func GivenWithSameState(t *testing.T) *Context {
	// ARGOCE_E2E_DEFAULT_TIMEOUT can be used to override the default timeout
	// for any context.
	timeout := env.ParseNumFromEnv("ARGOCD_E2E_DEFAULT_TIMEOUT", 10, 0, 180)
	return &Context{
		t:              t,
		destServer:     v1alpha1.KubernetesInternalAPIServerAddr,
		repoURLType:    fixture.RepoURLTypeFile,
		name:           fixture.Name(),
		timeout:        timeout,
		project:        "default",
		prune:          true,
		trackingMethod: argo.TrackingMethodLabel,
	}
}

func (c *Context) NamePrefix(namePrefix string) *Context {
	c.namePrefix = namePrefix
	return c
}

func (c *Context) NameSuffix(nameSuffix string) *Context {
	c.nameSuffix = nameSuffix
	return c
}

func (c *Context) Path(path string) *Context {
	c.path = path
	return c
}

func (c *Context) And(block func()) *Context {
	block()
	return c
}

func (c *Context) Sleep(seconds time.Duration) *Context {
	time.Sleep(seconds * time.Second)
	return c
}

func (c *Context) When() *Actions {
	time.Sleep(1 * time.Second)
	return &Actions{context: c}
}

func (c *Context) SetAppNamespace(namespace string) *Context {
	c.appNamespace = namespace
	return c
}

func (c *Context) AppNamespace() string {
	if c.appNamespace != "" {
		return c.appNamespace
	} else {
		return fixture.TestNamespace()
	}
}

func (c *Context) AppQualifiedName() string {
	if c.appNamespace != "" {
		return c.appNamespace + "/" + c.AppName()
	} else {
		return c.AppName()
	}
}

func (c *Context) AppName() string {
	return c.name
}
