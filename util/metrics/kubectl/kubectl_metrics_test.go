package kubectl

import (
	"testing"
)

func Test_RegisterWithClientGo_race(_ *testing.T) {
	// This test ensures that the RegisterWithClientGo function can be called concurrently without causing a data race.
	go RegisterWithClientGo()
	go RegisterWithClientGo()
}
