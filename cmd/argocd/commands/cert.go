package commands

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	stderrors "errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v3/cmd/argocd/commands/headless"
	"github.com/argoproj/argo-cd/v3/cmd/argocd/commands/utils"
	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"
	certificatepkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/certificate"
	appsv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	certutil "github.com/argoproj/argo-cd/v3/util/cert"
	"github.com/argoproj/argo-cd/v3/util/cli"
	"github.com/argoproj/argo-cd/v3/util/errors"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
)

// NewCertCommand returns a new instance of an `argocd repo` command
func NewCertCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
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
	command.AddCommand(NewCertGetCommand(clientOpts))
	command.AddCommand(NewCertListCommand(clientOpts))
	command.AddCommand(NewCertRemoveCommand(clientOpts))
	return command
}

func NewCertAddTLSCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		fromFile string
		upsert   bool
	)

	command := &cobra.Command{
		Use:   "add-tls SERVERNAME",
		Short: "Add TLS certificate data for connecting to repository server SERVERNAME",
		Run: cli.WithSignalContext(func(c *cobra.Command, args []string, _ context.CancelFunc) {
			ctx := c.Context()

			conn, certIf := headless.NewClientOrDie(clientOpts, c).NewCertClientOrDieWithContext(ctx)
			defer utilio.Close(conn)

			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			var (
				certificateArray []string
				err              error
			)

			if fromFile != "" {
				fmt.Printf("Reading TLS certificate data in PEM format from '%s'\n", fromFile)
				certificateArray, err = certutil.ParseTLSCertificatesFromPath(fromFile)
			} else {
				fmt.Println("Enter TLS certificate data in PEM format. Press CTRL-D when finished.")
				certificateArray, err = certutil.ParseTLSCertificatesFromStream(os.Stdin)
			}
			errors.CheckError(err)

			uniqueCerts, err := deduplicatePEMCertificates(certificateArray)
			errors.CheckError(err)

			if len(uniqueCerts) == 0 {
				fmt.Println("No valid certificates have been detected in the stream.")
				return
			}

			serverName := args[0]

			certificateList := []appsv1.RepositoryCertificate{
				{
					ServerName: serverName,
					CertType:   "https",
					CertData:   []byte(strings.Join(uniqueCerts, "\n")),
				},
			}

			_, err = certIf.CreateCertificate(
				ctx,
				&certificatepkg.RepositoryCertificateCreateRequest{
					Certificates: &appsv1.RepositoryCertificateList{
						Items: certificateList,
					},
					Upsert: upsert,
				},
			)
			errors.CheckError(err)

			fmt.Printf(
				"Created/updated TLS certificate entry for repository server %s with %d unique PEM certificates\n",
				serverName,
				len(uniqueCerts),
			)
		}),
	}
	command.Flags().StringVar(&fromFile, "from", "", "Read TLS certificate data from file (default is to read from stdin)")
	command.Flags().BoolVar(&upsert, "upsert", false, "Replace existing TLS certificate if certificate is different in input")
	return command
}

// certFingerprintSHA256 returns the SHA256 fingerprint of the given X.509 certificate.
// The fingerprint is returned as a lowercase hexadecimal string.
func certFingerprintSHA256(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(sum[:])
}

// deduplicatePEMCertificates removes duplicate PEM certificates from the input slice.
// Two certificates are considered duplicates if their SHA256 fingerprints match.
// The function returns a slice of unique certificates in the original order.
// If any certificate cannot be decoded into X.509 format, an error is returned.
func deduplicatePEMCertificates(pems []string) ([]string, error) {
	fingerprintMap := make(map[string]struct{})
	uniqueCerts := make([]string, 0)

	for _, entry := range pems {
		x509cert, err := certutil.DecodePEMCertificateToX509(entry)
		if err != nil {
			return nil, err
		}

		fingerprint := certFingerprintSHA256(x509cert)

		if _, exists := fingerprintMap[fingerprint]; exists {
			fmt.Printf(
				"WARNING: Duplicate certificate detected (SHA256 fingerprint %s, subject '%s'), skipping.\n",
				fingerprint,
				x509cert.Subject.String(),
			)
			continue
		}

		fingerprintMap[fingerprint] = struct{}{}
		uniqueCerts = append(uniqueCerts, entry)
	}

	return uniqueCerts, nil
}

// NewCertAddSSHCommand returns a new instance of an `argocd cert add` command
func NewCertAddSSHCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		fromFile     string
		batchProcess bool
		upsert       bool
		certificates []appsv1.RepositoryCertificate
	)

	command := &cobra.Command{
		Use:   "add-ssh --batch",
		Short: "Add SSH known host entries for repository servers",
		Run: cli.WithSignalContext(func(c *cobra.Command, _ []string, _ context.CancelFunc) {
			ctx := c.Context()

			conn, certIf := headless.NewClientOrDie(clientOpts, c).NewCertClientOrDieWithContext(ctx)
			defer utilio.Close(conn)

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
				err = stderrors.New("you need to specify --batch or specify --help for usage instructions")
			}

			errors.CheckError(err)

			if len(sshKnownHostsLists) == 0 {
				errors.Fatal(errors.ErrorGeneric, "No valid SSH known hosts data found.")
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
			response, err := certIf.CreateCertificate(ctx, &certificatepkg.RepositoryCertificateCreateRequest{
				Certificates: certList,
				Upsert:       upsert,
			})
			errors.CheckError(err)
			fmt.Printf("Successfully created %d SSH known host entries\n", len(response.Items))
		}),
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
	command := &cobra.Command{
		Use:   "rm REPOSERVER",
		Short: "Remove certificate of TYPE for REPOSERVER",
		Run: cli.WithSignalContext(func(c *cobra.Command, args []string, _ context.CancelFunc) {
			ctx := c.Context()

			if len(args) < 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, certIf := headless.NewClientOrDie(clientOpts, c).NewCertClientOrDieWithContext(ctx)
			defer utilio.Close(conn)
			hostNamePattern := args[0]

			// Prevent the user from specifying a wildcard as hostname as precaution
			// measure -- the user could still use "?*" or any other pattern to
			// remove all certificates, but it's less likely that it happens by
			// accident.
			if hostNamePattern == "*" {
				errors.Fatal(errors.ErrorGeneric, "A single wildcard is not allowed as REPOSERVER name.")
			}

			promptUtil := utils.NewPrompt(clientOpts.PromptsEnabled)
			canDelete := promptUtil.Confirm(fmt.Sprintf("Are you sure you want to remove all certificates for '%s'? [y/n]", hostNamePattern))
			if canDelete {
				certQuery = certificatepkg.RepositoryCertificateQuery{
					HostNamePattern: hostNamePattern,
					CertType:        certType,
					CertSubType:     certSubType,
				}
				removed, err := certIf.DeleteCertificate(ctx, &certQuery)
				errors.CheckError(err)
				if len(removed.Items) > 0 {
					for _, cert := range removed.Items {
						fmt.Printf("Removed cert for '%s' of type '%s' (subtype '%s')\n", cert.ServerName, cert.CertType, cert.CertSubType)
					}
				} else {
					fmt.Println("No certificates were removed (none matched the given pattern)")
				}
			} else {
				fmt.Printf("The command to remove all certificates for '%s' was cancelled.\n", hostNamePattern)
			}
		}),
	}
	command.Flags().StringVar(&certType, "cert-type", "", "Only remove certs of given type (ssh, https)")
	command.Flags().StringVar(&certSubType, "cert-sub-type", "", "Only remove certs of given sub-type (only for ssh)")
	return command
}

// NewCertGetCommand returns a new instance of an `argocd cert get` command
func NewCertGetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		certType    string
		certSubType string
		output      string
	)
	command := &cobra.Command{
		Use:   "get SERVERNAME",
		Short: "Get the certificates and SSH known hosts entries configured for SERVERNAME",
		Example: `  # Get all certificates and SSH known hosts entries for github.com
  argocd cert get github.com

  # Get only the TLS certificates for cd.example.com in YAML format
  argocd cert get cd.example.com --cert-type https -o yaml
`,
		Run: cli.WithSignalContext(func(c *cobra.Command, args []string, _ context.CancelFunc) {
			ctx := c.Context()

			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			errors.CheckError(validateCertGetFlags(certType, output))

			conn, certIf := headless.NewClientOrDie(clientOpts, c).NewCertClientOrDieWithContext(ctx)
			defer utilio.Close(conn)
			certificates, err := certIf.GetCertificate(ctx, &certificatepkg.RepositoryCertificateGetRequest{
				ServerName:  args[0],
				CertType:    certType,
				CertSubType: certSubType,
			})
			errors.CheckError(err)
			errors.CheckError(printCertificates(certificates.Items, output, time.Now()))
		}),
	}
	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: json|yaml|wide")
	command.Flags().StringVar(&certType, "cert-type", "", "Only get certs of given type (ssh, https)")
	command.Flags().StringVar(&certSubType, "cert-sub-type", "", "Only get certs of given sub-type (e.g. ssh-ed25519 or rsa)")
	return command
}

// validateCertGetFlags checks the flags of `argocd cert get` before the server is contacted.
func validateCertGetFlags(certType, output string) error {
	switch certType {
	case "", "ssh", "https":
	default:
		return stderrors.New("cert-type must be either ssh or https")
	}
	switch output {
	case "", "wide", "json", "yaml":
	default:
		return fmt.Errorf("unknown output format: %s", output)
	}
	return nil
}

// printCertificates prints the given certificates in the requested output format.
func printCertificates(certs []appsv1.RepositoryCertificate, output string, now time.Time) error {
	switch output {
	case "yaml", "json":
		return PrintResourceList(certs, output, false)
	case "wide", "":
		printCertDetails(os.Stdout, certs, now)
		return nil
	default:
		return fmt.Errorf("unknown output format: %s", output)
	}
}

// printCertDetails prints all details of the given certificates, including
// their data. TLS certificates are decoded to show their X.509 metadata, and
// now is used to flag certificates that are expired or not yet valid.
func printCertDetails(out io.Writer, certs []appsv1.RepositoryCertificate, now time.Time) {
	for i, c := range certs {
		if i > 0 {
			fmt.Fprintln(out)
		}
		fmt.Fprintf(out, "Server name:  %s\n", c.ServerName)
		fmt.Fprintf(out, "Type:         %s\n", c.CertType)
		fmt.Fprintf(out, "Sub type:     %s\n", c.CertSubType)

		if c.CertType != "https" {
			fmt.Fprintf(out, "Fingerprint:  %s\n", c.CertInfo)
			fmt.Fprintf(out, "Data:         %s %s %s\n", c.ServerName, c.CertSubType, c.CertData)
			continue
		}

		x509Cert, err := certutil.DecodePEMCertificateToX509(string(c.CertData))
		if err != nil {
			fmt.Fprintf(out, "Info:         %s\n", c.CertInfo)
		} else {
			validity := ""
			switch {
			case now.After(x509Cert.NotAfter):
				validity = " (expired)"
			case now.Before(x509Cert.NotBefore):
				validity = " (not yet valid)"
			}
			fmt.Fprintf(out, "Subject:      %s\n", x509Cert.Subject.String())
			fmt.Fprintf(out, "Issuer:       %s\n", x509Cert.Issuer.String())
			fmt.Fprintf(out, "DNS names:    %s\n", strings.Join(x509Cert.DNSNames, ", "))
			fmt.Fprintf(out, "Valid from:   %s\n", x509Cert.NotBefore.UTC().Format(time.RFC3339))
			fmt.Fprintf(out, "Valid until:  %s%s\n", x509Cert.NotAfter.UTC().Format(time.RFC3339), validity)
			fmt.Fprintf(out, "Fingerprint:  SHA256:%s\n", certFingerprintSHA256(x509Cert))
		}
		fmt.Fprintf(out, "Data:\n%s\n", strings.TrimSpace(string(c.CertData)))
	}
}

// NewCertListCommand returns a new instance of an `argocd cert rm` command
func NewCertListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		certType        string
		hostNamePattern string
		sortOrder       string
		output          string
	)
	command := &cobra.Command{
		Use:   "list",
		Short: "List configured certificates",
		Run: cli.WithSignalContext(func(c *cobra.Command, _ []string, _ context.CancelFunc) {
			ctx := c.Context()

			if certType != "" {
				switch certType {
				case "ssh":
				case "https":
				default:
					fmt.Println("cert-type must be either ssh or https")
					os.Exit(1)
				}
			}

			conn, certIf := headless.NewClientOrDie(clientOpts, c).NewCertClientOrDieWithContext(ctx)
			defer utilio.Close(conn)
			certificates, err := certIf.ListCertificates(ctx, &certificatepkg.RepositoryCertificateQuery{HostNamePattern: hostNamePattern, CertType: certType})
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
		}),
	}

	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: json|yaml|wide")
	command.Flags().StringVar(&sortOrder, "sort", "", "Set display sort order for output format wide. One of: hostname|type")
	command.Flags().StringVar(&certType, "cert-type", "", "Only list certificates of given type, valid: 'ssh','https'")
	command.Flags().StringVar(&hostNamePattern, "hostname-pattern", "", "Only list certificates for hosts matching given glob-pattern")
	return command
}

// Print table of certificate info
func printCertTable(certs []appsv1.RepositoryCertificate, sortOrder string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprint(w, "HOSTNAME\tTYPE\tSUBTYPE\tINFO\n")

	switch sortOrder {
	case "hostname", "":
		sort.Slice(certs, func(i, j int) bool {
			return certs[i].ServerName < certs[j].ServerName
		})
	case "type":
		sort.Slice(certs, func(i, j int) bool {
			return certs[i].CertType < certs[j].CertType
		})
	}

	for _, c := range certs {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", c.ServerName, c.CertType, c.CertSubType, c.CertInfo)
	}
	_ = w.Flush()
}
