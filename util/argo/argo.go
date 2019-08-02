package argo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/argoproj/argo-cd/common"
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/pkg/client/clientset/versioned/typed/application/v1alpha1"
	applicationsv1 "github.com/argoproj/argo-cd/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver/apiclient"

	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/cert"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/ksonnet"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/kustomize"
)

const (
	errDestinationMissing = "Destination server and/or namespace missing from app spec"
)

// FormatAppConditions returns string representation of give app condition list
func FormatAppConditions(conditions []argoappv1.ApplicationCondition) string {
	formattedConditions := make([]string, 0)
	for _, condition := range conditions {
		formattedConditions = append(formattedConditions, fmt.Sprintf("%s: %s", condition.Type, condition.Message))
	}
	return strings.Join(formattedConditions, ";")
}

// FilterByProjects returns applications which belongs to the specified project
func FilterByProjects(apps []argoappv1.Application, projects []string) []argoappv1.Application {
	if len(projects) == 0 {
		return apps
	}
	projectsMap := make(map[string]bool)
	for i := range projects {
		projectsMap[projects[i]] = true
	}
	items := make([]argoappv1.Application, 0)
	for i := 0; i < len(apps); i++ {
		a := apps[i]
		if _, ok := projectsMap[a.Spec.GetProject()]; ok {
			items = append(items, a)
		}
	}
	return items

}

// RefreshApp updates the refresh annotation of an application to coerce the controller to process it
func RefreshApp(appIf v1alpha1.ApplicationInterface, name string, refreshType argoappv1.RefreshType) (*argoappv1.Application, error) {
	metadata := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]string{
				common.AnnotationKeyRefresh: string(refreshType),
			},
		},
	}
	var err error
	patch, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 5; attempt++ {
		app, err := appIf.Patch(name, types.MergePatchType, patch)
		if err != nil {
			if !apierr.IsConflict(err) {
				return nil, err
			}
		} else {
			log.Infof("Requested app '%s' refresh", name)
			return app, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, err
}

// WaitForRefresh watches an application until its comparison timestamp is after the refresh timestamp
// If refresh timestamp is not present, will use current timestamp at time of call
func WaitForRefresh(ctx context.Context, appIf v1alpha1.ApplicationInterface, name string, timeout *time.Duration) (*argoappv1.Application, error) {
	var cancel context.CancelFunc
	if timeout != nil {
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}
	ch := kube.WatchWithRetry(ctx, func() (i watch.Interface, e error) {
		fieldSelector := fields.ParseSelectorOrDie(fmt.Sprintf("metadata.name=%s", name))
		listOpts := metav1.ListOptions{FieldSelector: fieldSelector.String()}
		return appIf.Watch(listOpts)
	})
	for next := range ch {
		if next.Error != nil {
			return nil, next.Error
		}
		app, ok := next.Object.(*argoappv1.Application)
		if !ok {
			return nil, fmt.Errorf("Application event object failed conversion: %v", next)
		}
		annotations := app.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		if _, ok := annotations[common.AnnotationKeyRefresh]; !ok {
			return app, nil
		}
	}
	return nil, fmt.Errorf("application refresh deadline exceeded")
}

func getCAPath(repoURL string) string {
	if git.IsHTTPSURL(repoURL) {
		if parsedURL, err := url.Parse(repoURL); err == nil {
			if caPath, err := cert.GetCertBundlePathForRepository(parsedURL.Host); err != nil {
				return caPath
			} else {
				log.Warnf("Could not get cert bundle path for host '%s'", parsedURL.Host)
			}
		} else {
			// We don't fail if we cannot parse the URL, but log a warning in that
			// case. And we execute the command in a verbatim way.
			log.Warnf("Could not parse repo URL '%s'", repoURL)
		}
	}
	return ""
}

// TODO: Figure out the correct way to distinguish between SSH and HTTPS repos
func GetRepoCreds(repo *argoappv1.Repository) git.Creds {
	if repo == nil {
		return git.NopCreds{}
	}
	if (repo.Username != "" && repo.Password != "") || (repo.TLSClientCertData != "" && repo.TLSClientCertKey != "") {
		return git.NewHTTPSCreds(repo.Username, repo.Password, repo.TLSClientCertData, repo.TLSClientCertKey, repo.IsInsecure())
	}
	if repo.SSHPrivateKey != "" {
		return git.NewSSHCreds(repo.SSHPrivateKey, getCAPath(repo.Repo), repo.IsInsecure())
	}
	return git.NopCreds{}
}

// ValidateRepo validates the repository specified in application spec. Following is checked:
// * the git repository is accessible
// * the git path contains valid manifests
// * there are parameters of only one app source type
// * ksonnet: the specified environment exists
func ValidateRepo(ctx context.Context, spec *argoappv1.ApplicationSpec, repoClientset apiclient.Clientset, db db.ArgoDB, kustomizeOptions *argoappv1.KustomizeOptions) ([]argoappv1.ApplicationCondition, argoappv1.ApplicationSourceType, error) {
	conditions := make([]argoappv1.ApplicationCondition, 0)

	// Test the repo
	conn, repoClient, err := repoClientset.NewRepoServerClient()
	if err != nil {
		return nil, "", err
	}
	defer util.Close(conn)
	repoAccessable := false
	repoRes, err := db.GetRepository(ctx, spec.Source.RepoURL)
	if err != nil {
		return nil, "", err
	}

	err = git.TestRepo(repoRes.Repo, GetRepoCreds(repoRes), repoRes.IsInsecure(), repoRes.EnableLFS)
	if err != nil {
		conditions = append(conditions, argoappv1.ApplicationCondition{
			Type:    argoappv1.ApplicationConditionInvalidSpecError,
			Message: fmt.Sprintf("repository not accessible: %v", err),
		})
	} else {
		repoAccessable = true
	}

	var appSourceType argoappv1.ApplicationSourceType
	// Verify only one source type is defined
	explicitSourceType, err := spec.Source.ExplicitType()
	if err != nil {
		conditions = append(conditions, argoappv1.ApplicationCondition{
			Type:    argoappv1.ApplicationConditionInvalidSpecError,
			Message: fmt.Sprintf("Unable to determine app source type: %v", err),
		})
	}

	if repoAccessable {
		if explicitSourceType != nil {
			appSourceType = *explicitSourceType
		} else {
			appSourceType, err = queryAppSourceType(ctx, spec, repoRes, repoClient)
		}

		if err != nil {
			conditions = append(conditions, argoappv1.ApplicationCondition{
				Type:    argoappv1.ApplicationConditionInvalidSpecError,
				Message: fmt.Sprintf("Unable to determine app source type: %v", err),
			})
		} else {
			switch appSourceType {
			case argoappv1.ApplicationSourceTypeKsonnet:
				err := verifyAppYAML(ctx, repoRes, spec, repoClient)
				if err != nil {
					conditions = append(conditions, argoappv1.ApplicationCondition{
						Type:    argoappv1.ApplicationConditionInvalidSpecError,
						Message: err.Error(),
					})
				}
			case argoappv1.ApplicationSourceTypeHelm:
				helmConditions := verifyHelmChart(ctx, repoRes, spec, repoClient)
				if len(helmConditions) > 0 {
					conditions = append(conditions, helmConditions...)
				}
			case argoappv1.ApplicationSourceTypeDirectory, argoappv1.ApplicationSourceTypeKustomize:
				mainDirConditions := verifyGenerateManifests(ctx, repoRes, []*argoappv1.HelmRepository{}, spec, repoClient, kustomizeOptions)
				if len(mainDirConditions) > 0 {
					conditions = append(conditions, mainDirConditions...)
				}
			}
		}
	}
	return conditions, appSourceType, nil
}

// ValidatePermissions ensures that the referenced cluster has been added to Argo CD and the app source repo and destination namespace/cluster are permitted in app project
func ValidatePermissions(ctx context.Context, spec *argoappv1.ApplicationSpec, proj *argoappv1.AppProject, db db.ArgoDB) ([]argoappv1.ApplicationCondition, error) {
	conditions := make([]argoappv1.ApplicationCondition, 0)
	if spec.Source.RepoURL == "" || spec.Source.Path == "" {
		conditions = append(conditions, argoappv1.ApplicationCondition{
			Type:    argoappv1.ApplicationConditionInvalidSpecError,
			Message: "spec.source.repoURL and spec.source.path are required",
		})
		return conditions, nil
	}

	if !proj.IsSourcePermitted(spec.Source) {
		conditions = append(conditions, argoappv1.ApplicationCondition{
			Type:    argoappv1.ApplicationConditionInvalidSpecError,
			Message: fmt.Sprintf("application repo %s is not permitted in project '%s'", spec.Source.RepoURL, spec.Project),
		})
	}

	if spec.Destination.Server != "" && spec.Destination.Namespace != "" {
		if !proj.IsDestinationPermitted(spec.Destination) {
			conditions = append(conditions, argoappv1.ApplicationCondition{
				Type:    argoappv1.ApplicationConditionInvalidSpecError,
				Message: fmt.Sprintf("application destination %v is not permitted in project '%s'", spec.Destination, spec.Project),
			})
		}
		// Ensure the k8s cluster the app is referencing, is configured in Argo CD
		_, err := db.GetCluster(ctx, spec.Destination.Server)
		if err != nil {
			if errStatus, ok := status.FromError(err); ok && errStatus.Code() == codes.NotFound {
				conditions = append(conditions, argoappv1.ApplicationCondition{
					Type:    argoappv1.ApplicationConditionInvalidSpecError,
					Message: fmt.Sprintf("cluster '%s' has not been configured", spec.Destination.Server),
				})
			} else {
				return nil, err
			}
		}
	} else {
		conditions = append(conditions, argoappv1.ApplicationCondition{Type: argoappv1.ApplicationConditionInvalidSpecError, Message: errDestinationMissing})
	}
	return conditions, nil
}

// GetAppProject returns a project from an application
func GetAppProject(spec *argoappv1.ApplicationSpec, projLister applicationsv1.AppProjectLister, ns string) (*argoappv1.AppProject, error) {
	return projLister.AppProjects(ns).Get(spec.GetProject())
}

func queryAppSourceType(ctx context.Context, spec *argoappv1.ApplicationSpec, repoRes *argoappv1.Repository, repoClient apiclient.RepoServerServiceClient) (argoappv1.ApplicationSourceType, error) {
	req := apiclient.ListDirRequest{
		Repo: &argoappv1.Repository{
			Repo: spec.Source.RepoURL,
		},
		Revision: spec.Source.TargetRevision,
		Path:     fmt.Sprintf("%s/*.yaml", spec.Source.Path),
	}
	req.Repo.CopyCredentialsFrom(repoRes)
	getRes, err := repoClient.ListDir(ctx, &req)
	if err != nil {
		return "", err
	}
	for _, gitPath := range getRes.Items {
		// gitPath may look like: app.yaml, or some/subpath/app.yaml
		trimmedPath := strings.TrimPrefix(gitPath, filepath.Clean(spec.Source.Path))
		trimmedPath = strings.TrimPrefix(trimmedPath, "/")
		if trimmedPath == "app.yaml" {
			return argoappv1.ApplicationSourceTypeKsonnet, nil
		}
		if trimmedPath == "Chart.yaml" {
			return argoappv1.ApplicationSourceTypeHelm, nil
		}
		if kustomize.IsKustomization(trimmedPath) {
			return argoappv1.ApplicationSourceTypeKustomize, nil
		}
	}
	return argoappv1.ApplicationSourceTypeDirectory, nil
}

// verifyAppYAML verifies that a ksonnet app.yaml is functional
func verifyAppYAML(ctx context.Context, repoRes *argoappv1.Repository, spec *argoappv1.ApplicationSpec, repoClient apiclient.RepoServerServiceClient) error {
	// Default revision to HEAD if unspecified
	if spec.Source.TargetRevision == "" {
		spec.Source.TargetRevision = "HEAD"
	}

	req := apiclient.GetFileRequest{
		Repo: &argoappv1.Repository{
			Repo: spec.Source.RepoURL,
		},
		Revision: spec.Source.TargetRevision,
		Path:     path.Join(spec.Source.Path, "app.yaml"),
	}
	req.Repo.CopyCredentialsFrom(repoRes)
	getRes, err := repoClient.GetFile(ctx, &req)
	if err != nil {
		return fmt.Errorf("Unable to load app.yaml: %v", err)
	}

	// Verify the specified environment is defined in the app spec
	if spec.Source.Ksonnet == nil {
		return fmt.Errorf("Ksonnet environment not specified")
	}

	dest, err := ksonnet.Destination(getRes.Data, spec.Source.Ksonnet.Environment)
	if err != nil {
		return err
	}

	// If server and namespace are not supplied, pull it from the app.yaml
	if spec.Destination.Server == "" {
		spec.Destination.Server = dest.Server
	}
	if spec.Destination.Namespace == "" {
		spec.Destination.Namespace = dest.Namespace
	}

	return nil
}

// verifyHelmChart verifies a helm chart is functional
// verifyHelmChart verifies a helm chart is functional
func verifyHelmChart(ctx context.Context, repoRes *argoappv1.Repository, spec *argoappv1.ApplicationSpec, repoClient apiclient.RepoServerServiceClient) []argoappv1.ApplicationCondition {
	var conditions []argoappv1.ApplicationCondition
	if spec.Destination.Server == "" || spec.Destination.Namespace == "" {
		conditions = append(conditions, argoappv1.ApplicationCondition{
			Type:    argoappv1.ApplicationConditionInvalidSpecError,
			Message: errDestinationMissing,
		})
	}
	req := apiclient.GetFileRequest{
		Repo: &argoappv1.Repository{
			Repo: spec.Source.RepoURL,
		},
		Revision: spec.Source.TargetRevision,
		Path:     path.Join(spec.Source.Path, "Chart.yaml"),
	}
	req.Repo.CopyCredentialsFrom(repoRes)
	_, err := repoClient.GetFile(ctx, &req)
	if err != nil {
		conditions = append(conditions, argoappv1.ApplicationCondition{
			Type:    argoappv1.ApplicationConditionInvalidSpecError,
			Message: fmt.Sprintf("Unable to load Chart.yaml: %v", err),
		})
	}
	return conditions
}

// verifyGenerateManifests verifies a repo path can generate manifests
func verifyGenerateManifests(
	ctx context.Context, repoRes *argoappv1.Repository, helmRepos []*argoappv1.HelmRepository, spec *argoappv1.ApplicationSpec, repoClient apiclient.RepoServerServiceClient, kustomizeOptions *argoappv1.KustomizeOptions) []argoappv1.ApplicationCondition {

	var conditions []argoappv1.ApplicationCondition
	if spec.Destination.Server == "" || spec.Destination.Namespace == "" {
		conditions = append(conditions, argoappv1.ApplicationCondition{
			Type:    argoappv1.ApplicationConditionInvalidSpecError,
			Message: errDestinationMissing,
		})
	}
	req := apiclient.ManifestRequest{
		Repo: &argoappv1.Repository{
			Repo: spec.Source.RepoURL,
		},
		HelmRepos:         helmRepos,
		Revision:          spec.Source.TargetRevision,
		Namespace:         spec.Destination.Namespace,
		ApplicationSource: &spec.Source,
		KustomizeOptions:  kustomizeOptions,
	}
	req.Repo.CopyCredentialsFrom(repoRes)

	// Only check whether we can access the application's path,
	// and not whether it actually contains any manifests.
	_, err := repoClient.GenerateManifest(ctx, &req)
	if err != nil {
		conditions = append(conditions, argoappv1.ApplicationCondition{
			Type:    argoappv1.ApplicationConditionInvalidSpecError,
			Message: fmt.Sprintf("Unable to generate manifests in %s: %v", spec.Source.Path, err),
		})
	}

	return conditions
}

// SetAppOperation updates an application with the specified operation, retrying conflict errors
func SetAppOperation(appIf v1alpha1.ApplicationInterface, appName string, op *argoappv1.Operation) (*argoappv1.Application, error) {
	for {
		a, err := appIf.Get(appName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if a.Operation != nil {
			return nil, status.Errorf(codes.FailedPrecondition, "another operation is already in progress")
		}
		a.Operation = op
		a.Status.OperationState = nil
		a, err = appIf.Update(a)
		if op.Sync == nil {
			return nil, status.Errorf(codes.InvalidArgument, "Operation unspecified")
		}
		if err == nil {
			return a, nil
		}
		if !apierr.IsConflict(err) {
			return nil, err
		}
		log.Warnf("Failed to set operation for app '%s' due to update conflict. Retrying again...", appName)
	}
}

// ContainsSyncResource determines if the given resource exists in the provided slice of sync operation resources.
func ContainsSyncResource(name string, gvk schema.GroupVersionKind, rr []argoappv1.SyncOperationResource) bool {
	for _, r := range rr {
		if r.HasIdentity(name, gvk) {
			return true
		}
	}
	return false
}

// NormalizeApplicationSpec will normalize an application spec to a preferred state. This is used
// for migrating application objects which are using deprecated legacy fields into the new fields,
// and defaulting fields in the spec (e.g. spec.project)
func NormalizeApplicationSpec(spec *argoappv1.ApplicationSpec, sourceType argoappv1.ApplicationSourceType) *argoappv1.ApplicationSpec {
	spec = spec.DeepCopy()
	if spec.Project == "" {
		spec.Project = common.DefaultAppProjectName
	}

	// 3. If any app sources are their zero values, then nil out the pointers to the source spec.
	// This makes it easier for users to switch between app source types if they are not using
	// any of the source-specific parameters.
	if spec.Source.Kustomize != nil && spec.Source.Kustomize.IsZero() {
		spec.Source.Kustomize = nil
	}
	if spec.Source.Helm != nil && spec.Source.Helm.IsZero() {
		spec.Source.Helm = nil
	}
	if spec.Source.Ksonnet != nil && spec.Source.Ksonnet.IsZero() {
		spec.Source.Ksonnet = nil
	}
	if spec.Source.Directory != nil && spec.Source.Directory.IsZero() {
		spec.Source.Directory = nil
	}
	return spec
}
