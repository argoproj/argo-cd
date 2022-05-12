package appset

import (
	"fmt"
	"os/exec"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	executil "github.com/argoproj/argo-cd/v2/util/exec"
)

// AppRBACName formats fully qualified application name for RBAC check
func AppSetRBACName(appSet *v1alpha1.ApplicationSet) string {
	return fmt.Sprintf("%s/%s", appSet.Spec.Template.Spec.GetProject(), appSet.ObjectMeta.Name)
}

func RunCommand(command v1alpha1.Command) (string, error) {
	if len(command.Command) == 0 {
		return "", fmt.Errorf("Command is empty")
	}
	cmd := exec.Command(command.Command[0], append(command.Command[1:], command.Args...)...)
	fmt.Print(cmd)
	return executil.Run(cmd)
}
