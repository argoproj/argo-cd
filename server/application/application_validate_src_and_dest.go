package application

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apierr "k8s.io/apimachinery/pkg/api/errors"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	applisters "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/db"
)

func (s *Server) ValidateSrcAndDst(ctx context.Context, requset *application.ApplicationValidationRequest) (*application.ApplicationValidateResponse, error) {
	app := requset.Application
	proj, err := argo.GetAppProject(app, applisters.NewAppProjectLister(s.projInformer.GetIndexer()), s.ns, s.settingsMgr, s.db, ctx)
	if err != nil {
		entity := projectEntity
		if apierr.IsNotFound(err) {
			errMsg := fmt.Sprintf("application references project %s which does not exist", app.Spec.Project)
			return &application.ApplicationValidateResponse{
				Error:  &errMsg,
				Entity: &entity,
			}, nil
		}
		errMsg := err.Error()
		return &application.ApplicationValidateResponse{
			Error:  &errMsg,
			Entity: &entity,
		}, nil
	}

	if err := validateDestination(ctx, &app.Spec.Destination, s.db); err != nil {
		entity := destinationEntity
		errMsg := fmt.Sprintf("application destination spec for %s is invalid: %s", app.ObjectMeta.Name, err.Error())
		return &application.ApplicationValidateResponse{
			Error:  &errMsg,
			Entity: &entity,
		}, nil
	}
	var conditions []appv1.ApplicationCondition
	conditions, err = argo.ValidateRepo(ctx, app, s.repoClientset, s.db, s.kubectl, proj, s.settingsMgr)
	if err != nil {
		entity := sourceEntity
		errMsg := err.Error()
		return &application.ApplicationValidateResponse{
			Error:  &errMsg,
			Entity: &entity,
		}, nil
	}
	if len(conditions) > 0 {
		entity := sourceEntity
		errMsg := fmt.Sprintf("application spec for %s is invalid: %s", app.ObjectMeta.Name, argo.FormatAppConditions(conditions))
		return &application.ApplicationValidateResponse{
			Error:  &errMsg,
			Entity: &entity,
		}, nil
	}
	return &application.ApplicationValidateResponse{
		Error:  nil,
		Entity: nil,
	}, nil
}

// validates destination name (argo.ValidateDestination) and server with extra logic
func validateDestination(ctx context.Context, dest *appv1.ApplicationDestination, db db.ArgoDB) error {
	err := argo.ValidateDestination(ctx, dest, db)
	if err != nil {
		return err
	}

	if dest.Server != "" {
		// Ensure the k8s cluster the app is referencing, is configured in Argo CD
		_, err := db.GetCluster(ctx, dest.Server)
		if err != nil {
			if errStatus, ok := status.FromError(err); ok && errStatus.Code() == codes.NotFound {
				return fmt.Errorf("cluster '%s' has not been configured", dest.Server)
			} else {
				return err
			}
		}
	} else if dest.Server == "" {
		return fmt.Errorf("destination server missing from app spec")
	}

	return nil
}
