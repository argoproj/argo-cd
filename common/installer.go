package common

import (
	"fmt"
	"reflect"
	"time"

	"github.com/argoproj/argo-cd/pkg/apis/application"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	apiv1 "k8s.io/api/core/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// InstallParameters has all the required parameters for installing ArgoCD.
type InstallParameters struct {
	Upgrade         bool
	DryRun          bool
	Namespace       string
	ControllerName  string
	ControllerImage string
	ServiceAccount  string
	SkipController  bool
}

// Installer allows to install ArgoCD resources.
type Installer struct {
	extensionsClient apiextensionsclient.Interface
	clientset        kubernetes.Interface
}

// Install performs installation
func (installer *Installer) Install(parameters InstallParameters) {
	installer.installAppCRD(parameters.DryRun)
	if !parameters.SkipController {
		installer.installController(parameters)
	}
}

// NewInstaller creates new instance of Installer
func NewInstaller(extensionsClient apiextensionsclient.Interface, clientset kubernetes.Interface) *Installer {
	return &Installer{
		extensionsClient: extensionsClient,
		clientset:        clientset,
	}
}

func (installer *Installer) installAppCRD(dryRun bool) {
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
	installer.createCRDHelper(&applicationCRD, dryRun)
}

func (installer *Installer) createCRDHelper(crd *apiextensionsv1beta1.CustomResourceDefinition, dryRun bool) {
	if dryRun {
		printYAML(crd)
		return
	}
	_, err := installer.extensionsClient.ApiextensionsV1beta1().CustomResourceDefinitions().Create(crd)
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
		getCrd, err := installer.extensionsClient.ApiextensionsV1beta1().CustomResourceDefinitions().Get(crd.ObjectMeta.Name, metav1.GetOptions{})
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

func (installer *Installer) installController(args InstallParameters) {
	controllerDeployment := appsv1beta2.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1beta2",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      args.ControllerName,
			Namespace: args.Namespace,
		},
		Spec: appsv1beta2.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": args.ControllerName,
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": args.ControllerName,
					},
				},
				Spec: apiv1.PodSpec{
					ServiceAccountName: args.ServiceAccount,
					Containers: []apiv1.Container{
						{
							Name:    args.ControllerName,
							Image:   args.ControllerImage,
							Command: []string{"argocd-application-controller"},
						},
					},
				},
			},
		},
	}
	installer.createDeploymentHelper(&controllerDeployment, args)
}

// createDeploymentHelper is helper to create or update an existing deployment (if --upgrade was supplied)
func (installer *Installer) createDeploymentHelper(deployment *appsv1beta2.Deployment, args InstallParameters) {
	depClient := installer.clientset.AppsV1beta2().Deployments(args.Namespace)
	var result *appsv1beta2.Deployment
	var err error
	if args.DryRun {
		printYAML(deployment)
		return
	}
	result, err = depClient.Create(deployment)
	if err != nil {
		if !apierr.IsAlreadyExists(err) {
			log.Fatal(err)
		}
		// deployment already exists
		existing, err := depClient.Get(deployment.ObjectMeta.Name, metav1.GetOptions{})
		if err != nil {
			log.Fatalf("Failed to get existing deployment: %v", err)
		}
		if upgradeNeeded(deployment, existing) {
			if !args.Upgrade {
				log.Fatalf("Deployment '%s' requires upgrade. Rerun with --upgrade to upgrade the deployment", deployment.ObjectMeta.Name)
			}
			existing, err = depClient.Update(deployment)
			if err != nil {
				log.Fatalf("Failed to update deployment: %v", err)
			}
			fmt.Printf("Existing deployment '%s' updated\n", existing.GetObjectMeta().GetName())
		} else {
			fmt.Printf("Existing deployment '%s' up-to-date\n", existing.GetObjectMeta().GetName())
		}
	} else {
		fmt.Printf("Deployment '%s' created\n", result.GetObjectMeta().GetName())
	}
}

// upgradeNeeded checks two deployments and returns whether or not there are obvious
// differences in a few deployment/container spec fields that would warrant an
// upgrade. WARNING: This is not intended to be comprehensive -- its primary purpose
// is to check if the controller/UI image is out of date with this version of argo.
func upgradeNeeded(dep1, dep2 *appsv1beta2.Deployment) bool {
	if len(dep1.Spec.Template.Spec.Containers) != len(dep2.Spec.Template.Spec.Containers) {
		return true
	}
	for i := 0; i < len(dep1.Spec.Template.Spec.Containers); i++ {
		ctr1 := dep1.Spec.Template.Spec.Containers[i]
		ctr2 := dep2.Spec.Template.Spec.Containers[i]
		if ctr1.Name != ctr2.Name {
			return true
		}
		if ctr1.Image != ctr2.Image {
			return true
		}
		if !reflect.DeepEqual(ctr1.Env, ctr2.Env) {
			return true
		}
		if !reflect.DeepEqual(ctr1.Command, ctr2.Command) {
			return true
		}
		if !reflect.DeepEqual(ctr1.Args, ctr2.Args) {
			return true
		}
	}
	return false
}

func printYAML(obj interface{}) {
	objBytes, err := yaml.Marshal(obj)
	if err != nil {
		log.Fatalf("Failed to marshal %v", obj)
	}
	fmt.Printf("---\n%s\n", string(objBytes))
}
