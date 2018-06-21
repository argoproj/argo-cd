package argo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/argoproj/argo-cd/common"
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/pkg/client/clientset/versioned/typed/application/v1alpha1"
)

// FilterByProject returns applications which belongs to the specified project
func FilterByProject(apps []argoappv1.Application, project string) []argoappv1.Application {
	items := make([]argoappv1.Application, 0)
	for i := 0; i < len(apps); i++ {
		a := apps[i]
		if project == a.Spec.GetProject() {
			items = append(items, a)
		}
	}
	return items
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

// WaitForRefresh watches a workflow until its comparison timestamp is after the refresh timestamp
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
