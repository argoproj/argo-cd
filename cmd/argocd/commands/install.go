package commands

import (
	"fmt"
	"time"

	"github.com/argoproj/argo-cd/pkg/apis/application"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// InstallFlags has all the required parameters for installing Argo CD.
type InstallFlags struct {
	DryRun bool // --dry-run
}

// NewInstallCommand returns a new instance of `argocd install` command
func NewInstallCommand(globalArgs *globalFlags) *cobra.Command {
	var (
		installArgs InstallFlags
	)
	var command = &cobra.Command{
		Use:   "install",
		Short: "Install the argocd components",
		Long:  "Install the argocd components",
		Run: func(c *cobra.Command, args []string) {
			extensionsClient := apiextensionsclient.NewForConfigOrDie(GetKubeConfig(globalArgs.kubeConfigPath, globalArgs.kubeConfigOverrides))
			installAppCRD(extensionsClient, installArgs)
			installClusterCRD(extensionsClient, installArgs)
		},
	}
	command.Flags().BoolVar(&installArgs.DryRun, "dry-run", false, "print the kubernetes manifests to stdout instead of installing")

	return command
}

func installAppCRD(extensionsClient *apiextensionsclient.Clientset, args InstallFlags) {
	applicationCRD := apiextensionsv1beta1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1alpha1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: application.ApplicationFullName,
		},
		Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
			Group:   application.Group,
			Version: appv1.SchemeGroupVersion.Version,
			Scope:   apiextensionsv1beta1.NamespaceScoped,
			Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
				Plural:     application.ApplicationPlural,
				Kind:       application.ApplicationKind,
				ShortNames: []string{application.ApplicationShortName},
			},
		},
	}
	createCRDHelper(extensionsClient, &applicationCRD, args.DryRun)
}

func installClusterCRD(extensionsClient *apiextensionsclient.Clientset, args InstallFlags) {
	clusterCRD := apiextensionsv1beta1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1alpha1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: application.ClusterFullName,
		},
		Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
			Group:   application.Group,
			Version: appv1.SchemeGroupVersion.Version,
			Scope:   apiextensionsv1beta1.NamespaceScoped,
			Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
				Plural:     application.ClusterPlural,
				Kind:       application.ClusterKind,
				ShortNames: []string{application.ClusterShortName},
			},
		},
	}
	createCRDHelper(extensionsClient, &clusterCRD, args.DryRun)
}

func createCRDHelper(extensionsClient *apiextensionsclient.Clientset, crd *apiextensionsv1beta1.CustomResourceDefinition, dryRun bool) {
	if dryRun {
		printYAML(crd)
		return
	}
	_, err := extensionsClient.ApiextensionsV1beta1().CustomResourceDefinitions().Create(crd)
	if err != nil {
		if !apierr.IsAlreadyExists(err) {
			log.Fatalf("Failed to create CustomResourceDefinition: %v", err)
		}
		fmt.Printf("CustomResourceDefinition '%s' already exists\n", crd.ObjectMeta.Name)
	} else {
		fmt.Printf("CustomResourceDefinition '%s' created", crd.ObjectMeta.Name)
	}
	// wait for CRD being established
	err = wait.Poll(500*time.Millisecond, 60*time.Second, func() (bool, error) {
		getCrd, err := extensionsClient.ApiextensionsV1beta1().CustomResourceDefinitions().Get(crd.ObjectMeta.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		for _, cond := range getCrd.Status.Conditions {
			switch cond.Type {
			case apiextensionsv1beta1.Established:
				if cond.Status == apiextensionsv1beta1.ConditionTrue {
					return true, err
				}
			case apiextensionsv1beta1.NamesAccepted:
				if cond.Status == apiextensionsv1beta1.ConditionFalse {
					log.Errorf("Name conflict: %v", cond.Reason)
				}
			}
		}
		return false, err
	})
	if err != nil {
		log.Fatalf("Failed to wait for CustomResourceDefinition: %v", err)
	}
}

func printYAML(obj interface{}) {
	objBytes, err := yaml.Marshal(obj)
	if err != nil {
		log.Fatalf("Failed to marshal %v", obj)
	}
	fmt.Printf("---\n%s\n", string(objBytes))
}
