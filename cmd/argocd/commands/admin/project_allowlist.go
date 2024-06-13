package admin

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v2/util/errors"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/cli"

	// load the gcp plugin (required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// load the oidc plugin (required to authenticate with OpenID Connect).
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	// load the azure plugin (required to authenticate with AKS clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/azure"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application"
)

// NewProjectAllowListGenCommand generates a project from clusterRole
func NewProjectAllowListGenCommand() *cobra.Command {
	var (
		clientConfig clientcmd.ClientConfig
		out          string
	)
	command := &cobra.Command{
		Use:   "generate-allow-list CLUSTERROLE_PATH PROJ_NAME",
		Short: "Generates project allow list from the specified clusterRole file",
		Example: `# Generates project allow list from the specified clusterRole file
argocd admin proj generate-allow-list /path/to/clusterrole.yaml my-project`,
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			clusterRoleFileName := args[0]
			projName := args[1]

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

			resourceList, err := getResourceList(clientConfig)
			errors.CheckError(err)
			globalProj, err := generateProjectAllowList(resourceList, clusterRoleFileName, projName)
			errors.CheckError(err)

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

func getResourceList(clientConfig clientcmd.ClientConfig) ([]*metav1.APIResourceList, error) {
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("error while creating client config: %w", err)
	}
	disco, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error while creating discovery client: %w", err)
	}
	serverResources, err := disco.ServerPreferredResources()
	if err != nil {
		return nil, fmt.Errorf("error while getting server resources: %w", err)
	}
	return serverResources, nil
}

func generateProjectAllowList(serverResources []*metav1.APIResourceList, clusterRoleFileName string, projName string) (*v1alpha1.AppProject, error) {
	yamlBytes, err := os.ReadFile(clusterRoleFileName)
	if err != nil {
		return nil, fmt.Errorf("error reading cluster role file: %w", err)
	}
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling cluster role file yaml: %w", err)
	}

	clusterRole := &rbacv1.ClusterRole{}
	err = scheme.Scheme.Convert(&obj, clusterRole, nil)
	if err != nil {
		return nil, fmt.Errorf("error converting cluster role yaml into ClusterRole struct: %w", err)
	}

	resourceList := make([]metav1.GroupKind, 0)
	for _, rule := range clusterRole.Rules {
		if len(rule.APIGroups) == 0 {
			continue
		}

		canCreate := false
		for _, verb := range rule.Verbs {
			if strings.EqualFold(verb, "Create") {
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
			Kind:       application.AppProjectKind,
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{Name: projName},
		Spec:       v1alpha1.AppProjectSpec{},
	}
	globalProj.Spec.NamespaceResourceWhitelist = resourceList
	return &globalProj, nil
}
