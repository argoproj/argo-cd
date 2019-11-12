package argo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/types"

	"github.com/argoproj/argo-cd/engine/pkg"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/engine/common"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/argoproj/argo-cd/engine/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/engine/pkg/client/clientset/versioned/typed/application/v1alpha1"
	applisters "github.com/argoproj/argo-cd/engine/pkg/client/listers/application/v1alpha1"
)

const (
	errDestinationMissing = "Destination server and/or namespace missing from app spec"
)

// FormatAppConditions returns string representation of give app condition list
func FormatAppConditions(conditions []v1alpha1.ApplicationCondition) string {
	formattedConditions := make([]string, 0)
	for _, condition := range conditions {
		formattedConditions = append(formattedConditions, fmt.Sprintf("%s: %s", condition.Type, condition.Message))
	}
	return strings.Join(formattedConditions, ";")
}

// GetAppProject returns a project from an application
func GetAppProject(spec *v1alpha1.ApplicationSpec, projLister applisters.AppProjectLister, ns string) (*v1alpha1.AppProject, error) {
	return projLister.AppProjects(ns).Get(spec.GetProject())
}

// ValidatePermissions ensures that the referenced cluster has been added to Argo CD and the app source repo and destination namespace/cluster are permitted in app project
func ValidatePermissions(ctx context.Context, spec *v1alpha1.ApplicationSpec, proj *v1alpha1.AppProject, db pkg.CredentialsStore) ([]v1alpha1.ApplicationCondition, error) {
	conditions := make([]v1alpha1.ApplicationCondition, 0)
	if spec.Source.RepoURL == "" || (spec.Source.Path == "" && spec.Source.Chart == "") {
		conditions = append(conditions, v1alpha1.ApplicationCondition{
			Type:    v1alpha1.ApplicationConditionInvalidSpecError,
			Message: "spec.source.repoURL and spec.source.path either spec.source.chart are required",
		})
		return conditions, nil
	}
	if spec.Source.Chart != "" && spec.Source.TargetRevision == "" {
		conditions = append(conditions, v1alpha1.ApplicationCondition{
			Type:    v1alpha1.ApplicationConditionInvalidSpecError,
			Message: "spec.source.targetRevision is required if the manifest source is a helm chart",
		})
		return conditions, nil
	}

	if !proj.IsSourcePermitted(spec.Source) {
		conditions = append(conditions, v1alpha1.ApplicationCondition{
			Type:    v1alpha1.ApplicationConditionInvalidSpecError,
			Message: fmt.Sprintf("application repo %s is not permitted in project '%s'", spec.Source.RepoURL, spec.Project),
		})
	}

	if spec.Destination.Server != "" && spec.Destination.Namespace != "" {
		if !proj.IsDestinationPermitted(spec.Destination) {
			conditions = append(conditions, v1alpha1.ApplicationCondition{
				Type:    v1alpha1.ApplicationConditionInvalidSpecError,
				Message: fmt.Sprintf("application destination %v is not permitted in project '%s'", spec.Destination, spec.Project),
			})
		}
		// Ensure the k8s cluster the app is referencing, is configured in Argo CD
		_, err := db.GetCluster(ctx, spec.Destination.Server)
		if err != nil {
			if errStatus, ok := status.FromError(err); ok && errStatus.Code() == codes.NotFound {
				conditions = append(conditions, v1alpha1.ApplicationCondition{
					Type:    v1alpha1.ApplicationConditionInvalidSpecError,
					Message: fmt.Sprintf("cluster '%s' has not been configured", spec.Destination.Server),
				})
			} else {
				return nil, err
			}
		}
	} else {
		conditions = append(conditions, v1alpha1.ApplicationCondition{Type: v1alpha1.ApplicationConditionInvalidSpecError, Message: errDestinationMissing})
	}
	return conditions, nil
}

// NormalizeApplicationSpec will normalize an application spec to a preferred state. This is used
// for migrating application objects which are using deprecated legacy fields into the new fields,
// and defaulting fields in the spec (e.g. spec.project)
func NormalizeApplicationSpec(spec *v1alpha1.ApplicationSpec) *v1alpha1.ApplicationSpec {
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

// SetAppOperation updates an application with the specified operation, retrying conflict errors
func SetAppOperation(appIf appclientset.ApplicationInterface, appName string, op *v1alpha1.Operation) (*v1alpha1.Application, error) {
	for {
		a, err := appIf.Get(appName, v1.GetOptions{})
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
		if !errors.IsConflict(err) {
			return nil, err
		}
		logrus.Warnf("Failed to set operation for app '%s' due to update conflict. Retrying again...", appName)
	}
}

// ContainsSyncResource determines if the given resource exists in the provided slice of sync operation resources.
func ContainsSyncResource(name string, gvk schema.GroupVersionKind, rr []v1alpha1.SyncOperationResource) bool {
	for _, r := range rr {
		if r.HasIdentity(name, gvk) {
			return true
		}
	}
	return false
}

// RefreshApp updates the refresh annotation of an application to coerce the controller to process it
func RefreshApp(appIf appclientset.ApplicationInterface, name string, refreshType v1alpha1.RefreshType) (*v1alpha1.Application, error) {
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
			if !errors.IsConflict(err) {
				return nil, err
			}
		} else {
			logrus.Infof("Requested app '%s' refresh", name)
			return app, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, err
}
