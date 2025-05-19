package app

import (
	"context"
	"time"

	"github.com/argoproj/gitops-engine/pkg/health"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	applicationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	util "github.com/argoproj/argo-cd/v3/util/io"
)

// this implements the "then" part of given/when/then
type Consequences struct {
	context *Context
	actions *Actions
	timeout int
}

func (c *Consequences) Expect(e Expectation) *Consequences {
	// this invocation makes sure this func is not reported as the cause of the failure - we are a "test helper"
	c.context.t.Helper()
	var message string
	var state state
	sleepIntervals := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		50 * time.Millisecond,
		100 * time.Millisecond,
		200 * time.Millisecond,
		300 * time.Millisecond,
		500 * time.Millisecond,
		1 * time.Second,
	}
	sleepIntervalsIdx := -1
	timeout := time.Duration(c.timeout) * time.Second
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(sleepIntervals[sleepIntervalsIdx]) {
		if sleepIntervalsIdx < len(sleepIntervals)-1 {
			sleepIntervalsIdx++
		}
		state, message = e(c)
		switch state {
		case succeeded:
			log.Infof("expectation succeeded: %s", message)
			return c
		case failed:
			c.context.t.Fatalf("failed expectation: %s", message)
			return c
		}
		log.Infof("pending: %s", message)
	}
	c.context.t.Fatal("timeout waiting for: " + message)
	return c
}

// ExpectConsistently will continuously evaluate a condition, and it must be true each time it is evaluated, otherwise the test is failed. The condition will be repeatedly evaluated until 'expirationDuration' is met, waiting 'waitDuration' after each success.
func (c *Consequences) ExpectConsistently(e Expectation, waitDuration time.Duration, expirationDuration time.Duration) *Consequences {
	// this invocation makes sure this func is not reported as the cause of the failure - we are a "test helper"
	c.context.t.Helper()

	expiration := time.Now().Add(expirationDuration)
	for time.Now().Before(expiration) {
		state, message := e(c)
		switch state {
		case succeeded:
			log.Infof("expectation succeeded: %s", message)
		case failed:
			c.context.t.Fatalf("failed expectation: %s", message)
			return c
		}

		// On condition success: wait, then retry
		log.Infof("Expectation '%s' passes, repeating to ensure consistency", message)
		time.Sleep(waitDuration)
	}

	// If the condition never failed before expiring, it is a pass.
	return c
}

func (c *Consequences) And(block func(app *v1alpha1.Application)) *Consequences {
	c.context.t.Helper()
	block(c.app())
	return c
}

func (c *Consequences) AndAction(block func()) *Consequences {
	c.context.t.Helper()
	block()
	return c
}

func (c *Consequences) Given() *Context {
	return c.context
}

func (c *Consequences) When() *Actions {
	time.Sleep(fixture.WhenThenSleepInterval)
	return c.actions
}

func (c *Consequences) app() *v1alpha1.Application {
	c.context.t.Helper()
	app, err := c.get()
	require.NoError(c.context.t, err)
	return app
}

func (c *Consequences) get() (*v1alpha1.Application, error) {
	return fixture.AppClientset.ArgoprojV1alpha1().Applications(c.context.AppNamespace()).Get(context.Background(), c.context.AppName(), metav1.GetOptions{})
}

func (c *Consequences) resource(kind, name, namespace string) v1alpha1.ResourceStatus {
	c.context.t.Helper()
	closer, client, err := fixture.ArgoCDClientset.NewApplicationClient()
	require.NoError(c.context.t, err)
	defer util.Close(closer)
	app, err := client.Get(context.Background(), &applicationpkg.ApplicationQuery{
		Name:         ptr.To(c.context.AppName()),
		Projects:     []string{c.context.project},
		AppNamespace: ptr.To(c.context.appNamespace),
	})
	require.NoError(c.context.t, err)
	for _, r := range app.Status.Resources {
		if r.Kind == kind && r.Name == name && (namespace == "" || namespace == r.Namespace) {
			return r
		}
	}
	return v1alpha1.ResourceStatus{
		Health: &v1alpha1.HealthStatus{
			Status:  health.HealthStatusMissing,
			Message: "not found",
		},
	}
}

func (c *Consequences) AndCLIOutput(block func(output string, err error)) *Consequences {
	c.context.t.Helper()
	block(c.actions.lastOutput, c.actions.lastError)
	return c
}
