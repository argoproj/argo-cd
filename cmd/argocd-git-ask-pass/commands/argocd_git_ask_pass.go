package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/argoproj/argo-cd/v2/reposerver/askpass"
	"github.com/argoproj/argo-cd/v2/util/errors"
	grpc_util "github.com/argoproj/argo-cd/v2/util/grpc"
	"github.com/argoproj/argo-cd/v2/util/io"
)

const (
	// cliName is the name of the CLI
	cliName = "argocd-git-ask-pass"
)

func NewCommand() *cobra.Command {
	command := cobra.Command{
		Use:               cliName,
		Short:             "Argo CD git credential helper",
		DisableAutoGenTag: true,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(os.Args) != 2 {
				errors.CheckError(fmt.Errorf("expected 1 argument, got %d", len(os.Args)-1))
			}
			nonce := os.Getenv(askpass.ASKPASS_NONCE_ENV)
			if nonce == "" {
				errors.CheckError(fmt.Errorf("%s is not set", askpass.ASKPASS_NONCE_ENV))
			}
			conn, err := grpc_util.BlockingDial(ctx, "unix", askpass.SocketPath, nil, grpc.WithTransportCredentials(insecure.NewCredentials()))
			errors.CheckError(err)
			defer io.Close(conn)
			client := askpass.NewAskPassServiceClient(conn)

			creds, err := client.GetCredentials(ctx, &askpass.CredentialsRequest{Nonce: nonce})
			errors.CheckError(err)
			switch {
			case strings.HasPrefix(os.Args[1], "Username"):
				fmt.Println(creds.Username)
			case strings.HasPrefix(os.Args[1], "Password"):
				fmt.Println(creds.Password)
			default:
				errors.CheckError(fmt.Errorf("unknown credential type '%s'", os.Args[1]))
			}
		},
	}

	return &command
}
