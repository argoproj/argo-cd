package argo

import (
	"encoding/json"
	"time"

	"github.com/argoproj/argo-cd/common"
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/pkg/client/clientset/versioned/typed/application/v1alpha1"
	log "github.com/sirupsen/logrus"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

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
