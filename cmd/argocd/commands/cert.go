package commands

import (
	"context"
	"crypto/x509"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/headless"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	certificatepkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/certificate"
	appsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	certutil "github.com/argoproj/argo-cd/v2/util/cert"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/io"
)

// NewCertCommand returns a new instance of an `argocd repo` command
func NewCertCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "cert",
		Short: "Manage repository certificates and SSH known hosts entries",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
		Example: `  # Add a TLS certificate for cd.example.com to ArgoCD cert store from a file
  argocd cert add-tls --from ~/mycert.pem cd.example.com

  # Add a TLS certificate for cd.example.com to ArgoCD via stdin
  cat ~/mycert.pem | argocd cert add-tls cd.example.com

  # Add SSH known host entries for cd.example.com to ArgoCD by scanning host
  ssh-keyscan cd.example.com | argocd cert add-ssh --batch

  # List all known TLS certificates
  argocd cert list --cert-type https

  # Remove all TLS certificates for cd.example.com
  argocd cert rm --cert-type https cd.example.com

  # Remove all certificates and SSH known host entries for cd.example.com
  argocd cert rm cd.example.com
`,
	}

	command.AddCommand(NewCertAddSSHCommand(clientOpts))
	command.AddCommand(NewCertAddTLSCommand(clientOpts))
	command.AddCommand(NewCertListCommand(clientOpts))
	command.AddCommand(NewCertRemoveCommand(clientOpts))
	return command
}

func NewCertAddTLSCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		fromFile string
		upsert   bool
	)
	var command = &cobra.Command{
		Use:   "add-tls SERVERNAME",
		Short: "Add TLS certificate data for connecting to repository server SERVERNAME",
		Run: func(c *cobra.Command, args []string) {
			conn, certIf := headless.NewClientOrDie(clientOpts, c).NewCertClientOrDie()
			defer io.Close(conn)

			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			var certificateArray []string
			var err error

			if fromFile != "" {
				fmt.Printf("Reading TLS certificate data in PEM format from '%s'\n", fromFile)
				certificateArray, err = certutil.ParseTLSCertificatesFromPath(fromFile)
			} else {
				fmt.Println("Enter TLS certificate data in PEM format. Press CTRL-D when finished.")
				certificateArray, err = certutil.ParseTLSCertificatesFromStream(os.Stdin)
			}

			errors.CheckError(err)

			certificateList := make([]appsv1.RepositoryCertificate, 0)

			subjectMap := make(map[string]*x509.Certificate)

			for _, entry := range certificateArray {
				// We want to make sure to only send valid certificate data to the
				// server, so we decode the certificate into X509 structure before
				// further processing it.
				x509cert, err := certutil.DecodePEMCertificateToX509(entry)
				errors.CheckError(err)

				// TODO: We need a better way to detect duplicates sent in the stream,
				// maybe by using fingerprints? For now, no two certs with the same
				// subject may be sent.
				if subjectMap[x509cert.Subject.String()] != nil {
					fmt.Printf("ERROR: Cert with subject '%s' already seen in the input stream.\n", x509cert.Subject.String())
					continue
				} else {
					subjectMap[x509cert.Subject.String()] = x509cert
				}
			}

			serverName := args[0]

			if len(certificateArray) > 0 {
				certificateList = append(certificateList, appsv1.RepositoryCertificate{
					ServerName: serverName,
					CertType:   "https",
					CertData:   []byte(strings.Join(certificateArray, "\n")),
				})
				certificates, err := certIf.CreateCertificate(context.Background(), &certificatepkg.RepositoryCertificateCreateRequest{
					Certificates: &appsv1.RepositoryCertificateList{
						Items: certificateList,
					},
					Upsert: upsert,
				})
				errors.CheckError(err)
				fmt.Printf("Created entry with %d PEM certificates for repository server %s\n", len(certificates.Items), serverName)
			} else {
				fmt.Printf("No valid certificates have been detected in the stream.\n")
			}
		},
	}
	command.Flags().StringVar(&fromFile, "from", "", "read TLS certificate data from file (default is to read from stdin)")
	command.Flags().BoolVar(&upsert, "upsert", false, "Replace existing TLS certificate if certificate is different in input")
	return command
}

// NewCertAddCommand returns a new instance of an `argocd cert add` command
func NewCertAddSSHCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		fromFile     string
		batchProcess bool
		upsert       bool
		certificates []appsv1.RepositoryCertificate
	)

	var command = &cobra.Command{
		Use:   "add-ssh --batch",
		Short: "Add SSH known host entries for repository servers",
		Run: func(c *cobra.Command, args []string) {

			conn, certIf := headless.NewClientOrDie(clientOpts, c).NewCertClientOrDie()
			defer io.Close(conn)

			var sshKnownHostsLists []string
			var err error

			// --batch is a flag, but it is mandatory for now.
			if batchProcess {
				if fromFile != "" {
					fmt.Printf("Reading SSH known hosts entries from file '%s'\n", fromFile)
					sshKnownHostsLists, err = certutil.ParseSSHKnownHostsFromPath(fromFile)
				} else {
					fmt.Println("Enter SSH known hosts entries, one per line. Press CTRL-D when finished.")
					sshKnownHostsLists, err = certutil.ParseSSHKnownHostsFromStream(os.Stdin)
				}
			} else {
				err = fmt.Errorf("You need to specify --batch or specify --help for usage instructions")
			}

			errors.CheckError(err)

			if len(sshKnownHostsLists) == 0 {
				errors.CheckError(fmt.Errorf("No valid SSH known hosts data found."))
			}

			for _, knownHostsEntry := range sshKnownHostsLists {
				_, certSubType, certData, err := certutil.TokenizeSSHKnownHostsEntry(knownHostsEntry)
				errors.CheckError(err)
				hostnameList, _, err := certutil.KnownHostsLineToPublicKey(knownHostsEntry)
				errors.CheckError(err)
				// Each key could be valid for multiple hostnames
				for _, hostname := range hostnameList {
					certificate := appsv1.RepositoryCertificate{
						ServerName:  hostname,
						CertType:    "ssh",
						CertSubType: certSubType,
						CertData:    certData,
					}
					certificates = append(certificates, certificate)
				}
			}

			certList := &appsv1.RepositoryCertificateList{Items: certificates}
			response, err := certIf.CreateCertificate(context.Background(), &certificatepkg.RepositoryCertificateCreateRequest{
				Certificates: certList,
				Upsert:       upsert,
			})
			errors.CheckError(err)
			fmt.Printf("Successfully created %d SSH known host entries\n", len(response.Items))
		},
	}
	command.Flags().StringVar(&fromFile, "from", "", "Read SSH known hosts data from file (default is to read from stdin)")
	command.Flags().BoolVar(&batchProcess, "batch", false, "Perform batch processing by reading in SSH known hosts data (mandatory flag)")
	command.Flags().BoolVar(&upsert, "upsert", false, "Replace existing SSH server public host keys if key is different in input")
	return command
}

// NewCertRemoveCommand returns a new instance of an `argocd cert rm` command
func NewCertRemoveCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		certType    string
		certSubType string
		certQuery   certificatepkg.RepositoryCertificateQuery
	)
	var command = &cobra.Command{
		Use:   "rm REPOSERVER",
		Short: "Remove certificate of TYPE for REPOSERVER",
		Run: func(c *cobra.Command, args []string) {
			if len(args) < 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, certIf := headless.NewClientOrDie(clientOpts, c).NewCertClientOrDie()
			defer io.Close(conn)
			hostNamePattern := args[0]

			// Prevent the user from specifying a wildcard as hostname as precaution
			// measure -- the user could still use "?*" or any other pattern to
			// remove all certificates, but it's less likely that it happens by
			// accident.
			if hostNamePattern == "*" {
				err := fmt.Errorf("A single wildcard is not allowed as REPOSERVER name.")
				errors.CheckError(err)
			}
			certQuery = certificatepkg.RepositoryCertificateQuery{
				HostNamePattern: hostNamePattern,
				CertType:        certType,
				CertSubType:     certSubType,
			}
			removed, err := certIf.DeleteCertificate(context.Background(), &certQuery)
			errors.CheckError(err)
			if len(removed.Items) > 0 {
				for _, cert := range removed.Items {
					fmt.Printf("Removed cert for '%s' of type '%s' (subtype '%s')\n", cert.ServerName, cert.CertType, cert.CertSubType)
				}
			} else {
				fmt.Println("No certificates were removed (none matched the given patterns)")
			}
		},
	}
	command.Flags().StringVar(&certType, "cert-type", "", "Only remove certs of given type (ssh, https)")
	command.Flags().StringVar(&certSubType, "cert-sub-type", "", "Only remove certs of given sub-type (only for ssh)")
	return command
}

// NewCertListCommand returns a new instance of an `argocd cert rm` command
func NewCertListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		certType        string
		hostNamePattern string
		sortOrder       string
		output          string
	)
	var command = &cobra.Command{
		Use:   "list",
		Short: "List configured certificates",
		Run: func(c *cobra.Command, args []string) {
			if certType != "" {
				switch certType {
				case "ssh":
				case "https":
				default:
					fmt.Println("cert-type must be either ssh or https")
					os.Exit(1)
				}
			}

			conn, certIf := headless.NewClientOrDie(clientOpts, c).NewCertClientOrDie()
			defer io.Close(conn)
			certificates, err := certIf.ListCertificates(context.Background(), &certificatepkg.RepositoryCertificateQuery{HostNamePattern: hostNamePattern, CertType: certType})
			errors.CheckError(err)

			switch output {
			case "yaml", "json":
				err := PrintResourceList(certificates.Items, output, false)
				errors.CheckError(err)
			case "wide", "":
				printCertTable(certificates.Items, sortOrder)
			default:
				errors.CheckError(fmt.Errorf("unknown output format: %s", output))
			}

		},
	}

	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: json|yaml|wide")
	command.Flags().StringVar(&sortOrder, "sort", "", "set display sort order for output format wide. One of: hostname|type")
	command.Flags().StringVar(&certType, "cert-type", "", "only list certificates of given type, valid: 'ssh','https'")
	command.Flags().StringVar(&hostNamePattern, "hostname-pattern", "", "only list certificates for hosts matching given glob-pattern")
	return command
}

// Print table of certificate info
func printCertTable(certs []appsv1.RepositoryCertificate, sortOrder string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "HOSTNAME\tTYPE\tSUBTYPE\tINFO\n")

	if sortOrder == "hostname" || sortOrder == "" {
		sort.Slice(certs, func(i, j int) bool {
			return certs[i].ServerName < certs[j].ServerName
		})
	} else if sortOrder == "type" {
		sort.Slice(certs, func(i, j int) bool {
			return certs[i].CertType < certs[j].CertType
		})
	}

	for _, c := range certs {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", c.ServerName, c.CertType, c.CertSubType, c.CertInfo)
	}
	_ = w.Flush()
}
