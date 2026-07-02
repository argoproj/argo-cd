package commands

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	jwtgo "github.com/golang-jwt/jwt/v5"
	"github.com/spf13/cobra"

	argoerrors "github.com/argoproj/argo-cd/v3/util/errors"
	jwtutil "github.com/argoproj/argo-cd/v3/util/jwt"
)

// newOIDCCommand returns a new instance of an oidc command that emits a pre-issued OIDC token (e.g. a projected ServiceAccount token) as a k8s auth token
func newOIDCCommand() *cobra.Command {
	var tokenFile string
	command := &cobra.Command{
		Use: "oidc",
		Run: func(_ *cobra.Command, _ []string) {
			tokenFile, err := validateTokenFile(tokenFile)
			argoerrors.CheckError(err)

			raw, err := os.ReadFile(tokenFile)
			argoerrors.CheckError(err)
			token := strings.TrimSpace(string(raw))
			if token == "" {
				argoerrors.CheckError(fmt.Errorf("token file %q is empty", tokenFile))
			}

			_, _ = fmt.Fprint(os.Stdout, formatJSON(token, tokenExpiration(token)))
		},
	}
	command.Flags().StringVar(&tokenFile, "token-file", "", "Path to a file containing the OIDC token (defaults to the ARGOCD_OIDC_TOKEN_FILE env var)")
	return command
}

// validateTokenFile resolves the token file path, falling back to the ARGOCD_OIDC_TOKEN_FILE env var, and returns an error if none is set
func validateTokenFile(tokenFile string) (string, error) {
	if tokenFile == "" {
		if envTokenFile, ok := os.LookupEnv("ARGOCD_OIDC_TOKEN_FILE"); ok {
			tokenFile = envTokenFile
		}
	}
	if tokenFile == "" {
		return "", errors.New("token file must be set via --token-file or ARGOCD_OIDC_TOKEN_FILE")
	}
	return tokenFile, nil
}

// tokenExpiration reads the unverified exp claim, falling back to a short TTL so the rotated token is re-read soon
func tokenExpiration(token string) time.Time {
	parsed, _, err := jwtgo.NewParser().ParseUnverified(token, jwtgo.MapClaims{})
	if err != nil {
		return time.Now().Add(time.Minute)
	}
	claims, err := jwtutil.MapClaims(parsed.Claims)
	if err != nil {
		return time.Now().Add(time.Minute)
	}
	exp, err := jwtutil.ExpirationTime(claims)
	if err != nil {
		return time.Now().Add(time.Minute)
	}
	return exp
}
