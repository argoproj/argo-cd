package revision_metadata

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/argoproj/argo-cd/errors"
)

var Author string

func init() {
	userName, err := exec.Command("git", "config", "--get", "user.name").Output()
	errors.CheckError(err)
	userEmail, err := exec.Command("git", "config", "--get", "user.email").Output()
	errors.CheckError(err)
	Author = fmt.Sprintf("%s <%s>", strings.TrimSpace(string(userName)), strings.TrimSpace(string(userEmail)))
}
