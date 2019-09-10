package e2e

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/errors"
	. "github.com/argoproj/argo-cd/test/e2e/fixture"
)

func TestClusterList(t *testing.T) {
	output := FailOnErr(RunCli("cluster", "list")).(string)
	assert.Equal(t, fmt.Sprintf(`SERVER                          NAME  VERSION  STATUS      MESSAGE
https://kubernetes.default.svc        %v     Successful  `, GetVersions().ServerVersion), output)
}
