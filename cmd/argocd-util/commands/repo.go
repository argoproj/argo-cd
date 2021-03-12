package commands

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	apiv1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	cmdutil "github.com/argoproj/argo-cd/cmd/util"
	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/errors"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/settings"
)

const (
	ArgoCDNamespace  = "argocd"
	repoSecretPrefix = "repo"
)

func NewRepoCommand() *cobra.Command {
	var command = &cobra.Command{
		Use:   "repo",
		Short: "Manage repositories configuration",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}
	command.AddCommand(NewGenRepoSpecCommand())

	return command
}

func NewGenRepoSpecCommand() *cobra.Command {
	var (
		repoOpts     cmdutil.RepoOptions
		outputFormat string
	)

	// For better readability and easier formatting
	var repoAddExamples = `  
  # Add a Git repository via SSH using a private key for authentication, ignoring the server's host key:
  argocd-util repo generate-spec git@git.example.com:repos/repo --insecure-ignore-host-key --ssh-private-key-path ~/id_rsa

  # Add a Git repository via SSH on a non-default port - need to use ssh:// style URLs here
  argocd-util repo generate-spec ssh://git@git.example.com:2222/repos/repo --ssh-private-key-path ~/id_rsa

  # Add a private Git repository via HTTPS using username/password and TLS client certificates:
  argocd-util repo generate-spec https://git.example.com/repos/repo --username git --password secret --tls-client-cert-path ~/mycert.crt --tls-client-cert-key-path ~/mycert.key

  # Add a private Git repository via HTTPS using username/password without verifying the server's TLS certificate
  argocd-util repo generate-spec https://git.example.com/repos/repo --username git --password secret --insecure-skip-server-verification

  # Add a public Helm repository named 'stable' via HTTPS
  argocd-util repo generate-spec https://kubernetes-charts.storage.googleapis.com --type helm --name stable  

  # Add a private Helm repository named 'stable' via HTTPS
  argocd-util repo generate-spec https://kubernetes-charts.storage.googleapis.com --type helm --name stable --username test --password test

  # Add a private Helm OCI-based repository named 'stable' via HTTPS
  argocd-util repo generate-spec helm-oci-registry.cn-zhangjiakou.cr.aliyuncs.com --type helm --name stable --enable-oci --username test --password test
`

	var command = &cobra.Command{
		Use:     "generate-spec REPOURL",
		Short:   "Generate declarative config for a repo",
		Example: repoAddExamples,
		Run: func(c *cobra.Command, args []string) {
			log.SetLevel(log.WarnLevel)
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			// Repository URL
			repoOpts.Repo.Repo = args[0]

			// Specifying ssh-private-key-path is only valid for SSH repositories
			if repoOpts.SshPrivateKeyPath != "" {
				if ok, _ := git.IsSSHURL(repoOpts.Repo.Repo); ok {
					keyData, err := ioutil.ReadFile(repoOpts.SshPrivateKeyPath)
					if err != nil {
						log.Fatal(err)
					}
					repoOpts.Repo.SSHPrivateKey = string(keyData)
				} else {
					err := fmt.Errorf("--ssh-private-key-path is only supported for SSH repositories.")
					errors.CheckError(err)
				}
			}

			// tls-client-cert-path and tls-client-cert-key-key-path must always be
			// specified together
			if (repoOpts.TlsClientCertPath != "" && repoOpts.TlsClientCertKeyPath == "") || (repoOpts.TlsClientCertPath == "" && repoOpts.TlsClientCertKeyPath != "") {
				err := fmt.Errorf("--tls-client-cert-path and --tls-client-cert-key-path must be specified together")
				errors.CheckError(err)
			}

			// Specifying tls-client-cert-path is only valid for HTTPS repositories
			if repoOpts.TlsClientCertPath != "" {
				if git.IsHTTPSURL(repoOpts.Repo.Repo) {
					tlsCertData, err := ioutil.ReadFile(repoOpts.TlsClientCertPath)
					errors.CheckError(err)
					tlsCertKey, err := ioutil.ReadFile(repoOpts.TlsClientCertKeyPath)
					errors.CheckError(err)
					repoOpts.Repo.TLSClientCertData = string(tlsCertData)
					repoOpts.Repo.TLSClientCertKey = string(tlsCertKey)
				} else {
					err := fmt.Errorf("--tls-client-cert-path is only supported for HTTPS repositories")
					errors.CheckError(err)
				}
			}

			// Set repository connection properties only when creating repository, not
			// when creating repository credentials.
			// InsecureIgnoreHostKey is deprecated and only here for backwards compat
			repoOpts.Repo.InsecureIgnoreHostKey = repoOpts.InsecureIgnoreHostKey
			repoOpts.Repo.Insecure = repoOpts.InsecureSkipServerVerification
			repoOpts.Repo.EnableLFS = repoOpts.EnableLfs
			repoOpts.Repo.EnableOCI = repoOpts.EnableOci

			if repoOpts.Repo.Type == "helm" && repoOpts.Repo.Name == "" {
				errors.CheckError(fmt.Errorf("must specify --name for repos of type 'helm'"))
			}

			// If the user set a username, but didn't supply password via --password,
			// then we prompt for it
			if repoOpts.Repo.Username != "" && repoOpts.Repo.Password == "" {
				repoOpts.Repo.Password = cli.PromptPassword(repoOpts.Repo.Password)
			}

			argoCDCM := &apiv1.ConfigMap{
				TypeMeta: v1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: v1.ObjectMeta{
					Name:      common.ArgoCDConfigMapName,
					Namespace: ArgoCDNamespace,
					Labels: map[string]string{
						"app.kubernetes.io/part-of": "argocd",
					},
				},
			}
			kubeClientset := fake.NewSimpleClientset(argoCDCM)
			settingsMgr := settings.NewSettingsManager(context.Background(), kubeClientset, ArgoCDNamespace)
			argoDB := db.NewDB(ArgoCDNamespace, settingsMgr, kubeClientset)

			var printResources []interface{}
			_, err := argoDB.CreateRepository(context.Background(), &repoOpts.Repo)
			errors.CheckError(err)

			secret, err := kubeClientset.CoreV1().Secrets(ArgoCDNamespace).Get(context.Background(), db.RepoURLToSecretName(repoSecretPrefix, repoOpts.Repo.Repo), v1.GetOptions{})
			if err != nil {
				if !apierr.IsNotFound(err) {
					errors.CheckError(err)
				}
			} else {
				cmdutil.ConvertSecretData(secret)
				printResources = append(printResources, secret)
			}

			cm, err := kubeClientset.CoreV1().ConfigMaps(ArgoCDNamespace).Get(context.Background(), common.ArgoCDConfigMapName, v1.GetOptions{})
			errors.CheckError(err)

			printResources = append(printResources, cm)
			errors.CheckError(cmdutil.PrintResources(printResources, outputFormat))
		},
	}
	command.Flags().StringVarP(&outputFormat, "output", "o", "yaml", "Output format. One of: json|yaml")
	cmdutil.AddRepoFlags(command, &repoOpts)
	return command
}
