package revision_metadata

import (
	"fmt"
	"strings"

	argoexec "github.com/argoproj/pkg/exec"

	"github.com/argoproj/gitops-engine/pkg/utils/errors"
)

var Author string

func init() {
	userName, err := argoexec.RunCommand("git", argoexec.CmdOpts{}, "config", "--get", "user.name")
	errors.CheckErrorWithCode(err, errors.ErrorCommandSpecific)
	userEmail, err := argoexec.RunCommand("git", argoexec.CmdOpts{}, "config", "--get", "user.email")
	errors.CheckErrorWithCode(err, errors.ErrorCommandSpecific)
	Author = fmt.Sprintf("%s <%s>", strings.TrimSpace(userName), strings.TrimSpace(userEmail))
}
