package imageUpdater

import (
	"fmt"
	"strings"
	"time"

	"github.com/argoproj/argo-cd/v2/image-updater/log"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

type state = string

const (
	failed    = "failed"
	pending   = "pending"
	succeeded = "succeeded"
)

func (c *Consequences) Expect(e Expectation) *Consequences {
	return c.ExpectWithDuration(e, time.Duration(30)*time.Second)
}

func SyncStatusIs(expected v1alpha1.SyncStatusCode) Expectation {
	return func(c *Consequences) (state, string) {
		actual := c.app().Status.Sync.Status
		return simple(actual == expected, fmt.Sprintf("sync status to be %s, is %s", expected, actual))
	}
}

func simple(success bool, message string) (state, string) {
	if success {
		return succeeded, message
	} else {
		return pending, message
	}
}

func (c *Consequences) ExpectWithDuration(e Expectation, timeout time.Duration) *Consequences {

	// this invocation makes sure this func is not reported as the cause of the failure - we are a "test helper"
	c.context.t.Helper()
	var message string
	var state state
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(3 * time.Second) {
		state, message = e(c)
		switch state {
		case succeeded:
			log.Infof("expectation succeeded: %s", message)
			return c
		case failed:
			c.context.t.Fatalf("failed expectation: %s", message)
			return c
		}
		log.Infof("expectation pending: %s", message)
	}
	c.context.t.Fatal("timeout waiting for: " + message)
	return c
}

// Expectation returns succeeded on succes condition, or pending/failed on failure, along with
// a message to describe the success/failure condition.
type Expectation func(c *Consequences) (state state, message string)

// Success asserts that the last command was successful
func Success(message string) Expectation {
	return func(c *Consequences) (state, string) {
		if c.actions.lastError != nil {
			return failed, fmt.Sprintf("error: %v", c.actions.lastError)
		}
		if !strings.Contains(c.actions.lastOutput, message) {
			return failed, fmt.Sprintf("output did not contain '%s'", message)
		}
		return succeeded, fmt.Sprintf("no error and output contained '%s'", message)
	}
}

// Error asserts that the last command was an error with substring match
func Error(message, err string) Expectation {
	return func(c *Consequences) (state, string) {
		if c.actions.lastError == nil {
			return failed, "no error"
		}
		if !strings.Contains(c.actions.lastOutput, message) {
			return failed, fmt.Sprintf("output does not contain '%s'", message)
		}
		if !strings.Contains(c.actions.lastError.Error(), err) {
			return failed, fmt.Sprintf("error does not contain '%s'", message)
		}
		return succeeded, fmt.Sprintf("error '%s'", message)
	}
}

func ApplicationImageUpdated(desiredImages v1alpha1.KustomizeImages) Expectation {

	return func(c *Consequences) (state, string) {

		foundApp, _ := c.actions.get()

		if foundApp == nil {
			return pending, fmt.Sprintf("missing app")
		}

		for i, image := range foundApp.Spec.Source.Kustomize.Images {
			if desiredImages[i] != image {
				return failed, "retrieved images did not match desired images "
			}
		}

		return succeeded, "app image successfully updated"
	}
}

func ApplicationGitRepoUpdated() {}
