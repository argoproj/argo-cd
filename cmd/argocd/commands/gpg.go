package commands

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/headless"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	gpgkeypkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/gpgkey"
	appsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/errors"
	argoio "github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/templates"
)

// NewGPGCommand returns a new instance of an `argocd repo` command
func NewGPGCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "gpg",
		Short: "Manage GPG keys used for signature verification",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
		Example: ``,
	}
	command.AddCommand(NewGPGListCommand(clientOpts))
	command.AddCommand(NewGPGGetCommand(clientOpts))
	command.AddCommand(NewGPGAddCommand(clientOpts))
	command.AddCommand(NewGPGDeleteCommand(clientOpts))
	return command
}

// NewGPGListCommand lists all configured public keys from the server
func NewGPGListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var output string
	command := &cobra.Command{
		Use:   "list",
		Short: "List configured GPG public keys",
		Example: templates.Examples(`
  # List all configured GPG public keys in wide format (default).
  argocd gpg list
		
  # List all configured GPG public keys in JSON format.
  argocd gpg list -o json
		
  # List all configured GPG public keys in YAML format.
  argocd gpg list -o yaml
  		`),

		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			conn, gpgIf := headless.NewClientOrDie(clientOpts, c).NewGPGKeyClientOrDie()
			defer argoio.Close(conn)
			keys, err := gpgIf.List(ctx, &gpgkeypkg.GnuPGPublicKeyQuery{})
			errors.CheckError(err)
			switch output {
			case "yaml", "json":
				err := PrintResourceList(keys.Items, output, false)
				errors.CheckError(err)
			case "wide", "":
				printKeyTable(keys.Items)
			default:
				errors.CheckError(fmt.Errorf("unknown output format: %s", output))
			}
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: json|yaml|wide")
	return command
}

// NewGPGGetCommand retrieves a single public key from the server
func NewGPGGetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var output string
	command := &cobra.Command{
		Use:   "get KEYID",
		Short: "Get the GPG public key with ID <KEYID> from the server",
		Example: templates.Examples(`  
  # Get a GPG public key with the specified KEYID in wide format (default).
  argocd gpg get KEYID
		
  # Get a GPG public key with the specified KEYID in JSON format.
  argocd gpg get KEYID -o json
		
  # Get a GPG public key with the specified KEYID in YAML format.
  argocd gpg get KEYID -o yaml
  		`),

		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 1 {
				errors.CheckError(fmt.Errorf("Missing KEYID argument"))
			}
			conn, gpgIf := headless.NewClientOrDie(clientOpts, c).NewGPGKeyClientOrDie()
			defer argoio.Close(conn)
			key, err := gpgIf.Get(ctx, &gpgkeypkg.GnuPGPublicKeyQuery{KeyID: args[0]})
			errors.CheckError(err)
			switch output {
			case "yaml", "json":
				err := PrintResourceList(key, output, false)
				errors.CheckError(err)
			case "wide", "":
				fmt.Printf("Key ID:          %s\n", key.KeyID)
				fmt.Printf("Key fingerprint: %s\n", key.Fingerprint)
				fmt.Printf("Key subtype:     %s\n", strings.ToUpper(key.SubType))
				fmt.Printf("Key owner:       %s\n", key.Owner)
				fmt.Printf("Key data follows until EOF:\n%s\n", key.KeyData)
			default:
				errors.CheckError(fmt.Errorf("unknown output format: %s", output))
			}
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: json|yaml|wide")
	return command
}

// NewGPGAddCommand adds a public key to the server's configuration
func NewGPGAddCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var fromFile string
	command := &cobra.Command{
		Use:   "add",
		Short: "Adds a GPG public key to the server's keyring",
		Example: templates.Examples(`
  # Add a GPG public key to the server's keyring from a file.
  argocd gpg add --from /path/to/keyfile
  		`),

		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if fromFile == "" {
				errors.CheckError(fmt.Errorf("--from is mandatory"))
			}
			keyData, err := os.ReadFile(fromFile)
			if err != nil {
				errors.CheckError(err)
			}
			conn, gpgIf := headless.NewClientOrDie(clientOpts, c).NewGPGKeyClientOrDie()
			defer argoio.Close(conn)
			resp, err := gpgIf.Create(ctx, &gpgkeypkg.GnuPGPublicKeyCreateRequest{Publickey: &appsv1.GnuPGPublicKey{KeyData: string(keyData)}})
			errors.CheckError(err)
			fmt.Printf("Created %d key(s) from input file", len(resp.Created.Items))
			if len(resp.Skipped) > 0 {
				fmt.Printf(", and %d key(s) were skipped because they exist already", len(resp.Skipped))
			}
			fmt.Printf(".\n")
		},
	}
	command.Flags().StringVarP(&fromFile, "from", "f", "", "Path to the file that contains the GPG public key to import")
	return command
}

// NewGPGDeleteCommand removes a key from the server's keyring
func NewGPGDeleteCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "rm KEYID",
		Short: "Removes a GPG public key from the server's keyring",
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 1 {
				errors.CheckError(fmt.Errorf("Missing KEYID argument"))
			}
			conn, gpgIf := headless.NewClientOrDie(clientOpts, c).NewGPGKeyClientOrDie()
			defer argoio.Close(conn)
			_, err := gpgIf.Delete(ctx, &gpgkeypkg.GnuPGPublicKeyQuery{KeyID: args[0]})
			errors.CheckError(err)
			fmt.Printf("Deleted key with key ID %s\n", args[0])
		},
	}
	return command
}

// Print table of certificate info
func printKeyTable(keys []appsv1.GnuPGPublicKey) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "KEYID\tTYPE\tIDENTITY\n")

	for _, k := range keys {
		fmt.Fprintf(w, "%s\t%s\t%s\n", k.KeyID, strings.ToUpper(k.SubType), k.Owner)
	}
	_ = w.Flush()
}
