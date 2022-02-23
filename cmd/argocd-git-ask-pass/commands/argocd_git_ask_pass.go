package commands

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/argoproj/argo-cd/v2/util/git"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"

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
	var command = cobra.Command{
		Use:               cliName,
		Short:             "Argo CD git credential helper",
		DisableAutoGenTag: true,
		Run: func(c *cobra.Command, args []string) {
			if len(os.Args) != 2 {
				errors.CheckError(fmt.Errorf("expected 1 argument, got %d", len(os.Args)-1))
			}
			nonce := os.Getenv(git.ASKPASS_NONCE_ENV)
			if nonce == "" {
				errors.CheckError(fmt.Errorf("%s is not set", git.ASKPASS_NONCE_ENV))
			}
			conn, err := grpc_util.BlockingDial(context.Background(), "unix", askpass.SocketPath, nil, grpc.WithInsecure())
			errors.CheckError(err)
			defer io.Close(conn)
			client := askpass.NewAskPassServiceClient(conn)

			creds, err := client.GetCredentials(context.Background(), &askpass.CredentialsRequest{Nonce: nonce})
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
