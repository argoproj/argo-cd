package test

import (
	"testing"
)

// invoke this method to indicate it is a flaky test that should be skipped on CI
func Flaky(t *testing.T) {
	LocalOnly(t)
}
