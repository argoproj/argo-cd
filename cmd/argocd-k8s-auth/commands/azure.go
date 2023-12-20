package commands

import (
	"fmt"
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
	var command = &cobra.Command{
		Use: "azure",
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			o := token.OptionsWithEnv()
			// we'll use default of WorkloadIdentityLogin for the login flow
			if o.LoginMethod == "" {
				o.LoginMethod = token.WorkloadIdentityLogin
			}
			if o.ServerID == "" {
				o.ServerID = DEFAULT_AAD_SERVER_APPLICATION_ID
			}

			if v, ok := os.LookupEnv(envServerApplicationID); ok {
				o.ServerID = v
			}
			if v, ok := os.LookupEnv(envEnvironmentName); ok {
				o.Environment = v
			}
			tokenProvider, err := token.GetTokenProvider(o)
			errors.CheckError(err)

			accessToken, err := tokenProvider.GetAccessToken(ctx)
			errors.CheckError(err)

			_, _ = fmt.Fprint(os.Stdout, formatJSON(accessToken.Token, accessToken.ExpiresOn))
		},
	}
	return command
}
