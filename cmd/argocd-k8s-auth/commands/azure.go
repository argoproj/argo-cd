package commands

import (
	"os"

	"github.com/Azure/kubelogin/pkg/token"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v2/util/errors"
)

var (
	envServerApplicationID = "AAD_SERVER_APPLICATION_ID"
	envEnvironmentName     = "AAD_ENVIRONMENT_NAME"
)

const (
	DEFAULT_AAD_SERVER_APPLICATION_ID = "6dae42f8-4368-4678-94ff-3960e28e3630"
)

func newAzureCommand() *cobra.Command {
	o := token.NewOptions()
	// we'll use default of WorkloadIdentityLogin for the login flow
	o.LoginMethod = token.WorkloadIdentityLogin
	o.ServerID = DEFAULT_AAD_SERVER_APPLICATION_ID
	command := &cobra.Command{
		Use: "azure",
		Run: func(c *cobra.Command, args []string) {
			o.UpdateFromEnv()
			if v, ok := os.LookupEnv(envServerApplicationID); ok {
				o.ServerID = v
			}
			if v, ok := os.LookupEnv(envEnvironmentName); ok {
				o.Environment = v
			}
			plugin, err := token.New(&o)
			errors.CheckError(err)
			err = plugin.Do()
			errors.CheckError(err)
		},
	}
	return command
}
