package generators

import (
	"context"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj/argo-cd/v2/applicationset/services"
)

func GetGenerators(ctx context.Context, c client.Client, k8sClient kubernetes.Interface, namespace string, argoCDService services.Repos, dynamicClient dynamic.Interface, scmConfig SCMConfig) map[string]Generator {
	terminalGenerators := map[string]Generator{
		"List":                    NewListGenerator(),
		"Clusters":                NewClusterGenerator(c, ctx, k8sClient, namespace),
		"Git":                     NewGitGenerator(argoCDService, namespace),
		"SCMProvider":             NewSCMProviderGenerator(c, scmConfig),
		"ClusterDecisionResource": NewDuckTypeGenerator(ctx, dynamicClient, k8sClient, namespace),
		"PullRequest":             NewPullRequestGenerator(c, scmConfig),
		"Plugin":                  NewPluginGenerator(c, ctx, k8sClient, namespace),
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
	}

	return topLevelGenerators
}
