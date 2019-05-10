package e2e

import (
	"fmt"
	"os"
	"testing"

	. "github.com/argoproj/argo-cd/test/e2e/fixtures"
)

var (
	fixture *Fixture
)

func TestMain(m *testing.M) {
	var err error
	fixture, err = NewFixture()
	if err != nil {
		println(fmt.Sprintf("Unable to create e2e fixture: %v", err))
		os.Exit(-1)
	} else {
		code := m.Run()
		fixture.Teardown()
		os.Exit(code)
	}
}
