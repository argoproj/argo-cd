package commands

import (
	"fmt"
	"os"
	"time"

	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/golang-jwt/jwt/v4"
	"github.com/spf13/cobra"
)

// newTokenFileCommand returns a new instance of a command that uses local token from the file
// See https://kubernetes.io/docs/concepts/storage/projected-volumes/#serviceaccounttoken
func newTokenFileCommand() *cobra.Command {
	var (
		file string
	)
	var command = &cobra.Command{
		Use: "token-file",
		Run: func(c *cobra.Command, args []string) {
			token, err := getTokenFile(file)
			errors.CheckError(err)
			_, _ = fmt.Fprint(os.Stdout, token)
		},
	}
	command.Flags().StringVar(&file, "file", "/var/run/secrets/kubernetes.io/serviceaccount/token", "Path to a token file")
	return command
}

func getTokenFile(path string) (string, error) {
	tokenBuf, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	tokenStr := string(tokenBuf)

	token, _, err := new(jwt.Parser).ParseUnverified(tokenStr, jwt.MapClaims{})
	if err != nil {
		return "", err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("Error extracting claims from token %s", path)
	}

	exp, ok := claims["exp"].(float64)
	if !ok {
		return "", fmt.Errorf("Error reading expiration date from token %s", path)
	}

	tokenExpiration := time.Unix(int64(exp), 0)
	return formatJSON(tokenStr, tokenExpiration), nil
}
