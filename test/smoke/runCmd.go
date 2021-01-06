package smoke

import (
	"os/exec"

	"github.com/argoproj/pkg/errors"
)

// RunCmd is a function to run generic shell commands
func RunCmd(name string, args ...string) ([]byte, error) {

	output, error := exec.Command(name, args...).Output()
	errors.CheckError(error)

	return output, error

}
