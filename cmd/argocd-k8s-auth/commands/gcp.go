package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/oauth2/google"

	"github.com/argoproj/argo-cd/v2/util/errors"
)

// defaultGCPScopes:
//   - cloud-platform is the base scope to authenticate to GCP.
//   - userinfo.email is used to authenticate to GKE APIs with gserviceaccount
//     email instead of numeric uniqueID.
//
// https://github.com/kubernetes/client-go/blob/be758edd136e61a1bffadf1c0235fceb8aee8e9e/plugin/pkg/client/auth/gcp/gcp.go#L59
var defaultGCPScopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
	"https://www.googleapis.com/auth/userinfo.email",
}

func newGCPCommand() *cobra.Command {
	command := &cobra.Command{
		Use: "gcp",
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			// Preferred way to retrieve GCP credentials
			// https://github.com/golang/oauth2/blob/9780585627b5122c8cc9c6a378ac9861507e7551/google/doc.go#L54-L68
			cred, err := google.FindDefaultCredentials(ctx, defaultGCPScopes...)
			errors.CheckError(err)
			token, err := cred.TokenSource.Token()
			errors.CheckError(err)
			_, _ = fmt.Fprint(os.Stdout, formatJSON(token.AccessToken, token.Expiry))
		},
	}
	return command
}
