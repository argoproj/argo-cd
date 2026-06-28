package revision_metadata

import (
	"fmt"
	"strings"

	"github.com/argoproj/argo-cd/v3/util/errors"
	argoexec "github.com/argoproj/argo-cd/v3/util/exec"
)

var Author string

func init() {
	userName, err := argoexec.RunCommand("git", argoexec.CmdOpts{}, "config", "--get", "user.name")
	errors.CheckError(err)
	userEmail, err := argoexec.RunCommand("git", argoexec.CmdOpts{}, "config", "--get", "user.email")
	errors.CheckError(err)
	Author = fmt.Sprintf("%s <%s>", strings.TrimSpace(userName), strings.TrimSpace(userEmail))
}
