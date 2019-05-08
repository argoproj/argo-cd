package e2e

import (
	"fmt"
	"os"
	"testing"

	fixture2 "github.com/argoproj/argo-cd/test/e2e/fixtures"
)

var (
	fixture *fixture2.Fixture
)

func TestMain(m *testing.M) {
	var err error
	fixture, err = fixture2.NewFixture()
	if err != nil {
		println(fmt.Sprintf("Unable to create e2e fixture: %v", err))
		os.Exit(-1)
	} else {
		code := m.Run()
		fixture.Cleanup()
		os.Exit(code)
	}
}
