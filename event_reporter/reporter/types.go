package reporter

import (
	"github.com/argoproj/argo-cd/v2/event_reporter/utils"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
)

type ReportedResource struct {
	rs             *appv1.ResourceStatus
	rsAsAppInfo    *ReportedResourceAsApp // passed if resource is application
	actualState    *application.ApplicationResourceResponse
	desiredState   *apiclient.Manifest
	manifestGenErr bool
}

type ReportedResourceAsApp struct {
	app                 *appv1.Application
	revisionsMetadata   *utils.AppSyncRevisionsMetadata
	applicationVersions *apiclient.ApplicationVersions
}

type ReportedEntityParentApp struct {
	app               *appv1.Application
	appTree           *appv1.ApplicationTree
	revisionsMetadata *utils.AppSyncRevisionsMetadata
}

type ArgoTrackingMetadata struct {
	AppInstanceLabelKey *string
	TrackingMethod      *appv1.TrackingMethod
}

func (rr *ReportedResource) GetApiVersion() string {
	apiVersion := rr.rs.Version
	if rr.rs.Group != "" {
		apiVersion = rr.rs.Group + "/" + rr.rs.Version
	}

	return apiVersion
}
