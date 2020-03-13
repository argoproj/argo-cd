package commands

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	gpgkeypkg "github.com/argoproj/argo-cd/pkg/apiclient/gpgkey"
	appsv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util"
)

// NewGPGCommand returns a new instance of an `argocd repo` command
func NewGPGCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
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
	return command
}

// NewGPGListCommand lists all configured public keys from the server
func NewGPGListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		output string
	)
	var command = &cobra.Command{
		Use:   "list",
		Short: "List configured GPG public keys",
		Run: func(c *cobra.Command, args []string) {
			conn, gpgIf := argocdclient.NewClientOrDie(clientOpts).NewGPGKeyClientOrDie()
			defer util.Close(conn)
			keys, err := gpgIf.ListGnuPGPublicKeys(context.Background(), &gpgkeypkg.GnuPGPublicKeyQuery{})
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
	var (
		output string
	)
	var command = &cobra.Command{
		Use:   "get KEYID",
		Short: "Get the GPG public key with ID <KEYID> from the server",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				errors.CheckError(fmt.Errorf("KEYID was not specified"))
			}
			conn, gpgIf := argocdclient.NewClientOrDie(clientOpts).NewGPGKeyClientOrDie()
			defer util.Close(conn)
			key, err := gpgIf.GetGnuPGPublicKey(context.Background(), &gpgkeypkg.GnuPGPublicKeyQuery{KeyID: args[0]})
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

func NewGPGAddCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		fromFile string
	)
	var command = &cobra.Command{
		Use:   "add",
		Short: "Adds a GPG public key to the server's keyring",
		Run: func(c *cobra.Command, args []string) {
			if fromFile == "" {
				errors.CheckError(fmt.Errorf("--from is mandatory"))
			}
			keyData, err := ioutil.ReadFile(fromFile)
			if err != nil {
				errors.CheckError(err)
			}
			conn, gpgIf := argocdclient.NewClientOrDie(clientOpts).NewGPGKeyClientOrDie()
			defer util.Close(conn)
			resp, err := gpgIf.CreateGnuPGPublicKey(context.Background(), &gpgkeypkg.GnuPGPublicKeyCreateRequest{Publickey: string(keyData)})
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

// func NewCertAddTLSCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
// 	var (
// 		fromFile string
// 		upsert   bool
// 	)
// 	var command = &cobra.Command{
// 		Use:   "add-tls SERVERNAME",
// 		Short: "Add TLS certificate data for connecting to repository server SERVERNAME",
// 		Run: func(c *cobra.Command, args []string) {
// 			conn, certIf := argocdclient.NewClientOrDie(clientOpts).NewCertClientOrDie()
// 			defer util.Close(conn)

// 			if len(args) != 1 {
// 				c.HelpFunc()(c, args)
// 				os.Exit(1)
// 			}

// 			var certificateArray []string
// 			var err error

// 			if fromFile != "" {
// 				fmt.Printf("Reading TLS certificate data in PEM format from '%s'\n", fromFile)
// 				certificateArray, err = certutil.ParseTLSCertificatesFromPath(fromFile)
// 			} else {
// 				fmt.Println("Enter TLS certificate data in PEM format. Press CTRL-D when finished.")
// 				certificateArray, err = certutil.ParseTLSCertificatesFromStream(os.Stdin)
// 			}

// 			errors.CheckError(err)

// 			certificateList := make([]appsv1.RepositoryCertificate, 0)

// 			subjectMap := make(map[string]*x509.Certificate)

// 			for _, entry := range certificateArray {
// 				// We want to make sure to only send valid certificate data to the
// 				// server, so we decode the certificate into X509 structure before
// 				// further processing it.
// 				x509cert, err := certutil.DecodePEMCertificateToX509(entry)
// 				errors.CheckError(err)

// 				// TODO: We need a better way to detect duplicates sent in the stream,
// 				// maybe by using fingerprints? For now, no two certs with the same
// 				// subject may be sent.
// 				if subjectMap[x509cert.Subject.String()] != nil {
// 					fmt.Printf("ERROR: Cert with subject '%s' already seen in the input stream.\n", x509cert.Subject.String())
// 					continue
// 				} else {
// 					subjectMap[x509cert.Subject.String()] = x509cert
// 				}
// 			}

// 			serverName := args[0]

// 			if len(certificateArray) > 0 {
// 				certificateList = append(certificateList, appsv1.RepositoryCertificate{
// 					ServerName: serverName,
// 					CertType:   "https",
// 					CertData:   []byte(strings.Join(certificateArray, "\n")),
// 				})
// 				certificates, err := certIf.CreateCertificate(context.Background(), &certificatepkg.RepositoryCertificateCreateRequest{
// 					Certificates: &appsv1.RepositoryCertificateList{
// 						Items: certificateList,
// 					},
// 					Upsert: upsert,
// 				})
// 				errors.CheckError(err)
// 				fmt.Printf("Created entry with %d PEM certificates for repository server %s\n", len(certificates.Items), serverName)
// 			} else {
// 				fmt.Printf("No valid certificates have been detected in the stream.\n")
// 			}
// 		},
// 	}
// 	command.Flags().StringVar(&fromFile, "from", "", "read TLS certificate data from file (default is to read from stdin)")
// 	command.Flags().BoolVar(&upsert, "upsert", false, "Replace existing TLS certificate if certificate is different in input")
// 	return command
// }

// // NewCertAddCommand returns a new instance of an `argocd cert add` command
// func NewCertAddSSHCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
// 	var (
// 		fromFile     string
// 		batchProcess bool
// 		upsert       bool
// 		certificates []appsv1.RepositoryCertificate
// 	)

// 	var command = &cobra.Command{
// 		Use:   "add-ssh --batch",
// 		Short: "Add SSH known host entries for repository servers",
// 		Run: func(c *cobra.Command, args []string) {

// 			conn, certIf := argocdclient.NewClientOrDie(clientOpts).NewCertClientOrDie()
// 			defer util.Close(conn)

// 			var sshKnownHostsLists []string
// 			var err error

// 			// --batch is a flag, but it is mandatory for now.
// 			if batchProcess {
// 				if fromFile != "" {
// 					fmt.Printf("Reading SSH known hosts entries from file '%s'\n", fromFile)
// 					sshKnownHostsLists, err = certutil.ParseSSHKnownHostsFromPath(fromFile)
// 				} else {
// 					fmt.Println("Enter SSH known hosts entries, one per line. Press CTRL-D when finished.")
// 					sshKnownHostsLists, err = certutil.ParseSSHKnownHostsFromStream(os.Stdin)
// 				}
// 			} else {
// 				err = fmt.Errorf("You need to specify --batch or specify --help for usage instructions")
// 			}

// 			errors.CheckError(err)

// 			if len(sshKnownHostsLists) == 0 {
// 				errors.CheckError(fmt.Errorf("No valid SSH known hosts data found."))
// 			}

// 			for _, knownHostsEntry := range sshKnownHostsLists {
// 				_, certSubType, certData, err := certutil.TokenizeSSHKnownHostsEntry(knownHostsEntry)
// 				errors.CheckError(err)
// 				hostnameList, _, err := certutil.KnownHostsLineToPublicKey(knownHostsEntry)
// 				errors.CheckError(err)
// 				// Each key could be valid for multiple hostnames
// 				for _, hostname := range hostnameList {
// 					certificate := appsv1.RepositoryCertificate{
// 						ServerName:  hostname,
// 						CertType:    "ssh",
// 						CertSubType: certSubType,
// 						CertData:    certData,
// 					}
// 					certificates = append(certificates, certificate)
// 				}
// 			}

// 			certList := &appsv1.RepositoryCertificateList{Items: certificates}
// 			response, err := certIf.CreateCertificate(context.Background(), &certificatepkg.RepositoryCertificateCreateRequest{
// 				Certificates: certList,
// 				Upsert:       upsert,
// 			})
// 			errors.CheckError(err)
// 			fmt.Printf("Successfully created %d SSH known host entries\n", len(response.Items))
// 		},
// 	}
// 	command.Flags().StringVar(&fromFile, "from", "", "Read SSH known hosts data from file (default is to read from stdin)")
// 	command.Flags().BoolVar(&batchProcess, "batch", false, "Perform batch processing by reading in SSH known hosts data (mandatory flag)")
// 	command.Flags().BoolVar(&upsert, "upsert", false, "Replace existing SSH server public host keys if key is different in input")
// 	return command
// }

// // NewCertRemoveCommand returns a new instance of an `argocd cert rm` command
// func NewCertRemoveCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
// 	var (
// 		certType    string
// 		certSubType string
// 		certQuery   certificatepkg.RepositoryCertificateQuery
// 	)
// 	var command = &cobra.Command{
// 		Use:   "rm REPOSERVER",
// 		Short: "Remove certificate of TYPE for REPOSERVER",
// 		Run: func(c *cobra.Command, args []string) {
// 			if len(args) < 1 {
// 				c.HelpFunc()(c, args)
// 				os.Exit(1)
// 			}
// 			conn, certIf := argocdclient.NewClientOrDie(clientOpts).NewCertClientOrDie()
// 			defer util.Close(conn)
// 			hostNamePattern := args[0]

// 			// Prevent the user from specifying a wildcard as hostname as precaution
// 			// measure -- the user could still use "?*" or any other pattern to
// 			// remove all certificates, but it's less likely that it happens by
// 			// accident.
// 			if hostNamePattern == "*" {
// 				err := fmt.Errorf("A single wildcard is not allowed as REPOSERVER name.")
// 				errors.CheckError(err)
// 			}
// 			certQuery = certificatepkg.RepositoryCertificateQuery{
// 				HostNamePattern: hostNamePattern,
// 				CertType:        certType,
// 				CertSubType:     certSubType,
// 			}
// 			removed, err := certIf.DeleteCertificate(context.Background(), &certQuery)
// 			errors.CheckError(err)
// 			if len(removed.Items) > 0 {
// 				for _, cert := range removed.Items {
// 					fmt.Printf("Removed cert for '%s' of type '%s' (subtype '%s')\n", cert.ServerName, cert.CertType, cert.CertSubType)
// 				}
// 			} else {
// 				fmt.Println("No certificates were removed (none matched the given patterns)")
// 			}
// 		},
// 	}
// 	command.Flags().StringVar(&certType, "cert-type", "", "Only remove certs of given type (ssh, https)")
// 	command.Flags().StringVar(&certSubType, "cert-sub-type", "", "Only remove certs of given sub-type (only for ssh)")
// 	return command
// }

// Print table of certificate info
func printKeyTable(keys []appsv1.GnuPGPublicKey) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "KEYID\tTYPE\tIDENTITY\n")

	for _, k := range keys {
		fmt.Fprintf(w, "%s\t%s\t%s\n", k.KeyID, strings.ToUpper(k.SubType), k.Owner)
	}
	_ = w.Flush()
}
