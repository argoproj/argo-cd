//go:build !darwin || (cgo && darwin)

package commands

import (
	"fmt"
	"os"

	"github.com/Azure/kubelogin/pkg/token"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v3/util/errors"
)

var (
	envServerApplicationID = "AAD_SERVER_APPLICATION_ID"
	envEnvironmentName     = "AAD_ENVIRONMENT_NAME"
)

const (
	DEFAULT_AAD_SERVER_APPLICATION_ID = "6dae42f8-4368-4678-94ff-3960e28e3630"
)

func newAzureCommand() *cobra.Command {
	command := &cobra.Command{
		Use: "azure",
		Run: func(c *cobra.Command, _ []string) {
			o := token.OptionsWithEnv()
			if o.LoginMethod == "" { // no environment variable overrides
				// we'll use default of WorkloadIdentityLogin for the login flow
				o.LoginMethod = token.WorkloadIdentityLogin
			}
			o.ServerID = DEFAULT_AAD_SERVER_APPLICATION_ID
			if v, ok := os.LookupEnv(envServerApplicationID); ok {
				o.ServerID = v
			}
			if v, ok := os.LookupEnv(envEnvironmentName); ok {
				o.Environment = v
			}
			tp, err := token.GetTokenProvider(o)
			errors.CheckError(err)
			tok, err := tp.GetAccessToken(c.Context())
			errors.CheckError(err)
			_, _ = fmt.Fprint(os.Stdout, formatJSON(tok.Token, tok.ExpiresOn))
		},
	}
	return command
}
