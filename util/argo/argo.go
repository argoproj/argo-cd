package argo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/argoproj/argo-cd/common"
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/pkg/client/clientset/versioned/typed/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver"
	"github.com/argoproj/argo-cd/reposerver/repository"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/ksonnet"
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

// ParamToMap converts a ComponentParameter list to a map for easy filtering
func ParamToMap(params []argoappv1.ComponentParameter) map[string]map[string]bool {
	validAppSet := make(map[string]map[string]bool)
	for _, p := range params {
		if validAppSet[p.Component] == nil {
			validAppSet[p.Component] = make(map[string]bool)
		}
		validAppSet[p.Component][p.Name] = true
	}
	return validAppSet
}

// CheckValidParam checks if the parameter passed is overridable for the given appMap
func CheckValidParam(appMap map[string]map[string]bool, newParam argoappv1.ComponentParameter) bool {
	if val, ok := appMap[newParam.Component]; ok {
		if _, ok2 := val[newParam.Name]; ok2 {
			return true
		}
	}
	return false
}

// RefreshApp updates the refresh annotation of an application to coerce the controller to process it
func RefreshApp(appIf v1alpha1.ApplicationInterface, name string) (*argoappv1.Application, error) {
	refreshString := time.Now().UTC().Format(time.RFC3339)
	metadata := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]string{
				common.AnnotationKeyRefresh: refreshString,
			},
		},
		"status": map[string]interface{}{
			"comparisonResult": map[string]interface{}{
				"comparedAt": nil,
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
			log.Infof("Refreshed app '%s' for controller reprocessing (%s)", name, refreshString)
			return app, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, err
}

// WaitForRefresh watches an application until its comparison timestamp is after the refresh timestamp
func WaitForRefresh(appIf v1alpha1.ApplicationInterface, name string, timeout *time.Duration) (*argoappv1.Application, error) {
	ctx := context.Background()
	var cancel context.CancelFunc
	if timeout != nil {
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}
	fieldSelector := fields.ParseSelectorOrDie(fmt.Sprintf("metadata.name=%s", name))
	listOpts := metav1.ListOptions{FieldSelector: fieldSelector.String()}
	watchIf, err := appIf.Watch(listOpts)
	if err != nil {
		return nil, err
	}
	defer watchIf.Stop()

	for {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			if err != nil {
				if err == context.DeadlineExceeded {
					return nil, fmt.Errorf("Timed out (%v) waiting for application to refresh", timeout)
				}
				return nil, fmt.Errorf("Error waiting for refresh: %v", err)
			}
			return nil, fmt.Errorf("Application watch on %s closed", name)
		case next := <-watchIf.ResultChan():
			if next.Type == watch.Error {
				errMsg := "Application watch completed with error"
				if status, ok := next.Object.(*metav1.Status); ok {
					errMsg = fmt.Sprintf("%s: %v", errMsg, status)
				}
				return nil, errors.New(errMsg)
			}
			app, ok := next.Object.(*argoappv1.Application)
			if !ok {
				return nil, fmt.Errorf("Application event object failed conversion: %v", next)
			}
			refreshTimestampStr := app.ObjectMeta.Annotations[common.AnnotationKeyRefresh]
			refreshTimestamp, err := time.Parse(time.RFC3339, refreshTimestampStr)
			if err != nil {
				return nil, fmt.Errorf("Unable to parse '%s': %v", common.AnnotationKeyRefresh, err)
			}
			if app.Status.ComparisonResult.ComparedAt.After(refreshTimestamp) {
				return app, nil
			}
		}
	}
}

// GetSpecErrors returns list of conditions which indicates that app spec is invalid. Following is checked:
// * the git repository is accessible
// * the git path contains a valid app.yaml
// * the specified environment exists
// * the referenced cluster has been added to ArgoCD
// * the app source repo and destination namespace/cluster are permitted in app project
func GetSpecErrors(
	ctx context.Context, spec *argoappv1.ApplicationSpec, proj *argoappv1.AppProject, repoClientset reposerver.Clientset, db db.ArgoDB) ([]argoappv1.ApplicationCondition, error) {

	conditions := make([]argoappv1.ApplicationCondition, 0)

	// Test the repo
	conn, repoClient, err := repoClientset.NewRepositoryClient()
	if err != nil {
		return nil, err
	}

	repoAccessable := false
	defer util.Close(conn)
	repoRes, err := db.GetRepository(ctx, spec.Source.RepoURL)
	if err != nil {
		if errStatus, ok := status.FromError(err); ok && errStatus.Code() == codes.NotFound {
			// The repo has not been added to ArgoCD so we do not have credentials to access it.
			// We support the mode where apps can be created from public repositories. Test the
			// repo to make sure it is publicly accessible
			err = git.TestRepo(spec.Source.RepoURL, "", "", "")
			if err != nil {
				conditions = append(conditions, argoappv1.ApplicationCondition{
					Type:    argoappv1.ApplicationConditionInvalidSpecError,
					Message: fmt.Sprintf("No credentials available for source repository and repository is not publicly accessible: %v", err),
				})
			} else {
				repoAccessable = true
			}
		} else {
			return nil, err
		}
	} else {
		repoAccessable = true
	}

	if repoAccessable {
		appSourceType, err := queryAppSourceType(ctx, spec, repoRes, repoClient)
		if err != nil {
			conditions = append(conditions, argoappv1.ApplicationCondition{
				Type:    argoappv1.ApplicationConditionInvalidSpecError,
				Message: fmt.Sprintf("Unable to determine app source type: %v", err),
			})
		} else {
			switch appSourceType {
			case repository.AppSourceKsonnet:
				err := verifyAppYAML(ctx, repoRes, spec, repoClient)
				if err != nil {
					conditions = append(conditions, argoappv1.ApplicationCondition{
						Type:    argoappv1.ApplicationConditionInvalidSpecError,
						Message: err.Error(),
					})

				}
			case repository.AppSourceHelm:
				helmConditions := verifyHelmChart(ctx, repoRes, spec, repoClient)
				if len(helmConditions) > 0 {
					conditions = append(conditions, helmConditions...)
				}
			case repository.AppSourceDirectory, repository.AppSourceKustomize:
				maniDirConditions := verifyGenerateManifests(ctx, repoRes, spec, repoClient)
				if len(maniDirConditions) > 0 {
					conditions = append(conditions, maniDirConditions...)
				}
			}

		}
	}

	if spec.Project == "" {
		spec.Project = common.DefaultAppProjectName
	}

	if !proj.IsSourcePermitted(spec.Source) {
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
		// Ensure the k8s cluster the app is referencing, is configured in ArgoCD
		_, err = db.GetCluster(ctx, spec.Destination.Server)
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
	}
	return conditions, nil
}

// GetAppProject returns a project from an application
func GetAppProject(spec *argoappv1.ApplicationSpec, appclientset appclientset.Interface, ns string) (*argoappv1.AppProject, error) {
	if spec.BelongsToDefaultProject() {
		return appclientset.ArgoprojV1alpha1().AppProjects(ns).Get(common.DefaultAppProjectName, metav1.GetOptions{})
	}
	return appclientset.ArgoprojV1alpha1().AppProjects(ns).Get(spec.Project, metav1.GetOptions{})
}

// queryAppSourceType queries repo server for yaml files in a directory, and determines its
// application source type based on the files in the directory.
func queryAppSourceType(ctx context.Context, spec *argoappv1.ApplicationSpec, repoRes *argoappv1.Repository, repoClient repository.RepositoryServiceClient) (repository.AppSourceType, error) {
	req := repository.ListDirRequest{
		Repo: &argoappv1.Repository{
			Repo: spec.Source.RepoURL,
		},
		Revision: spec.Source.TargetRevision,
		Path:     fmt.Sprintf("%s/*.yaml", spec.Source.Path),
	}
	if repoRes != nil {
		req.Repo.Username = repoRes.Username
		req.Repo.Password = repoRes.Password
		req.Repo.SSHPrivateKey = repoRes.SSHPrivateKey
	}
	getRes, err := repoClient.ListDir(ctx, &req)
	if err != nil {
		return "", err
	}
	for _, gitPath := range getRes.Items {
		// gitPath may look like: app.yaml, or some/subpath/app.yaml
		trimmedPath := strings.TrimPrefix(gitPath, spec.Source.Path)
		trimmedPath = strings.TrimPrefix(trimmedPath, "/")
		if trimmedPath == "app.yaml" {
			return repository.AppSourceKsonnet, nil
		}
		if trimmedPath == "Chart.yaml" {
			return repository.AppSourceHelm, nil
		}
		if trimmedPath == "kustomization.yaml" {
			return repository.AppSourceKustomize, nil
		}
	}
	return repository.AppSourceDirectory, nil
}

// verifyAppYAML verifies that a ksonnet app.yaml is functional
func verifyAppYAML(ctx context.Context, repoRes *argoappv1.Repository, spec *argoappv1.ApplicationSpec, repoClient repository.RepositoryServiceClient) error {
	req := repository.GetFileRequest{
		Repo: &argoappv1.Repository{
			Repo: spec.Source.RepoURL,
		},
		Revision: spec.Source.TargetRevision,
		Path:     path.Join(spec.Source.Path, "app.yaml"),
	}
	if repoRes != nil {
		req.Repo.Username = repoRes.Username
		req.Repo.Password = repoRes.Password
		req.Repo.SSHPrivateKey = repoRes.SSHPrivateKey
	}
	getRes, err := repoClient.GetFile(ctx, &req)
	if err != nil {
		return fmt.Errorf("Unable to load app.yaml: %v", err)
	}

	// Default revision to HEAD if unspecified
	if spec.Source.TargetRevision == "" {
		spec.Source.TargetRevision = "HEAD"
	}

	// Verify the specified environment is defined in the app spec
	dest, err := ksonnet.Destination(getRes.Data, spec.Source.Environment)
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
func verifyHelmChart(ctx context.Context, repoRes *argoappv1.Repository, spec *argoappv1.ApplicationSpec, repoClient repository.RepositoryServiceClient) []argoappv1.ApplicationCondition {
	var conditions []argoappv1.ApplicationCondition
	if spec.Destination.Server == "" || spec.Destination.Namespace == "" {
		conditions = append(conditions, argoappv1.ApplicationCondition{
			Type:    argoappv1.ApplicationConditionInvalidSpecError,
			Message: errDestinationMissing,
		})
	}
	req := repository.GetFileRequest{
		Repo: &argoappv1.Repository{
			Repo: spec.Source.RepoURL,
		},
		Revision: spec.Source.TargetRevision,
		Path:     path.Join(spec.Source.Path, "Chart.yaml"),
	}
	if repoRes != nil {
		req.Repo.Username = repoRes.Username
		req.Repo.Password = repoRes.Password
		req.Repo.SSHPrivateKey = repoRes.SSHPrivateKey
	}
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
func verifyGenerateManifests(ctx context.Context, repoRes *argoappv1.Repository, spec *argoappv1.ApplicationSpec, repoClient repository.RepositoryServiceClient) []argoappv1.ApplicationCondition {
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
		Revision:  spec.Source.TargetRevision,
		Path:      spec.Source.Path,
		Namespace: spec.Destination.Namespace,
	}
	if repoRes != nil {
		req.Repo.Username = repoRes.Username
		req.Repo.Password = repoRes.Password
		req.Repo.SSHPrivateKey = repoRes.SSHPrivateKey
	}
	manRes, err := repoClient.GenerateManifest(ctx, &req)
	if err != nil {
		conditions = append(conditions, argoappv1.ApplicationCondition{
			Type:    argoappv1.ApplicationConditionInvalidSpecError,
			Message: fmt.Sprintf("Unable to generate manifests in %s: %v", spec.Source.Path, err),
		})
	} else if len(manRes.Manifests) == 0 {
		conditions = append(conditions, argoappv1.ApplicationCondition{
			Type:    argoappv1.ApplicationConditionInvalidSpecError,
			Message: fmt.Sprintf("Path '%s' contained no kubernetes manifests", spec.Source.Path),
		})
	}
	return conditions
}

// SetAppOperation updates an application with the specified operation, retrying conflict errors
func SetAppOperation(ctx context.Context, appIf v1alpha1.ApplicationInterface, audit *AuditLogger, appName string, op *argoappv1.Operation) (*argoappv1.Application, error) {
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
		if op.Sync == nil && op.Rollback == nil {
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
