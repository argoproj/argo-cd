package argo

import (
	"context"
	"encoding/json"
	"fmt"
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
	"github.com/argoproj/argo-cd/reposerver"
	"github.com/argoproj/argo-cd/reposerver/repository"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/repos"
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

// GetSpecErrors returns list of conditions which indicates that app spec is invalid. Following is checked:
// * the repository is accessible
// * the git path contains valid manifests
// * the referenced cluster has been added to Argo CD
// * the app source repo and destination namespace/cluster are permitted in app project
// * there are parameters of only one app source type
// * ksonnet: the specified environment exists
func GetSpecErrors(
	ctx context.Context,
	spec *argoappv1.ApplicationSpec,
	proj *argoappv1.AppProject,
	repoClientset reposerver.Clientset,
	db db.ArgoDB,
) ([]argoappv1.ApplicationCondition, argoappv1.ApplicationSourceType, error) {
	conditions := make([]argoappv1.ApplicationCondition, 0)
	if spec.Source.RepoURL == "" || spec.Source.Path == "" {
		conditions = append(conditions, argoappv1.ApplicationCondition{
			Type:    argoappv1.ApplicationConditionInvalidSpecError,
			Message: "spec.source.repoURL and spec.source.path are required",
		})
		return conditions, "", nil
	}

	// Test the repo
	conn, repoClient, err := repoClientset.NewRepoServerClient()
	if err != nil {
		return nil, "", err
	}
	defer util.Close(conn)
	repoAccessible := false
	repoRes, err := db.GetRepository(ctx, spec.Source.RepoURL)

	repoType := "git"
	if repoRes != nil && repoRes.Type != "" {
		repoType = repoRes.Type
	}
	factory := repos.GetFactory(repoType)

	if err != nil {
		if errStatus, ok := status.FromError(err); ok && errStatus.Code() == codes.NotFound {
			// The repo has not been added to Argo CD so we do not have credentials to access it.
			// We support the mode where apps can be created from public repositories. Test the
			// repo to make sure it is publicly accessible
			switch f := factory.(type) {
			case git.RepoCfgFactory:
				_, err = f.GetRepoCfg(spec.Source.RepoURL, "", "", "", false)
			}

			if err != nil {
				conditions = append(conditions, argoappv1.ApplicationCondition{
					Type:    argoappv1.ApplicationConditionInvalidSpecError,
					Message: fmt.Sprintf("No credentials available for source repository and repository is not publicly accessible: %v", err),
				})
			} else {
				repoAccessible = true
			}
		} else {
			return nil, "", err
		}
	} else {
		repoAccessible = true
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

	if repoAccessible {
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
			maniDirConditions := verifyGenerateManifests(ctx, repoRes, []*argoappv1.Repository{}, spec, repoClient)
			if len(maniDirConditions) > 0 {
				conditions = append(conditions, maniDirConditions...)
			}
		}
	}

	if !proj.IsSourcePermitted(spec.Source, factory.NormalizeURL) {
		conditions = append(conditions, argoappv1.ApplicationCondition{
			Type:    argoappv1.ApplicationConditionInvalidSpecError,
			Message: fmt.Sprintf("application source %v is not permitted in project '%s'", spec.Source, spec.Project),
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
		_, err = db.GetCluster(ctx, spec.Destination.Server)
		if err != nil {
			if errStatus, ok := status.FromError(err); ok && errStatus.Code() == codes.NotFound {
				conditions = append(conditions, argoappv1.ApplicationCondition{
					Type:    argoappv1.ApplicationConditionInvalidSpecError,
					Message: fmt.Sprintf("cluster '%s' has not been configured", spec.Destination.Server),
				})
			} else {
				return nil, "", err
			}
		}
	}
	return conditions, appSourceType, nil
}

// GetAppProject returns a project from an application
func GetAppProject(spec *argoappv1.ApplicationSpec, projLister applicationsv1.AppProjectLister, ns string) (*argoappv1.AppProject, error) {
	return projLister.AppProjects(ns).Get(spec.GetProject())
}

func queryAppSourceType(ctx context.Context, spec *argoappv1.ApplicationSpec, repoRes *argoappv1.Repository, repoClient repository.RepoServerServiceClient) (argoappv1.ApplicationSourceType, error) {
	req := repository.GetAppRequest{
		Repo: &argoappv1.Repository{
			Repo: spec.Source.RepoURL,
		},
		Revision: spec.Source.TargetRevision,
		Path:     spec.Source.Path,
	}
	if repoRes != nil {
		req.Repo.Type = repoRes.Type
		req.Repo.Name = repoRes.Name
		req.Repo.Username = repoRes.Username
		req.Repo.Password = repoRes.Password
		req.Repo.SSHPrivateKey = repoRes.SSHPrivateKey
		req.Repo.CAData = repoRes.CAData
		req.Repo.CertData = repoRes.CertData
		req.Repo.KeyData = repoRes.KeyData
	}
	getRes, err := repoClient.GetApp(ctx, &req)
	if err != nil {
		return "", err
	}

	switch getRes.Tool {
	case "ksonnet":
		return argoappv1.ApplicationSourceTypeKsonnet, nil
	case "helm":
		return argoappv1.ApplicationSourceTypeHelm, nil
	case "kustomize":
		return argoappv1.ApplicationSourceTypeKustomize, nil
	}
	return argoappv1.ApplicationSourceTypeDirectory, nil
}

// verifyGenerateManifests verifies a repo path can generate manifests
func verifyGenerateManifests(
	ctx context.Context, repoRes *argoappv1.Repository, repos []*argoappv1.Repository, spec *argoappv1.ApplicationSpec, repoClient repository.RepoServerServiceClient) []argoappv1.ApplicationCondition {

	var conditions []argoappv1.ApplicationCondition
	if spec.Destination.Server == "" || spec.Destination.Namespace == "" {
		conditions = append(conditions, argoappv1.ApplicationCondition{
			Type:    argoappv1.ApplicationConditionInvalidSpecError,
			Message: errDestinationMissing,
		})
	}
	req := repository.ManifestRequest{
		Repo: &argoappv1.Repository{
			Repo: spec.Source.RepoURL,
		},
		Repos:             repos,
		Revision:          spec.Source.TargetRevision,
		Namespace:         spec.Destination.Namespace,
		ApplicationSource: &spec.Source,
	}
	if repoRes != nil {
		req.Repo.Type = repoRes.Type
		req.Repo.Name = repoRes.Name
		req.Repo.Username = repoRes.Username
		req.Repo.Password = repoRes.Password
		req.Repo.SSHPrivateKey = repoRes.SSHPrivateKey
		req.Repo.CAData = repoRes.CAData
		req.Repo.CertData = repoRes.CertData
		req.Repo.KeyData = repoRes.KeyData
	}

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
	// 1. carry over legacy componentParameterOverride field (v0.11 and below) into
	// ksonnet, helm, kustomize specific fields. Only do this if source specific config is empty
	// since this is a one-time conversion.
	if len(spec.Source.ComponentParameterOverrides) > 0 {
		switch sourceType {
		case argoappv1.ApplicationSourceTypeKsonnet:
			if spec.Source.Ksonnet == nil {
				spec.Source.Ksonnet = &argoappv1.ApplicationSourceKsonnet{}
			}
			if len(spec.Source.Ksonnet.Parameters) == 0 {
				for _, p := range spec.Source.ComponentParameterOverrides {
					spec.Source.Ksonnet.Parameters = append(spec.Source.Ksonnet.Parameters, argoappv1.KsonnetParameter(p))

				}
			}
		case argoappv1.ApplicationSourceTypeHelm:
			if spec.Source.Helm == nil {
				spec.Source.Helm = &argoappv1.ApplicationSourceHelm{}
			}
			if len(spec.Source.Helm.Parameters) == 0 {
				for _, p := range spec.Source.ComponentParameterOverrides {
					spec.Source.Helm.Parameters = append(spec.Source.Helm.Parameters, argoappv1.HelmParameter{
						Name:  p.Name,
						Value: p.Value,
					})
				}
			}
		case argoappv1.ApplicationSourceTypeKustomize:
			if spec.Source.Kustomize == nil {
				spec.Source.Kustomize = &argoappv1.ApplicationSourceKustomize{}
			}
			if len(spec.Source.Kustomize.ImageTags) == 0 {
				for _, p := range spec.Source.ComponentParameterOverrides {
					if p.Component != "imagetag" {
						continue
					}
					spec.Source.Kustomize.ImageTags = append(spec.Source.Kustomize.ImageTags, argoappv1.KustomizeImageTag{
						Name:  p.Name,
						Value: p.Value,
					})
				}
			}
		}
	}

	// 2. duplicate the preferred fields into legacy componentParameterOverride field so that they
	// are always in-sync.
	// NOTE: this step effectively ignore any changes which made only to the legacy fields. This
	// *should* be OK since older CLIs are blocked, and the UI will be using the new fields. This
	// may break custom REST API clients
	var cpo []argoappv1.ComponentParameter
	if spec.Source.Ksonnet != nil {
		for _, p := range spec.Source.Ksonnet.Parameters {
			cpo = append(cpo, argoappv1.ComponentParameter(p))
		}
	}
	if spec.Source.Helm != nil {
		for _, p := range spec.Source.Helm.Parameters {
			cpo = append(cpo, argoappv1.ComponentParameter{
				Name:  p.Name,
				Value: p.Value,
			})
		}
	}
	if spec.Source.Kustomize != nil {
		for _, p := range spec.Source.Kustomize.ImageTags {
			cpo = append(cpo, argoappv1.ComponentParameter{
				Component: "imagetag",
				Name:      p.Name,
				Value:     p.Value,
			})
		}
	}
	spec.Source.ComponentParameterOverrides = cpo

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
