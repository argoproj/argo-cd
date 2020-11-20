package util

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

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
