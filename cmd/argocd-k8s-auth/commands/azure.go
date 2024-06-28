package commands

import (
	"context"
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
	// we'll use default of WorkloadIdentityLogin for the login flow
	o := &token.Options{
		LoginMethod: token.WorkloadIdentityLogin,
		ServerID:    DEFAULT_AAD_SERVER_APPLICATION_ID,
	}

	command := &cobra.Command{
		Use: "azure",
		Run: func(c *cobra.Command, args []string) {
			if v, ok := os.LookupEnv(envServerApplicationID); ok {
				o.ServerID = v
			}
			if v, ok := os.LookupEnv(envEnvironmentName); ok {
				o.Environment = v
			}
			plugin, err := token.GetTokenProvider(o)
			errors.CheckError(err)
			_, err = plugin.GetAccessToken(context.Background())
			errors.CheckError(err)
		},
	}
	return command
}
