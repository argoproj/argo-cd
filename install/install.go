package install

import (
	"fmt"
	"strings"
	"syscall"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/util/diff"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/password"
	"github.com/argoproj/argo-cd/util/session"
	"github.com/argoproj/argo-cd/util/settings"
	tlsutil "github.com/argoproj/argo-cd/util/tls"
	"github.com/ghodss/yaml"
	"github.com/gobuffalo/packr"
	log "github.com/sirupsen/logrus"
	"github.com/yudai/gojsondiff/formatter"
	"golang.org/x/crypto/ssh/terminal"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
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
	DefaultRepoServerImage  = imageNamespace + "/argocd-repo-server:" + imageTag
)

// InstallOptions stores a collection of installation settings.
type InstallOptions struct {
	DryRun            bool
	Upgrade           bool
	UpdateSuperuser   bool
	UpdateSignature   bool
	SuperuserPassword string
	Namespace         string
	ControllerImage   string
	UIImage           string
	ServerImage       string
	RepoServerImage   string
	ImagePullPolicy   string
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
	if opts.Namespace == "" {
		inst.Namespace = DefaultInstallNamespace
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
	i.InstallSettings()
	i.InstallApplicationController()
	i.InstallArgoCDServer()
	i.InstallArgoCDRepoServer()
}

func (i *Installer) Uninstall(deleteNamespace, deleteCRD bool) {
	manifests := i.box.List()
	for _, manifestPath := range manifests {
		if strings.HasSuffix(manifestPath, ".yaml") || strings.HasSuffix(manifestPath, ".yml") {
			var obj unstructured.Unstructured
			i.unmarshalManifest(manifestPath, &obj)
			switch strings.ToLower(obj.GetKind()) {
			case "namespace":
				if !deleteNamespace {
					log.Infof("Skipped deletion of Namespace: '%s'", obj.GetName())
					continue
				}
			case "customresourcedefinition":
				if !deleteCRD {
					log.Infof("Skipped deletion of CustomResourceDefinition: '%s'", obj.GetName())
					continue
				}
			}
			i.MustUninstallResource(&obj)
		}
	}
}

func (i *Installer) InstallNamespace() {
	if i.Namespace != DefaultInstallNamespace {
		// don't create namespace if a different one was supplied
		return
	}
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

func (i *Installer) InstallSettings() {
	kubeclientset, err := kubernetes.NewForConfig(i.config)
	errors.CheckError(err)
	settingsMgr := settings.NewSettingsManager(kubeclientset, i.Namespace)
	cdSettings, err := settingsMgr.GetSettings()
	if err != nil {
		if apierr.IsNotFound(err) {
			cdSettings = &settings.ArgoCDSettings{}
		} else {
			log.Fatal(err)
		}
	}

	if cdSettings.ServerSignature == nil || i.UpdateSignature {
		// set JWT signature
		signature, err := session.MakeSignature(32)
		errors.CheckError(err)
		cdSettings.ServerSignature = signature
	}

	if cdSettings.LocalUsers == nil {
		cdSettings.LocalUsers = make(map[string]string)
	}
	if _, ok := cdSettings.LocalUsers[common.ArgoCDAdminUsername]; !ok || i.UpdateSuperuser {
		passwordRaw := i.SuperuserPassword
		if passwordRaw == "" {
			passwordRaw = readAndConfirmPassword()
		}
		hashedPassword, err := password.HashPassword(passwordRaw)
		errors.CheckError(err)
		cdSettings.LocalUsers = map[string]string{
			common.ArgoCDAdminUsername: hashedPassword,
		}
	}

	if cdSettings.Certificate == nil {
		// generate TLS cert
		hosts := []string{
			"localhost",
			"argocd-server",
			fmt.Sprintf("argocd-server.%s", i.Namespace),
			fmt.Sprintf("argocd-server.%s.svc", i.Namespace),
			fmt.Sprintf("argocd-server.%s.svc.cluster.local", i.Namespace),
		}
		certOpts := tlsutil.CertOptions{
			Hosts:        hosts,
			Organization: "Argo CD",
			IsCA:         true,
		}
		cert, err := tlsutil.GenerateX509KeyPair(certOpts)
		errors.CheckError(err)
		cdSettings.Certificate = cert
	}

	err = settingsMgr.SaveSettings(cdSettings)
	errors.CheckError(err)
}

func readAndConfirmPassword() string {
	for {
		fmt.Print("*** Enter an admin password: ")
		password, err := terminal.ReadPassword(syscall.Stdin)
		errors.CheckError(err)
		fmt.Print("\n")
		fmt.Print("*** Confirm the admin password: ")
		confirmPassword, err := terminal.ReadPassword(syscall.Stdin)
		errors.CheckError(err)
		fmt.Print("\n")
		if string(password) == string(confirmPassword) {
			return string(password)
		}
		log.Error("Passwords do not match")
	}
}

func (i *Installer) InstallApplicationController() {
	var applicationControllerServiceAccount apiv1.ServiceAccount
	var applicationControllerRole rbacv1.Role
	var applicationControllerRoleBinding rbacv1.RoleBinding
	var applicationControllerDeployment appsv1beta2.Deployment
	i.unmarshalManifest("03a_application-controller-sa.yaml", &applicationControllerServiceAccount)
	i.unmarshalManifest("03b_application-controller-role.yaml", &applicationControllerRole)
	i.unmarshalManifest("03c_application-controller-rolebinding.yaml", &applicationControllerRoleBinding)
	i.unmarshalManifest("03d_application-controller-deployment.yaml", &applicationControllerDeployment)
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
	i.unmarshalManifest("04a_argocd-server-sa.yaml", &argoCDServerServiceAccount)
	i.unmarshalManifest("04b_argocd-server-role.yaml", &argoCDServerControllerRole)
	i.unmarshalManifest("04c_argocd-server-rolebinding.yaml", &argoCDServerControllerRoleBinding)
	i.unmarshalManifest("04d_argocd-server-deployment.yaml", &argoCDServerControllerDeployment)
	i.unmarshalManifest("04e_argocd-server-service.yaml", &argoCDServerService)
	argoCDServerControllerDeployment.Spec.Template.Spec.InitContainers[0].Image = i.ServerImage
	argoCDServerControllerDeployment.Spec.Template.Spec.InitContainers[0].ImagePullPolicy = apiv1.PullPolicy(i.ImagePullPolicy)
	argoCDServerControllerDeployment.Spec.Template.Spec.InitContainers[1].Image = i.UIImage
	argoCDServerControllerDeployment.Spec.Template.Spec.InitContainers[1].ImagePullPolicy = apiv1.PullPolicy(i.ImagePullPolicy)
	argoCDServerControllerDeployment.Spec.Template.Spec.Containers[0].Image = i.ServerImage
	argoCDServerControllerDeployment.Spec.Template.Spec.Containers[0].ImagePullPolicy = apiv1.PullPolicy(i.ImagePullPolicy)
	i.MustInstallResource(kube.MustToUnstructured(&argoCDServerServiceAccount))
	i.MustInstallResource(kube.MustToUnstructured(&argoCDServerControllerRole))
	i.MustInstallResource(kube.MustToUnstructured(&argoCDServerControllerRoleBinding))
	i.MustInstallResource(kube.MustToUnstructured(&argoCDServerControllerDeployment))
	i.MustInstallResource(kube.MustToUnstructured(&argoCDServerService))

}

func (i *Installer) InstallArgoCDRepoServer() {
	var argoCDRepoServerControllerDeployment appsv1beta2.Deployment
	var argoCDRepoServerService apiv1.Service
	i.unmarshalManifest("05a_argocd-repo-server-deployment.yaml", &argoCDRepoServerControllerDeployment)
	i.unmarshalManifest("05b_argocd-repo-server-service.yaml", &argoCDRepoServerService)
	argoCDRepoServerControllerDeployment.Spec.Template.Spec.Containers[0].Image = i.RepoServerImage
	argoCDRepoServerControllerDeployment.Spec.Template.Spec.Containers[0].ImagePullPolicy = apiv1.PullPolicy(i.ImagePullPolicy)
	i.MustInstallResource(kube.MustToUnstructured(&argoCDRepoServerControllerDeployment))
	i.MustInstallResource(kube.MustToUnstructured(&argoCDRepoServerService))
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

func (i *Installer) MustUninstallResource(obj *unstructured.Unstructured) {
	err := i.UninstallResource(obj)
	errors.CheckError(err)
}

// UninstallResource deletes a resource from the cluster
func (i *Installer) UninstallResource(obj *unstructured.Unstructured) error {
	if isNamespaced(obj) {
		obj.SetNamespace(i.Namespace)
	}
	gvk := obj.GroupVersionKind()
	dclient, err := i.dynClientPool.ClientForGroupVersionKind(gvk)
	if err != nil {
		return err
	}
	apiResource, err := kube.ServerResourceForGroupVersionKind(i.disco, gvk)
	if err != nil {
		return err
	}
	reIf := dclient.Resource(apiResource, i.Namespace)
	deletePolicy := metav1.DeletePropagationForeground
	err = reIf.Delete(obj.GetName(), &metav1.DeleteOptions{PropagationPolicy: &deletePolicy})
	if err != nil {
		if apierr.IsNotFound(err) {
			fmt.Printf("%s '%s' not found\n", obj.GetKind(), obj.GetName())
			return nil
		}
		return err
	}
	fmt.Printf("%s '%s' deleted\n", obj.GetKind(), obj.GetName())
	return nil
}

func printYAML(obj interface{}) {
	objBytes, err := yaml.Marshal(obj)
	if err != nil {
		log.Fatalf("Failed to marshal %v", obj)
	}
	fmt.Printf("---\n%s\n", string(objBytes))
}
