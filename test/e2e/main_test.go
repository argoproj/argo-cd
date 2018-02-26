package e2e

import (
	"fmt"
	"os"
	"testing"
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
		defer fixture.TearDown()
		m.Run()
	}
}
