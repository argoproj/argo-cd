package generators

import (
	"context"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj/argo-cd/v3/applicationset/services"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

func GetGenerators(ctx context.Context, c client.Client, k8sClient kubernetes.Interface, controllerNamespace string, argoCDService services.Repos, dynamicClient dynamic.Interface, scmConfig SCMConfig, clusterInformer *settings.ClusterInformer) map[string]Generator {
	terminalGenerators := map[string]Generator{
		"List":                    NewListGenerator(),
		"Clusters":                NewClusterGenerator(c, controllerNamespace),
		"Git":                     NewGitGenerator(argoCDService, controllerNamespace),
		"SCMProvider":             NewSCMProviderGenerator(c, scmConfig),
		"ClusterDecisionResource": NewDuckTypeGenerator(ctx, dynamicClient, k8sClient, controllerNamespace, clusterInformer),
		"PullRequest":             NewPullRequestGenerator(c, scmConfig),
		"Plugin":                  NewPluginGenerator(c, controllerNamespace),
		"Oci":                     NewOciGenerator(argoCDService),
	}

	nestedGenerators := map[string]Generator{
		"List":                    terminalGenerators["List"],
		"Clusters":                terminalGenerators["Clusters"],
		"Git":                     terminalGenerators["Git"],
		"SCMProvider":             terminalGenerators["SCMProvider"],
		"ClusterDecisionResource": terminalGenerators["ClusterDecisionResource"],
		"PullRequest":             terminalGenerators["PullRequest"],
		"Plugin":                  terminalGenerators["Plugin"],
		"Matrix":                  NewMatrixGenerator(terminalGenerators),
		"Merge":                   NewMergeGenerator(terminalGenerators),
		"Oci":                     terminalGenerators["Oci"],
	}

	topLevelGenerators := map[string]Generator{
		"List":                    terminalGenerators["List"],
		"Clusters":                terminalGenerators["Clusters"],
		"Git":                     terminalGenerators["Git"],
		"SCMProvider":             terminalGenerators["SCMProvider"],
		"ClusterDecisionResource": terminalGenerators["ClusterDecisionResource"],
		"PullRequest":             terminalGenerators["PullRequest"],
		"Plugin":                  terminalGenerators["Plugin"],
		"Matrix":                  NewMatrixGenerator(nestedGenerators),
		"Merge":                   NewMergeGenerator(nestedGenerators),
		"Oci":                     terminalGenerators["Oci"],
	}

	return topLevelGenerators
}
