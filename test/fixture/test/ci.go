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

// invoke this method to indicate test should only run on CI, i.e. edge-case test on code that rarely changes and needs
// extra software install
func CIOnly(t *testing.T) {
	if os.Getenv("CI") != "true" {
		t.Skipf("test %s skipped when envvar CI!=true", t.Name())
	}
}
