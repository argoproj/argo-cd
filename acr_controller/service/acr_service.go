package service

import (
	"context"
	"encoding/json"
	"sync"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	argoclient "github.com/argoproj/argo-cd/v2/acr_controller/application"
	appclient "github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	application "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
)

type ACRService interface {
	ChangeRevision(ctx context.Context, application *application.Application) error
}

type acrService struct {
	applicationClientset     appclientset.Interface
	applicationServiceClient argoclient.ApplicationClient
	lock                     sync.Mutex
	logger                   *log.Logger
}

func NewACRService(applicationClientset appclientset.Interface, applicationServiceClient argoclient.ApplicationClient) ACRService {
	return &acrService{
		applicationClientset:     applicationClientset,
		applicationServiceClient: applicationServiceClient,
		logger:                   log.New(),
	}
}

func getChangeRevisionFromRevisions(revisions []string) string {
	if len(revisions) > 0 {
		return revisions[0]
	}
	return ""
}

func getChangeRevision(app *application.Application) string {
	if app.Status.OperationState != nil && app.Status.OperationState.Operation.Sync != nil {
		changeRevision := app.Status.OperationState.Operation.Sync.ChangeRevision
		if changeRevision != "" {
			return changeRevision
		}
		if changeRevision = getChangeRevisionFromRevisions(app.Status.OperationState.Operation.Sync.ChangeRevisions); changeRevision != "" {
			return changeRevision
		}
	}
	return ""
}

func (c *acrService) ChangeRevision(ctx context.Context, a *application.Application) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	app, err := c.applicationClientset.ArgoprojV1alpha1().Applications(a.Namespace).Get(ctx, a.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if app.Operation == nil || app.Operation.Sync == nil {
		return nil
	}

	if getChangeRevision(app) != "" {
		c.logger.Infof("Change revision already calculated for application %s", app.Name)
		return nil
	}

	revision, err := c.calculateRevision(ctx, app)
	if err != nil {
		return err
	}

	if revision == nil || *revision == "" {
		c.logger.Infof("Revision for application %s is empty", app.Name)
		return nil
	}

	c.logger.Infof("Change revision for application %s is %s", app.Name, *revision)

	app, err = c.applicationClientset.ArgoprojV1alpha1().Applications(app.Namespace).Get(ctx, app.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	revisions := []string{*revision}

	if app.Status.OperationState != nil && app.Status.OperationState.Operation.Sync != nil {
		c.logger.Infof("Patch operation sync result for application %s", app.Name)
		return c.patchOperationSyncResultWithChangeRevision(ctx, app, revisions)
	}

	c.logger.Infof("Patch operation for application %s", app.Name)
	return c.patchOperationWithChangeRevision(ctx, app, revisions)
}

func (c *acrService) calculateRevision(ctx context.Context, a *application.Application) (*string, error) {
	currentRevision, previousRevision := c.getRevisions(ctx, a)
	c.logger.Infof("Calculate revision for application '%s', current revision '%s', previous revision '%s'", a.Name, currentRevision, previousRevision)
	changeRevisionResult, err := c.applicationServiceClient.GetChangeRevision(ctx, &appclient.ChangeRevisionRequest{
		AppName:          pointer.String(a.GetName()),
		Namespace:        pointer.String(a.GetNamespace()),
		CurrentRevision:  pointer.String(currentRevision),
		PreviousRevision: pointer.String(previousRevision),
	})
	if err != nil {
		return nil, err
	}
	return changeRevisionResult.Revision, nil
}

func (c *acrService) patchOperationWithChangeRevision(ctx context.Context, a *application.Application, revisions []string) error {
	if len(revisions) == 1 {
		patch, _ := json.Marshal(map[string]interface{}{
			"operation": map[string]interface{}{
				"sync": map[string]interface{}{
					"changeRevision": revisions[0],
				},
			},
		})
		_, err := c.applicationClientset.ArgoprojV1alpha1().Applications(a.Namespace).Patch(ctx, a.Name, types.MergePatchType, patch, metav1.PatchOptions{})
		return err
	}

	patch, _ := json.Marshal(map[string]interface{}{
		"operation": map[string]interface{}{
			"sync": map[string]interface{}{
				"changeRevisions": revisions,
			},
		},
	})
	_, err := c.applicationClientset.ArgoprojV1alpha1().Applications(a.Namespace).Patch(ctx, a.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	return err
}

func (c *acrService) patchOperationSyncResultWithChangeRevision(ctx context.Context, a *application.Application, revisions []string) error {
	if len(revisions) == 1 {
		patch, _ := json.Marshal(map[string]interface{}{
			"status": map[string]interface{}{
				"operationState": map[string]interface{}{
					"operation": map[string]interface{}{
						"sync": map[string]interface{}{
							"changeRevision": revisions[0],
						},
					},
				},
			},
		})
		_, err := c.applicationClientset.ArgoprojV1alpha1().Applications(a.Namespace).Patch(ctx, a.Name, types.MergePatchType, patch, metav1.PatchOptions{})
		return err
	}

	patch, _ := json.Marshal(map[string]interface{}{
		"status": map[string]interface{}{
			"operationState": map[string]interface{}{
				"operation": map[string]interface{}{
					"sync": map[string]interface{}{
						"changeRevisions": revisions,
					},
				},
			},
		},
	})
	_, err := c.applicationClientset.ArgoprojV1alpha1().Applications(a.Namespace).Patch(ctx, a.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	return err
}

func getCurrentRevisionFromOperation(a *application.Application) string {
	if a.Operation != nil && a.Operation.Sync != nil {
		return a.Operation.Sync.Revision
	}
	return ""
}

func (c *acrService) getRevisions(ctx context.Context, a *application.Application) (string, string) {
	if a.Status.History == nil || len(a.Status.History) == 0 {
		// it is first sync operation, and we have only current revision
		return getCurrentRevisionFromOperation(a), ""
	}

	// in case if sync is already done, we need to use revision from sync result and previous revision from history
	if a.Status.Sync.Status == "Synced" && a.Status.OperationState != nil && a.Status.OperationState.SyncResult != nil {
		currentRevision := a.Status.OperationState.SyncResult.Revision
		// in case if we have only one history record, we need to return empty previous revision, because it is first sync result
		if len(a.Status.History) == 1 {
			return currentRevision, ""
		}
		return currentRevision, a.Status.History[len(a.Status.History)-2].Revision
	}

	// in case if sync is in progress, we need to use revision from operation and revision from latest history record
	currentRevision := getCurrentRevisionFromOperation(a)
	previousRevision := a.Status.History[len(a.Status.History)-1].Revision
	return currentRevision, previousRevision
}
