package util

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/errors"
)

func PrintKubeContexts(ca clientcmd.ConfigAccess) {
	config, err := ca.GetStartingConfig()
	errors.CheckError(err)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer func() { _ = w.Flush() }()
	columnNames := []string{"CURRENT", "NAME", "CLUSTER", "SERVER"}
	_, err = fmt.Fprintf(w, "%s\n", strings.Join(columnNames, "\t"))
	errors.CheckError(err)

	// sort names so output is deterministic
	contextNames := make([]string, 0)
	for name := range config.Contexts {
		contextNames = append(contextNames, name)
	}
	sort.Strings(contextNames)

	if config.Clusters == nil {
		return
	}

	for _, name := range contextNames {
		// ignore malformed kube config entries
		context := config.Contexts[name]
		if context == nil {
			continue
		}
		cluster := config.Clusters[context.Cluster]
		if cluster == nil {
			continue
		}
		prefix := " "
		if config.CurrentContext == name {
			prefix = "*"
		}
		_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", prefix, name, context.Cluster, cluster.Server)
		errors.CheckError(err)
	}
}

func NewCluster(name string, namespaces []string, conf *rest.Config, managerBearerToken string, awsAuthConf *argoappv1.AWSAuthConfig, execProviderConf *argoappv1.ExecProviderConfig) *argoappv1.Cluster {
	tlsClientConfig := argoappv1.TLSClientConfig{
		Insecure:   conf.TLSClientConfig.Insecure,
		ServerName: conf.TLSClientConfig.ServerName,
		CAData:     conf.TLSClientConfig.CAData,
		CertData:   conf.TLSClientConfig.CertData,
		KeyData:    conf.TLSClientConfig.KeyData,
	}
	if len(conf.TLSClientConfig.CAData) == 0 && conf.TLSClientConfig.CAFile != "" {
		data, err := ioutil.ReadFile(conf.TLSClientConfig.CAFile)
		errors.CheckError(err)
		tlsClientConfig.CAData = data
	}
	if len(conf.TLSClientConfig.CertData) == 0 && conf.TLSClientConfig.CertFile != "" {
		data, err := ioutil.ReadFile(conf.TLSClientConfig.CertFile)
		errors.CheckError(err)
		tlsClientConfig.CertData = data
	}
	if len(conf.TLSClientConfig.KeyData) == 0 && conf.TLSClientConfig.KeyFile != "" {
		data, err := ioutil.ReadFile(conf.TLSClientConfig.KeyFile)
		errors.CheckError(err)
		tlsClientConfig.KeyData = data
	}

	clst := argoappv1.Cluster{
		Server:     conf.Host,
		Name:       name,
		Namespaces: namespaces,
		Config: argoappv1.ClusterConfig{
			TLSClientConfig:    tlsClientConfig,
			AWSAuthConfig:      awsAuthConf,
			ExecProviderConfig: execProviderConf,
		},
	}

	// Bearer token will preferentially be used for auth if present,
	// Even in presence of key/cert credentials
	// So set bearer token only if the key/cert data is absent
	if len(tlsClientConfig.CertData) == 0 || len(tlsClientConfig.KeyData) == 0 {
		clst.Config.BearerToken = managerBearerToken
	}

	return &clst
}

type ClusterOptions struct {
	InCluster               bool
	Upsert                  bool
	ServiceAccount          string
	AwsRoleArn              string
	AwsClusterName          string
	SystemNamespace         string
	Namespaces              []string
	Name                    string
	Shard                   int64
	ExecProviderCommand     string
	ExecProviderArgs        []string
	ExecProviderEnv         map[string]string
	ExecProviderAPIVersion  string
	ExecProviderInstallHint string
}

func AddClusterFlags(command *cobra.Command, opts *ClusterOptions) {
	command.Flags().BoolVar(&opts.InCluster, "in-cluster", false, "Indicates Argo CD resides inside this cluster and should connect using the internal k8s hostname (kubernetes.default.svc)")
	command.Flags().StringVar(&opts.AwsClusterName, "aws-cluster-name", "", "AWS Cluster name if set then aws cli eks token command will be used to access cluster")
	command.Flags().StringVar(&opts.AwsRoleArn, "aws-role-arn", "", "Optional AWS role arn. If set then AWS IAM Authenticator assume a role to perform cluster operations instead of the default AWS credential provider chain.")
	command.Flags().StringArrayVar(&opts.Namespaces, "namespace", nil, "List of namespaces which are allowed to manage")
	command.Flags().StringVar(&opts.Name, "name", "", "Overwrite the cluster name")
	command.Flags().Int64Var(&opts.Shard, "shard", -1, "Cluster shard number; inferred from hostname if not set")
	command.Flags().StringVar(&opts.ExecProviderCommand, "exec-command", "", "Command to run to provide client credentials to the cluster. You may need to build a custom ArgoCD image to ensure the command is available at runtime.")
	command.Flags().StringArrayVar(&opts.ExecProviderArgs, "exec-command-args", nil, "Arguments to supply to the --exec-command command")
	command.Flags().StringToStringVar(&opts.ExecProviderEnv, "exec-command-env", nil, "Environment vars to set when running the --exec-command command")
	command.Flags().StringVar(&opts.ExecProviderAPIVersion, "exec-command-api-version", "", "Preferred input version of the ExecInfo for the --exec-command")
	command.Flags().StringVar(&opts.ExecProviderInstallHint, "exec-command-install-hint", "", "Text shown to the user when the --exec-command executable doesn't seem to be present")
}
