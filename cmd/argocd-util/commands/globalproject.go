package commands

import (
	"bufio"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/gitops-engine/pkg/utils/errors"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/cli"

	// load the gcp plugin (required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// load the oidc plugin (required to authenticate with OpenID Connect).
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	// load the azure plugin (required to authenticate with AKS clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/azure"
)

// NewGlobalProjectGenCommand generates a project from clusterRole
func NewGlobalProjectGenCommand() *cobra.Command {
	var (
		clientConfig clientcmd.ClientConfig
		out          string
	)
	var command = &cobra.Command{
		Use:   "globalproject CLUSTERROLE_PATH",
		Short: "Generates global project for the specified clusterRole",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			clusterRoleFileName := args[0]

			var writer io.Writer
			if out == "-" {
				writer = os.Stdout
			} else {
				f, err := os.Create(out)
				errors.CheckError(err)
				bw := bufio.NewWriter(f)
				writer = bw
				defer func() {
					err = bw.Flush()
					errors.CheckError(err)
					err = f.Close()
					errors.CheckError(err)
				}()
			}

			globalProj := generateGlobalProject(clientConfig, clusterRoleFileName)

			yamlBytes, err := yaml.Marshal(globalProj)
			errors.CheckError(err)

			_, err = writer.Write(yamlBytes)
			errors.CheckError(err)
		},
	}
	clientConfig = cli.AddKubectlFlagsToCmd(command)
	command.Flags().StringVarP(&out, "out", "o", "-", "Output to the specified file instead of stdout")

	return command
}

func generateGlobalProject(clientConfig clientcmd.ClientConfig, clusterRoleFileName string) v1alpha1.AppProject {
	yamlBytes, err := ioutil.ReadFile(clusterRoleFileName)
	errors.CheckError(err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	errors.CheckError(err)

	clusterRole := &rbacv1.ClusterRole{}
	err = scheme.Scheme.Convert(&obj, clusterRole, nil)
	errors.CheckError(err)

	config, err := clientConfig.ClientConfig()
	errors.CheckError(err)
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	errors.CheckError(err)
	serverResources, err := disco.ServerPreferredResources()
	errors.CheckError(err)

	resourceList := make([]metav1.GroupKind, 0)
	for _, rule := range clusterRole.Rules {
		if len(rule.APIGroups) <= 0 {
			continue
		}

		canCreate := false
		for _, verb := range rule.Verbs {
			if strings.ToLower(verb) == strings.ToLower("Create") {
				canCreate = true
				break
			}
		}

		if !canCreate {
			continue
		}

		ruleApiGroup := rule.APIGroups[0]
		for _, ruleResource := range rule.Resources {
			for _, apiResourcesList := range serverResources {
				gv, err := schema.ParseGroupVersion(apiResourcesList.GroupVersion)
				if err != nil {
					gv = schema.GroupVersion{}
				}
				if ruleApiGroup == gv.Group {
					for _, apiResource := range apiResourcesList.APIResources {
						if apiResource.Name == ruleResource {
							resourceList = append(resourceList, metav1.GroupKind{Group: ruleApiGroup, Kind: apiResource.Kind})
						}
					}
				}
			}
		}
	}
	globalProj := v1alpha1.AppProject{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AppProject",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{Name: "argocd-global-iksm"},
		Spec: v1alpha1.AppProjectSpec{
			Destinations: []v1alpha1.ApplicationDestination{
				{Namespace: "*", Server: "https://kubernetes.default.svc"},
			},
			SourceRepos: []string{"*"},
		},
	}
	globalProj.Spec.NamespaceResourceWhitelist = resourceList
	return globalProj
}
