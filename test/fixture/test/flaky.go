package test

import (
	"os"
	"testing"
)

// invoke this method to indicate it is a flaky test that should be skipped on CI
func Flaky(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skipf("flaky test %s skipped when envvar CI=true", t.Name())
	}
}