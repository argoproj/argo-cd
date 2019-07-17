package test

import (
	"os"
	"testing"
)

// invoke this method to indicate  test that should be skipped on CI, i.e. you only need it for manual testing/locally
func LocalOnly(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skipf("test %s skipped when envvar CI=true", t.Name())
	}
}
