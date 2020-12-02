package commands

import (
	"context"
	"fmt"
	"github.com/ghodss/yaml"
	"io/ioutil"
	"log"
	"os"

	"github.com/spf13/cobra"
	apiv1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	cmdutil "github.com/argoproj/argo-cd/cmd/util"
	"github.com/argoproj/argo-cd/common"
	appsv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/argo"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/errors"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/settings"
)

func NewGenRepoConfigCommand() *cobra.Command {
	var (
		repoOpts     cmdutil.RepoOptions
		clientConfig clientcmd.ClientConfig
		outputFormat string
	)

	// For better readability and easier formatting
	var repoAddExamples = `  # Add a Git repository via SSH using a private key for authentication, ignoring the server's host key:
	argocd-util config repo-add git@git.example.com:repos/repo --insecure-ignore-host-key --ssh-private-key-path ~/id_rsa

	# Add a Git repository via SSH on a non-default port - need to use ssh:// style URLs here
	argocd-util config repo-add ssh://git@git.example.com:2222/repos/repo --ssh-private-key-path ~/id_rsa

  # Add a private Git repository via HTTPS using username/password and TLS client certificates:
  argocd-util config repo-add https://git.example.com/repos/repo --username git --password secret --tls-client-cert-path ~/mycert.crt --tls-client-cert-key-path ~/mycert.key

  # Add a private Git repository via HTTPS using username/password without verifying the server's TLS certificate
  argocd-util config repo-add https://git.example.com/repos/repo --username git --password secret --insecure-skip-server-verification

  # Add a public Helm repository named 'stable' via HTTPS
  argocd-util config repo-add https://kubernetes-charts.storage.googleapis.com --type helm --name stable  

  # Add a private Helm repository named 'stable' via HTTPS
  argocd-util config repo-add https://kubernetes-charts.storage.googleapis.com --type helm --name stable --username test --password test

  # Add a private Helm OCI-based repository named 'stable' via HTTPS
  argocd-util config repo-add helm-oci-registry.cn-zhangjiakou.cr.aliyuncs.com --type helm --name stable --enable-oci --username test --password test
`

	var command = &cobra.Command{
		Use:     "repo-add REPOURL",
		Short:   "Generate declarative config for a repo",
		Example: repoAddExamples,
		Run: func(c *cobra.Command, args []string) {
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
				errors.CheckError(fmt.Errorf("Must specify --name for repos of type 'helm'"))
			}

			/*conn, repoIf := argocdclient.NewClientOrDie(clientOpts).NewRepoClientOrDie()
			defer io.Close(conn)*/

			// If the user set a username, but didn't supply password via --password,
			// then we prompt for it
			if repoOpts.Repo.Username != "" && repoOpts.Repo.Password == "" {
				repoOpts.Repo.Password = cli.PromptPassword(repoOpts.Repo.Password)
			}

			// We let the server check access to the repository before adding it. If
			// it is a private repo, but we cannot access with with the credentials
			// that were supplied, we bail out.
			//
			// Skip validation if we are just adding credentials template, chances
			// are high that we do not have the given URL pointing to a valid Git
			// repo anyway.
			/*repoAccessReq := repositorypkg.RepoAccessQuery{
				Repo:              repo.Repo,
				Type:              repo.Type,
				Name:              repo.Name,
				Username:          repo.Username,
				Password:          repo.Password,
				SshPrivateKey:     repo.SSHPrivateKey,
				TlsClientCertData: repo.TLSClientCertData,
				TlsClientCertKey:  repo.TLSClientCertKey,
				Insecure:          repo.IsInsecure(),
				EnableOci:         repo.EnableOCI,
			}
			_, err := repoIf.ValidateAccess(context.Background(), &repoAccessReq)
			errors.CheckError(err)

			repoCreateReq := repositorypkg.RepoCreateRequest{
				Repo:   &repo,
				Upsert: upsert,
			}

			createdRepo, err := repoIf.Create(context.Background(), &repoCreateReq)
			errors.CheckError(err)*/
			//fmt.Printf("repository '%s' added\n", createdRepo.Repo)

			cfg, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)

			kubeClientset := kubernetes.NewForConfigOrDie(cfg)
			settingsMgr := settings.NewSettingsManager(context.Background(), kubeClientset, namespace)
			argoDB := db.NewDB(namespace, settingsMgr, kubeClientset)

			err = validateConnection(&repoOpts.Repo, argoDB)
			errors.CheckError(err)

			repoInfo := settings.Repository{
				URL:                   repoOpts.Repo.Repo,
				Type:                  repoOpts.Repo.Type,
				Name:                  repoOpts.Repo.Name,
				InsecureIgnoreHostKey: repoOpts.Repo.IsInsecure(),
				Insecure:              repoOpts.Repo.IsInsecure(),
				EnableLFS:             repoOpts.Repo.EnableLFS,
				EnableOci:             repoOpts.Repo.EnableOCI,
			}
			secrets, err := generateSecrets(repoInfo, &repoOpts.Repo, argoDB)
			errors.CheckError(err)
			for _, s := range secrets {
				err = cmdutil.PrintResource(s, outputFormat)
				errors.CheckError(err)
			}

			repos, err := settingsMgr.GetRepositories()
			errors.CheckError(err)

			repos = append(repos, repoInfo)
			cm, err := generateConfigMap(repos, namespace, kubeClientset, settingsMgr)
			errors.CheckError(err)
			err = cmdutil.PrintResource(cm, outputFormat)
			errors.CheckError(err)
		},
	}
	clientConfig = cli.AddKubectlFlagsToCmd(command)
	command.Flags().StringVar(&outputFormat, "o", "yaml", "Output format (yaml|json)")
	cmdutil.AddRepoFlags(command, &repoOpts)
	return command
}

// check we can connect to the repo, copying any existing creds
func validateConnection(repository *appsv1.Repository, db db.ArgoDB) error {
	repo := repository.DeepCopy()
	if !repo.HasCredentials() {
		creds, err := db.GetRepositoryCredentials(context.Background(), repo.Repo)
		if err != nil {
			return err
		}
		repo.CopyCredentialsFrom(creds)
	}
	err := argo.TestRepo(repo)
	if err != nil {
		return err
	}
	return nil
}

func generateSecrets(repoInfo settings.Repository, r *appsv1.Repository, argoDB db.ArgoDB) ([]*apiv1.Secret, error) {
	return argoDB.UpdateRepositorySecrets(&repoInfo, r, db.UpsertOptions{DryRun: true})
}

func generateConfigMap(repos []settings.Repository, namespace string, kubeClientset *kubernetes.Clientset, mgr *settings.SettingsManager) (*apiv1.ConfigMap, error) {
	argoCDCM, err := mgr.GetConfigMap()
	createCM := false
	if err != nil {
		if !apierr.IsNotFound(err) {
			return nil, err
		}
		argoCDCM = &apiv1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: common.ArgoCDConfigMapName,
			},
		}
		createCM = true
	}
	if argoCDCM.Data == nil {
		argoCDCM.Data = make(map[string]string)
	}

	if len(repos) > 0 {
		yamlStr, err := yaml.Marshal(repos)
		if err != nil {
			return nil, err
		}
		argoCDCM.Data[settings.RepositoriesKey] = string(yamlStr)
	} else {
		delete(argoCDCM.Data, settings.RepositoriesKey)
	}

	var cm *apiv1.ConfigMap
	if createCM {
		cm, err = kubeClientset.CoreV1().ConfigMaps(namespace).Create(context.Background(), argoCDCM, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}})
	} else {
		cm, err = kubeClientset.CoreV1().ConfigMaps(namespace).Update(context.Background(), argoCDCM, metav1.UpdateOptions{DryRun: []string{metav1.DryRunAll}})
	}
	if err != nil {
		return nil, err
	}

	mgr.InvalidateCache()

	return cm, nil
}
