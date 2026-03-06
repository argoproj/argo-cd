package cluster

import (
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/clusterauth"

	clusterpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/cluster"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
)

// this implements the "when" part of given/when/then
//
// none of the func implement error checks, and that is complete intended, you should check for errors
// using the Then()
type Actions struct {
	context      *Context
	lastOutput   string
	lastError    error
	ignoreErrors bool
}

func (a *Actions) IgnoreErrors() *Actions {
	a.ignoreErrors = true
	return a
}

func (a *Actions) DoNotIgnoreErrors() *Actions {
	a.ignoreErrors = false
	return a
}

func (a *Actions) Create() *Actions {
	_, clusterClient, _ := fixture.ArgoCDClientset.NewClusterClient()

	_, err := clusterClient.Create(context.Background(), &clusterpkg.ClusterCreateRequest{
		Cluster: &v1alpha1.Cluster{
			Server:             a.context.server,
			Name:               a.context.GetName(),
			Config:             v1alpha1.ClusterConfig{BearerToken: a.context.bearerToken},
			Namespaces:         a.context.namespaces,
			RefreshRequestedAt: nil,
			Info:               v1alpha1.ClusterInfo{},
			Shard:              nil,
			ClusterResources:   false,
			Project:            a.context.project,
		},
		Upsert: a.context.upsert,
	})
	if err != nil {
		if !a.ignoreErrors {
			log.Fatalf("Failed to upsert cluster %v", err.Error())
		}
		a.lastError = errors.New(err.Error())
	}

	return a
}

func (a *Actions) CreateWithRBAC() *Actions {
	pathOpts := clientcmd.NewDefaultPathOptions()
	config, err := pathOpts.GetStartingConfig()
	if err != nil {
		a.lastError = err
		return a
	}
	clientConfig := clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{})
	conf, err := clientConfig.ClientConfig()
	if err != nil {
		a.lastError = err
		return a
	}
	client := kubernetes.NewForConfigOrDie(conf)

	_, err = clusterauth.InstallClusterManagerRBAC(client, "kube-system", []string{}, false, "", common.BearerTokenTimeout)
	if err != nil {
		a.lastError = err
		return a
	}

	// Create a kubeconfig with the current cluster name
	err = a.createKubeconfigForCluster(config, a.context.GetName())
	if err != nil {
		a.lastError = err
		return a
	}

	return a.Create()
}

// Helper function to create a kubeconfig file with the given cluster name
func (a *Actions) createKubeconfigForCluster(config *clientcmdapi.Config, newClusterName string) error {
	// Get the current context
	currentContext := config.Contexts[config.CurrentContext]
	if currentContext == nil {
		return errors.New("no current context found")
	}

	// Get the original cluster
	originalCluster := config.Clusters[currentContext.Cluster]
	if originalCluster == nil {
		return errors.New("cluster not found in config")
	}

	// Create a new cluster entry with the same config but different name
	newCluster := originalCluster.DeepCopy()
	config.Clusters[newClusterName] = newCluster

	// Create a new context pointing to the new cluster
	newContext := currentContext.DeepCopy()
	newContext.Cluster = newClusterName
	config.Contexts[newClusterName] = newContext

	// Set the new context as current
	config.CurrentContext = newClusterName

	// Write to a temporary kubeconfig file
	tmpFile, err := os.CreateTemp("", "kubeconfig-*.yaml")
	if err != nil {
		return err
	}
	defer tmpFile.Close()

	// Write the modified config to the temp file
	if err := clientcmd.WriteToFile(*config, tmpFile.Name()); err != nil {
		return err
	}

	// Set the KUBECONFIG environment variable to use this temp file
	// This will be use by subsequent kubectl/argocd commands to connect to the cluster
	a.context.T().Setenv("KUBECONFIG", tmpFile.Name())

	return nil
}

func (a *Actions) List() *Actions {
	a.context.T().Helper()
	a.runCli("cluster", "list")
	return a
}

func (a *Actions) Get() *Actions {
	a.context.T().Helper()
	a.runCli("cluster", "get", a.context.server)
	return a
}

func (a *Actions) GetByName() *Actions {
	a.context.T().Helper()
	a.runCli("cluster", "get", a.context.GetName())
	return a
}

func (a *Actions) SetNamespaces() *Actions {
	a.context.T().Helper()
	a.runCli("cluster", "set", a.context.GetName(), "--namespace", strings.Join(a.context.namespaces, ","))
	return a
}

func (a *Actions) DeleteByName() *Actions {
	a.context.T().Helper()

	a.runCli("cluster", "rm", a.context.GetName(), "--yes")
	return a
}

func (a *Actions) DeleteByServer() *Actions {
	a.context.T().Helper()

	a.runCli("cluster", "rm", a.context.server, "--yes")
	return a
}

func (a *Actions) Then() *Consequences {
	a.context.T().Helper()
	time.Sleep(fixture.WhenThenSleepInterval)
	return &Consequences{a.context, a}
}

func (a *Actions) runCli(args ...string) {
	a.context.T().Helper()
	a.lastOutput, a.lastError = fixture.RunCli(args...)
}
