package argo

import (
	"encoding/json"
	"time"

	"github.com/argoproj/argo-cd/common"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/pkg/client/clientset/versioned/typed/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver/repository"
	log "github.com/sirupsen/logrus"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// ResolveServerNamespace resolves server and namespace to use given an application spec,
// and a manifest response. It looks to explicit server/namespace overridden in the app CRD spec
// and falls back to the server/namespace defined in the ksonnet environment
func ResolveServerNamespace(destination appv1.ApplicationDestination, manifestInfo *repository.ManifestResponse) (string, string) {
	server := manifestInfo.Server
	namespace := manifestInfo.Namespace
	if destination.Server != "" {
		server = destination.Server
	}
	if destination.Namespace != "" {
		namespace = destination.Namespace
	}
	return server, namespace
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
