package install

import (
	"fmt"
	"log"

	"github.com/yudai/gojsondiff/formatter"

	"github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/util/diff"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/ghodss/yaml"
	"github.com/gobuffalo/packr"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

var (
	// These values will be overridden by the link flags during build
	// (e.g. imageTag will use the official release tag on tagged builds)
	imageNamespace = "argoproj"
	imageTag       = "latest"

	// Default namespace and image names which `argocd install` uses during install
	DefaultInstallNamespace = "argocd"
	DefaultControllerImage  = imageNamespace + "/argocd-application-controller:" + imageTag
	DefaultUIImage          = imageNamespace + "/argocd-ui:" + imageTag
	DefaultServerImage      = imageNamespace + "/argocd-server:" + imageTag
)

type InstallOptions struct {
	DryRun          bool
	Upgrade         bool
	Namespace       string
	ControllerImage string
	UIImage         string
	ServerImage     string
	ImagePullPolicy string
}

type Installer struct {
	InstallOptions
	box           packr.Box
	config        *rest.Config
	dynClientPool dynamic.ClientPool
	disco         discovery.DiscoveryInterface
}

func NewInstaller(config *rest.Config, opts InstallOptions) (*Installer, error) {
	shallowCopy := *config
	inst := Installer{
		InstallOptions: opts,
		box:            packr.NewBox("./manifests"),
		config:         &shallowCopy,
	}
	var err error
	inst.dynClientPool = dynamic.NewDynamicClientPool(inst.config)
	inst.disco, err = discovery.NewDiscoveryClientForConfig(inst.config)
	if err != nil {
		return nil, err
	}
	return &inst, nil
}

func (i *Installer) Install() {
	i.InstallNamespace()
	i.InstallApplicationCRD()
	i.InstallApplicationController()
	i.InstallArgoCDServer()
}

func (i *Installer) InstallNamespace() {
	var namespace apiv1.Namespace
	i.unmarshalManifest("00_namespace.yaml", &namespace)
	namespace.ObjectMeta.Name = i.Namespace
	i.MustInstallResource(kube.MustToUnstructured(&namespace))
}

func (i *Installer) InstallApplicationCRD() {
	var applicationCRD apiextensionsv1beta1.CustomResourceDefinition
	i.unmarshalManifest("01_application-crd.yaml", &applicationCRD)
	i.MustInstallResource(kube.MustToUnstructured(&applicationCRD))
}

func (i *Installer) InstallApplicationController() {
	var applicationControllerServiceAccount apiv1.ServiceAccount
	var applicationControllerRole rbacv1.Role
	var applicationControllerRoleBinding rbacv1.RoleBinding
	var applicationControllerDeployment appsv1beta2.Deployment
	i.unmarshalManifest("02a_application-controller-sa.yaml", &applicationControllerServiceAccount)
	i.unmarshalManifest("02b_application-controller-role.yaml", &applicationControllerRole)
	i.unmarshalManifest("02c_application-controller-rolebinding.yaml", &applicationControllerRoleBinding)
	i.unmarshalManifest("02d_application-controller-deployment.yaml", &applicationControllerDeployment)
	applicationControllerDeployment.Spec.Template.Spec.Containers[0].Image = i.ControllerImage
	applicationControllerDeployment.Spec.Template.Spec.Containers[0].ImagePullPolicy = apiv1.PullPolicy(i.ImagePullPolicy)
	i.MustInstallResource(kube.MustToUnstructured(&applicationControllerServiceAccount))
	i.MustInstallResource(kube.MustToUnstructured(&applicationControllerRole))
	i.MustInstallResource(kube.MustToUnstructured(&applicationControllerRoleBinding))
	i.MustInstallResource(kube.MustToUnstructured(&applicationControllerDeployment))
}

func (i *Installer) InstallArgoCDServer() {
	var argoCDServerServiceAccount apiv1.ServiceAccount
	var argoCDServerControllerRole rbacv1.Role
	var argoCDServerControllerRoleBinding rbacv1.RoleBinding
	var argoCDServerControllerDeployment appsv1beta2.Deployment
	var argoCDServerService apiv1.Service
	i.unmarshalManifest("03a_argocd-server-sa.yaml", &argoCDServerServiceAccount)
	i.unmarshalManifest("03b_argocd-server-role.yaml", &argoCDServerControllerRole)
	i.unmarshalManifest("03c_argocd-server-rolebinding.yaml", &argoCDServerControllerRoleBinding)
	i.unmarshalManifest("03d_argocd-server-deployment.yaml", &argoCDServerControllerDeployment)
	i.unmarshalManifest("03e_argocd-server-service.yaml", &argoCDServerService)
	argoCDServerControllerDeployment.Spec.Template.Spec.InitContainers[0].Image = i.UIImage
	argoCDServerControllerDeployment.Spec.Template.Spec.InitContainers[0].ImagePullPolicy = apiv1.PullPolicy(i.ImagePullPolicy)
	argoCDServerControllerDeployment.Spec.Template.Spec.Containers[0].Image = i.ServerImage
	argoCDServerControllerDeployment.Spec.Template.Spec.Containers[0].ImagePullPolicy = apiv1.PullPolicy(i.ImagePullPolicy)
	i.MustInstallResource(kube.MustToUnstructured(&argoCDServerServiceAccount))
	i.MustInstallResource(kube.MustToUnstructured(&argoCDServerControllerRole))
	i.MustInstallResource(kube.MustToUnstructured(&argoCDServerControllerRoleBinding))
	i.MustInstallResource(kube.MustToUnstructured(&argoCDServerControllerDeployment))
	i.MustInstallResource(kube.MustToUnstructured(&argoCDServerService))
}

func (i *Installer) unmarshalManifest(fileName string, obj interface{}) {
	yamlBytes, err := i.box.MustBytes(fileName)
	errors.CheckError(err)
	err = yaml.Unmarshal(yamlBytes, obj)
	errors.CheckError(err)
}

func (i *Installer) MustInstallResource(obj *unstructured.Unstructured) *unstructured.Unstructured {
	obj, err := i.InstallResource(obj)
	errors.CheckError(err)
	return obj
}

func isNamespaced(obj *unstructured.Unstructured) bool {
	switch obj.GetKind() {
	case "Namespace", "ClusterRole", "ClusterRoleBinding", "CustomResourceDefinition":
		return false
	}
	return true
}

// InstallResource creates or updates a resource. If installed resource is up-to-date, does nothing
func (i *Installer) InstallResource(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	if isNamespaced(obj) {
		obj.SetNamespace(i.Namespace)
	}
	// remove 'creationTimestamp' and 'status' fields from object so that the diff will not be modified
	obj.SetCreationTimestamp(metav1.Time{})
	delete(obj.Object, "status")
	if i.DryRun {
		printYAML(obj)
		return nil, nil
	}
	gvk := obj.GroupVersionKind()
	dclient, err := i.dynClientPool.ClientForGroupVersionKind(gvk)
	if err != nil {
		return nil, err
	}
	apiResource, err := kube.ServerResourceForGroupVersionKind(i.disco, gvk)
	if err != nil {
		return nil, err
	}
	reIf := dclient.Resource(apiResource, i.Namespace)
	liveObj, err := reIf.Create(obj)
	if err == nil {
		fmt.Printf("%s '%s' created\n", liveObj.GetKind(), liveObj.GetName())
		return liveObj, nil
	}
	if !apierr.IsAlreadyExists(err) {
		return nil, err
	}
	liveObj, err = reIf.Get(obj.GetName(), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	diffRes := diff.Diff(obj, liveObj)
	if !diffRes.Modified {
		fmt.Printf("%s '%s' up-to-date\n", liveObj.GetKind(), liveObj.GetName())
		return liveObj, nil
	}
	if !i.Upgrade {
		log.Println(diffRes.ASCIIFormat(obj, formatter.AsciiFormatterConfig{}))
		return nil, fmt.Errorf("%s '%s' already exists. Rerun with --upgrade to update", obj.GetKind(), obj.GetName())
	}
	liveObj, err = reIf.Update(obj)
	if err != nil {
		return nil, err
	}
	fmt.Printf("%s '%s' updated\n", liveObj.GetKind(), liveObj.GetName())
	return liveObj, nil
}

func printYAML(obj interface{}) {
	objBytes, err := yaml.Marshal(obj)
	if err != nil {
		log.Fatalf("Failed to marshal %v", obj)
	}
	fmt.Printf("---\n%s\n", string(objBytes))
}
