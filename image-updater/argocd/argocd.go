package argocd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/image-updater/image"
	"github.com/argoproj/argo-cd/v2/image-updater/kube"
	"github.com/argoproj/argo-cd/v2/image-updater/log"
	"github.com/argoproj/argo-cd/v2/image-updater/metrics"

	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Kubernetes based client
type k8sClient struct {
	kubeClient *kube.KubernetesClient
}

func (client *k8sClient) GetApplication(ctx context.Context, appName string) (*v1alpha1.Application, error) {
	return client.kubeClient.ApplicationsClientset.ArgoprojV1alpha1().Applications(client.kubeClient.Namespace).Get(ctx, appName, v1.GetOptions{})
}

func (client *k8sClient) ListApplications() ([]v1alpha1.Application, error) {
	list, err := client.kubeClient.ApplicationsClientset.ArgoprojV1alpha1().Applications(client.kubeClient.Namespace).List(context.TODO(), v1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (client *k8sClient) UpdateSpec(ctx context.Context, spec *application.ApplicationUpdateSpecRequest) (*v1alpha1.ApplicationSpec, error) {
	for {
		app, err := client.kubeClient.ApplicationsClientset.ArgoprojV1alpha1().Applications(client.kubeClient.Namespace).Get(ctx, spec.GetName(), v1.GetOptions{})
		if err != nil {
			return nil, err
		}
		app.Spec = *spec.Spec

		updatedApp, err := client.kubeClient.ApplicationsClientset.ArgoprojV1alpha1().Applications(client.kubeClient.Namespace).Update(ctx, app, v1.UpdateOptions{})
		if err != nil {
			if errors.IsConflict(err) {
				continue
			}
			return nil, err
		}
		return &updatedApp.Spec, nil
	}

}

// NewAPIClient creates a new API client for ArgoCD and connects to the ArgoCD
// API server.
func NewK8SClient(kubeClient *kube.KubernetesClient) (ArgoCD, error) {
	return &k8sClient{kubeClient: kubeClient}, nil
}

// Native
type argoCD struct {
	Client argocdclient.Client
}

// ArgoCD is the interface for accessing Argo CD functions we need
type ArgoCD interface {
	GetApplication(ctx context.Context, appName string) (*v1alpha1.Application, error)
	ListApplications() ([]v1alpha1.Application, error)
	UpdateSpec(ctx context.Context, spec *application.ApplicationUpdateSpecRequest) (*v1alpha1.ApplicationSpec, error)
}

// Type of the application
type ApplicationType int

const (
	ApplicationTypeUnsupported ApplicationType = 0
	ApplicationTypeHelm        ApplicationType = 1
	ApplicationTypeKustomize   ApplicationType = 2
)

// Basic wrapper struct for ArgoCD client options
type ClientOptions struct {
	ServerAddr      string
	Insecure        bool
	Plaintext       bool
	Certfile        string
	GRPCWeb         bool
	GRPCWebRootPath string
	AuthToken       string
}

// NewAPIClient creates a new API client for ArgoCD and connects to the ArgoCD
// API server.
func NewAPIClient(opts *ClientOptions) (ArgoCD, error) {

	envAuthToken := os.Getenv("ARGOCD_TOKEN")
	if envAuthToken != "" && opts.AuthToken == "" {
		opts.AuthToken = envAuthToken
	}

	rOpts := argocdclient.ClientOptions{
		ServerAddr:      opts.ServerAddr,
		PlainText:       opts.Plaintext,
		Insecure:        opts.Insecure,
		CertFile:        opts.Certfile,
		GRPCWeb:         opts.GRPCWeb,
		GRPCWebRootPath: opts.GRPCWebRootPath,
		AuthToken:       opts.AuthToken,
	}
	client, err := argocdclient.NewClient(&rOpts)
	if err != nil {
		return nil, err
	}
	return &argoCD{Client: client}, nil
}

type ApplicationImages struct {
	Application v1alpha1.Application
	Images      image.ContainerImageList
}

// Will hold a list of applications with the images allowed to considered for
// update.
type ImageList map[string]ApplicationImages

// Match a name against a list of patterns
func nameMatchesPattern(name string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, p := range patterns {
		log.Tracef("Matching application name %s against pattern %s", name, p)
		if m, err := filepath.Match(p, name); err != nil {
			log.Warnf("Invalid application name pattern '%s': %v", p, err)
		} else if m {
			return true
		}
	}
	return false
}

// Match app labels against provided filter label
func matchAppLabels(appName string, appLabels map[string]string, filterLabel string) bool {

	if filterLabel == "" {
		return true
	}

	filterLabelMap, err := parseLabel(filterLabel)
	if err != nil {
		log.Errorf("Unable match app labels against %s: %s", filterLabel, err)
		return false
	}

	for filterLabelKey, filterLabelValue := range filterLabelMap {
		log.Tracef("Matching application name %s against label %s", appName, filterLabel)
		if appLabelValue, ok := appLabels[filterLabelKey]; ok {
			if appLabelValue == filterLabelValue {
				return true
			}
		}
	}
	return false
}

// Retrieve a list of applications from ArgoCD that qualify for image updates
// Application needs either to be of type Kustomize or Helm and must have the
// correct annotation in order to be considered.
func FilterApplicationsForUpdate(apps []v1alpha1.Application, patterns []string, appLabel string) (map[string]ApplicationImages, error) {
	var appsForUpdate = make(map[string]ApplicationImages)

	for _, app := range apps {
		logCtx := log.WithContext().AddField("application", app.GetName())
		// Check whether application has our annotation set
		annotations := app.GetAnnotations()
		if _, ok := annotations[common.ImageUpdaterAnnotation]; !ok {
			logCtx.Tracef("skipping app '%s' of type '%s' because required annotation is missing", app.GetName(), app.Status.SourceType)
			continue
		}

		// Check for valid application type
		if !IsValidApplicationType(&app) {
			logCtx.Warnf("skipping app '%s' of type '%s' because it's not of supported source type", app.GetName(), app.Status.SourceType)
			continue
		}

		// Check if application name matches requested patterns
		if !nameMatchesPattern(app.GetName(), patterns) {
			logCtx.Debugf("Skipping app '%s' because it does not match requested patterns", app.GetName())
			continue
		}

		// Check if application carries requested label
		if !matchAppLabels(app.GetName(), app.GetLabels(), appLabel) {
			logCtx.Debugf("Skipping app '%s' because it does not carry requested label", app.GetName())
			continue
		}

		logCtx.Tracef("processing app '%s' of type '%v'", app.GetName(), app.Status.SourceType)
		imageList := parseImageList(annotations)
		appImages := ApplicationImages{}
		appImages.Application = app
		appImages.Images = *imageList
		appsForUpdate[app.GetName()] = appImages
	}

	return appsForUpdate, nil
}

func parseImageList(annotations map[string]string) *image.ContainerImageList {
	results := make(image.ContainerImageList, 0)
	if updateImage, ok := annotations[common.ImageUpdaterAnnotation]; ok {
		splits := strings.Split(updateImage, ",")
		for _, s := range splits {
			img := image.NewFromIdentifier(strings.TrimSpace(s))
			if kustomizeImage := img.GetParameterKustomizeImageName(annotations); kustomizeImage != "" {
				img.KustomizeImage = image.NewFromIdentifier(kustomizeImage)
			}
			results = append(results, img)
		}
	}
	return &results
}

func parseLabel(inputLabel string) (map[string]string, error) {
	var selectedLabels map[string]string
	const labelFieldDelimiter = "="
	if inputLabel != "" {
		selectedLabels = map[string]string{}
		fields := strings.Split(inputLabel, labelFieldDelimiter)
		if len(fields) != 2 {
			return nil, fmt.Errorf("labels should have key%svalue, but instead got: %s", labelFieldDelimiter, inputLabel)
		}
		selectedLabels[fields[0]] = fields[1]
	}
	return selectedLabels, nil
}

// GetApplication gets the application named appName from Argo CD API
func (client *argoCD) GetApplication(ctx context.Context, appName string) (*v1alpha1.Application, error) {
	conn, appClient, err := client.Client.NewApplicationClient()
	metrics.Clients().IncreaseArgoCDClientRequest(client.Client.ClientOptions().ServerAddr, 1)
	if err != nil {
		metrics.Clients().IncreaseArgoCDClientError(client.Client.ClientOptions().ServerAddr, 1)
		return nil, err
	}
	defer conn.Close()

	metrics.Clients().IncreaseArgoCDClientRequest(client.Client.ClientOptions().ServerAddr, 1)
	app, err := appClient.Get(ctx, &application.ApplicationQuery{Name: &appName})
	if err != nil {
		metrics.Clients().IncreaseArgoCDClientError(client.Client.ClientOptions().ServerAddr, 1)
		return nil, err
	}

	return app, nil
}

// ListApplications returns a list of all application names that the API user
// has access to.
func (client *argoCD) ListApplications() ([]v1alpha1.Application, error) {
	conn, appClient, err := client.Client.NewApplicationClient()
	metrics.Clients().IncreaseArgoCDClientRequest(client.Client.ClientOptions().ServerAddr, 1)
	if err != nil {
		metrics.Clients().IncreaseArgoCDClientError(client.Client.ClientOptions().ServerAddr, 1)
		return nil, err
	}
	defer conn.Close()

	metrics.Clients().IncreaseArgoCDClientRequest(client.Client.ClientOptions().ServerAddr, 1)
	apps, err := appClient.List(context.TODO(), &application.ApplicationQuery{})
	if err != nil {
		metrics.Clients().IncreaseArgoCDClientError(client.Client.ClientOptions().ServerAddr, 1)
		return nil, err
	}

	return apps.Items, nil
}

// UpdateSpec updates the spec for given application
func (client *argoCD) UpdateSpec(ctx context.Context, in *application.ApplicationUpdateSpecRequest) (*v1alpha1.ApplicationSpec, error) {
	conn, appClient, err := client.Client.NewApplicationClient()
	metrics.Clients().IncreaseArgoCDClientRequest(client.Client.ClientOptions().ServerAddr, 1)
	if err != nil {
		metrics.Clients().IncreaseArgoCDClientError(client.Client.ClientOptions().ServerAddr, 1)
		return nil, err
	}
	defer conn.Close()

	metrics.Clients().IncreaseArgoCDClientRequest(client.Client.ClientOptions().ServerAddr, 1)
	spec, err := appClient.UpdateSpec(ctx, in)
	if err != nil {
		metrics.Clients().IncreaseArgoCDClientError(client.Client.ClientOptions().ServerAddr, 1)
		return nil, err
	}

	return spec, nil
}

// getHelmParamNamesFromAnnotation inspects the given annotations for whether
// the annotations for specifying Helm parameter names are being set and
// returns their values.
func getHelmParamNamesFromAnnotation(annotations map[string]string, symbolicName string) (string, string) {
	// Return default values without symbolic name given
	if symbolicName == "" {
		return "image.name", "image.tag"
	}

	var annotationName, helmParamName, helmParamVersion string

	// Image spec is a full-qualified specifier, if we have it, we return early
	annotationName = fmt.Sprintf(common.HelmParamImageSpecAnnotation, symbolicName)
	if param, ok := annotations[annotationName]; ok {
		log.Tracef("found annotation %s", annotationName)
		return strings.TrimSpace(param), ""
	}

	annotationName = fmt.Sprintf(common.HelmParamImageNameAnnotation, symbolicName)
	if param, ok := annotations[annotationName]; ok {
		log.Tracef("found annotation %s", annotationName)
		helmParamName = param
	}

	annotationName = fmt.Sprintf(common.HelmParamImageTagAnnotation, symbolicName)
	if param, ok := annotations[annotationName]; ok {
		log.Tracef("found annotation %s", annotationName)
		helmParamVersion = param
	}

	return helmParamName, helmParamVersion
}

// Get a named helm parameter from a list of parameters
func getHelmParam(params []v1alpha1.HelmParameter, name string) *v1alpha1.HelmParameter {
	for _, param := range params {
		if param.Name == name {
			return &param
		}
	}
	return nil
}

// mergeHelmParams merges a list of Helm parameters specified by merge into the
// Helm parameters given as src.
func mergeHelmParams(src []v1alpha1.HelmParameter, merge []v1alpha1.HelmParameter) []v1alpha1.HelmParameter {
	retParams := make([]v1alpha1.HelmParameter, 0)
	merged := make(map[string]interface{})

	// first look for params that need replacement
	for _, srcParam := range src {
		found := false
		for _, mergeParam := range merge {
			if srcParam.Name == mergeParam.Name {
				retParams = append(retParams, mergeParam)
				merged[mergeParam.Name] = true
				found = true
				break
			}
		}
		if !found {
			retParams = append(retParams, srcParam)
		}
	}

	// then check which we still need in dest list and merge those, too
	for _, mergeParam := range merge {
		if _, ok := merged[mergeParam.Name]; !ok {
			retParams = append(retParams, mergeParam)
		}
	}

	return retParams
}

// SetHelmImage sets image parameters for a Helm application
func SetHelmImage(app *v1alpha1.Application, newImage *image.ContainerImage) error {
	if appType := getApplicationType(app); appType != ApplicationTypeHelm {
		return fmt.Errorf("cannot set Helm params on non-Helm application")
	}

	appName := app.GetName()

	var hpImageName, hpImageTag, hpImageSpec string

	hpImageSpec = newImage.GetParameterHelmImageSpec(app.Annotations)
	hpImageName = newImage.GetParameterHelmImageName(app.Annotations)
	hpImageTag = newImage.GetParameterHelmImageTag(app.Annotations)

	if hpImageSpec == "" {
		if hpImageName == "" {
			hpImageName = common.DefaultHelmImageName
		}
		if hpImageTag == "" {
			hpImageTag = common.DefaultHelmImageTag
		}
	}

	log.WithContext().
		AddField("application", appName).
		AddField("image", newImage.GetFullNameWithoutTag()).
		Debugf("target parameters: image-spec=%s image-name=%s, image-tag=%s", hpImageSpec, hpImageName, hpImageTag)

	mergeParams := make([]v1alpha1.HelmParameter, 0)

	// The logic behind this is that image-spec is an override - if this is set,
	// we simply ignore any image-name and image-tag parameters that might be
	// there.
	if hpImageSpec != "" {
		p := v1alpha1.HelmParameter{Name: hpImageSpec, Value: newImage.GetFullNameWithTag(), ForceString: true}
		mergeParams = append(mergeParams, p)
	} else {
		if hpImageName != "" {
			p := v1alpha1.HelmParameter{Name: hpImageName, Value: newImage.GetFullNameWithoutTag(), ForceString: true}
			mergeParams = append(mergeParams, p)
		}
		if hpImageTag != "" {
			p := v1alpha1.HelmParameter{Name: hpImageTag, Value: newImage.GetTagWithDigest(), ForceString: true}
			mergeParams = append(mergeParams, p)
		}
	}

	if app.Spec.Source.Helm == nil {
		app.Spec.Source.Helm = &v1alpha1.ApplicationSourceHelm{}
	}

	if app.Spec.Source.Helm.Parameters == nil {
		app.Spec.Source.Helm.Parameters = make([]v1alpha1.HelmParameter, 0)
	}

	app.Spec.Source.Helm.Parameters = mergeHelmParams(app.Spec.Source.Helm.Parameters, mergeParams)

	return nil
}

// SetKustomizeImage sets a Kustomize image for given application
func SetKustomizeImage(app *v1alpha1.Application, newImage *image.ContainerImage) error {
	if appType := getApplicationType(app); appType != ApplicationTypeKustomize {
		return fmt.Errorf("cannot set Kustomize image on non-Kustomize application")
	}

	var ksImageParam string
	ksImageName := newImage.GetParameterKustomizeImageName(app.Annotations)
	if ksImageName != "" {
		ksImageParam = fmt.Sprintf("%s=%s", ksImageName, newImage.GetFullNameWithTag())
	} else {
		ksImageParam = newImage.GetFullNameWithTag()
	}

	log.WithContext().AddField("application", app.GetName()).Tracef("Setting Kustomize parameter %s", ksImageParam)

	if app.Spec.Source.Kustomize == nil {
		app.Spec.Source.Kustomize = &v1alpha1.ApplicationSourceKustomize{}
	}

	for i, kImg := range app.Spec.Source.Kustomize.Images {
		curr := image.NewFromIdentifier(string(kImg))
		override := image.NewFromIdentifier(ksImageParam)

		if curr.ImageName == override.ImageName {
			curr.ImageAlias = override.ImageAlias
			app.Spec.Source.Kustomize.Images[i] = v1alpha1.KustomizeImage(override.String())
		}

	}

	app.Spec.Source.Kustomize.MergeImage(v1alpha1.KustomizeImage(ksImageParam))

	return nil
}

// GetImagesFromApplication returns the list of known images for the given application
func GetImagesFromApplication(app *v1alpha1.Application) image.ContainerImageList {
	images := make(image.ContainerImageList, 0)

	for _, imageStr := range app.Status.Summary.Images {
		image := image.NewFromIdentifier(imageStr)
		images = append(images, image)
	}

	// The Application may wish to update images that don't create a container we can detect.
	// Check the image list for images with a force-update annotation, and add them if they are not already present.
	annotations := app.Annotations
	for _, img := range *parseImageList(annotations) {
		if img.HasForceUpdateOptionAnnotation(annotations) {
			img.ImageTag = nil // the tag from the image list will be a version constraint, which isn't a valid tag
			images = append(images, img)
		}
	}

	return images
}

// GetApplicationTypeByName first retrieves application with given appName and
// returns its application type
func GetApplicationTypeByName(client ArgoCD, appName string) (ApplicationType, error) {
	app, err := client.GetApplication(context.TODO(), appName)
	if err != nil {
		return ApplicationTypeUnsupported, err
	}
	return getApplicationType(app), nil
}

// GetApplicationType returns the type of the ArgoCD application
func GetApplicationType(app *v1alpha1.Application) ApplicationType {
	return getApplicationType(app)
}

// IsValidApplicationType returns true if we can update the application
func IsValidApplicationType(app *v1alpha1.Application) bool {
	return getApplicationType(app) != ApplicationTypeUnsupported
}

// getApplicationType returns the type of the application
func getApplicationType(app *v1alpha1.Application) ApplicationType {
	sourceType := app.Status.SourceType
	if st, set := app.Annotations[common.WriteBackTargetAnnotation]; set &&
		strings.HasPrefix(st, common.KustomizationPrefix) {
		sourceType = v1alpha1.ApplicationSourceTypeKustomize
	}
	if sourceType == v1alpha1.ApplicationSourceTypeKustomize {
		return ApplicationTypeKustomize
	} else if sourceType == v1alpha1.ApplicationSourceTypeHelm {
		return ApplicationTypeHelm
	} else {
		return ApplicationTypeUnsupported
	}
}

// String returns a string representation of the application type
func (a ApplicationType) String() string {
	switch a {
	case ApplicationTypeKustomize:
		return "Kustomize"
	case ApplicationTypeHelm:
		return "Helm"
	case ApplicationTypeUnsupported:
		return "Unsupported"
	default:
		return "Unknown"
	}
}
