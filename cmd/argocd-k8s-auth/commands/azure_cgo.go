//go:build !darwin || (cgo && darwin)

package commands

import (
	"fmt"
	"os"
	"strconv"

	"github.com/Azure/kubelogin/pkg/token"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v3/util/errors"
)

const (
	DEFAULT_AAD_SERVER_APPLICATION_ID = "6dae42f8-4368-4678-94ff-3960e28e3630"
	envServerApplicationID            = "AAD_SERVER_APPLICATION_ID"
	envEnvironmentName                = "AAD_ENVIRONMENT_NAME"
	envIsPoPTokenEnabled              = "AAD_IS_POP_TOKEN_ENABLED"
	envPoPTokenClaims                 = "AAD_POP_TOKEN_CLAIMS"
)

// buildAzureTokenOptions constructs a token.Options from environment variables.
// It sets a default login method of WorkloadIdentityLogin when none is provided,
// applies the AAD server application ID and environment overrides, and configures
// POP token support for ServicePrincipalLogin.
func buildAzureTokenOptions() (*token.Options, error) {
	o := token.OptionsWithEnv()
	if o.LoginMethod == "" {
		o.LoginMethod = token.WorkloadIdentityLogin
	}
	o.ServerID = DEFAULT_AAD_SERVER_APPLICATION_ID
	if v, ok := os.LookupEnv(envServerApplicationID); ok {
		o.ServerID = v
	}
	if v, ok := os.LookupEnv(envEnvironmentName); ok {
		o.Environment = v
	}
	if o.LoginMethod == token.ServicePrincipalLogin {
		if v, ok := os.LookupEnv(envIsPoPTokenEnabled); ok {
			if enabled, err := strconv.ParseBool(v); err == nil && enabled {
				popClaims, ok := os.LookupEnv(envPoPTokenClaims)
				if !ok || popClaims == "" {
					return nil, fmt.Errorf("env %s is enabled but %s is not set", envIsPoPTokenEnabled, envPoPTokenClaims)
				}
				o.IsPoPTokenEnabled = true
				o.PoPTokenClaims = popClaims
			}
		}
	}
	return o, nil
}

func newAzureCommand() *cobra.Command {
	command := &cobra.Command{
		Use: "azure",
		Run: func(c *cobra.Command, _ []string) {
			o, err := buildAzureTokenOptions()
			errors.CheckError(err)
			verboseLog("argocd-k8s-auth azure: login-method=%q server-id=%q environment=%q", o.LoginMethod, o.ServerID, o.Environment)
			tp, err := token.GetTokenProvider(o)
			errors.CheckError(err)
			tok, err := tp.GetAccessToken(c.Context())
			errors.CheckError(err)
			_, _ = fmt.Fprint(os.Stdout, formatJSON(tok.Token, tok.ExpiresOn))
		},
	}
	return command
}
