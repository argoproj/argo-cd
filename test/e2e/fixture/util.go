package fixture

import (
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/argoproj/argo-cd/v2/util/errors"
)

var (
	matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
	matchAllCap   = regexp.MustCompile("([a-z0-9])([A-Z])")
)

// returns dns friends string which is no longer than 63 characters and has specified postfix at the end
func DnsFriendly(str string, postfix string) string {
	str = matchFirstCap.ReplaceAllString(str, "${1}-${2}")
	str = matchAllCap.ReplaceAllString(str, "${1}-${2}")
	str = strings.ToLower(str)

	if diff := len(str) + len(postfix) - 63; diff > 0 {
		str = str[:len(str)-diff]
	}
	return str + postfix
}

func RunFunctionsInParallelAndCheckErrors(t *testing.T, functions map[string]func() error) {
	t.Helper()

	var wg sync.WaitGroup
	var mutex sync.Mutex
	results := map[string]error{}

	for name, function := range functions {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := function()
			mutex.Lock()
			defer mutex.Unlock()
			results[name] = err
		}()
	}
	wg.Wait()

	for _, err := range results {
		errors.CheckError(err)
	}
}
