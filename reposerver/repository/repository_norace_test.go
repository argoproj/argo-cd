// +build !race

package repository

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver/apiclient"
)

func TestHelmDependencyWithConcurrency(t *testing.T) {

	// !race:
	// Un-synchronized use of a random source, will be fixed when this is merged:
	// https://github.com/argoproj/argo-cd/issues/4728

	cleanup := func() {
		_ = os.Remove(filepath.Join("../../util/helm/testdata/helm2-dependency", helmDepUpMarkerFile))
		_ = os.RemoveAll(filepath.Join("../../util/helm/testdata/helm2-dependency", "charts"))
	}
	cleanup()
	defer cleanup()

	helmRepo := argoappv1.Repository{Name: "bitnami", Type: "helm", Repo: "https://charts.bitnami.com/bitnami"}
	var wg sync.WaitGroup
	wg.Add(3)
	for i := 0; i < 3; i++ {
		go func() {
			res, err := helmTemplate("../../util/helm/testdata/helm2-dependency", "../..", nil, &apiclient.ManifestRequest{
				ApplicationSource: &argoappv1.ApplicationSource{},
				Repos:             []*argoappv1.Repository{&helmRepo},
			}, false)

			assert.NoError(t, err)
			assert.NotNil(t, res)
			wg.Done()
		}()
	}
	wg.Wait()
}
