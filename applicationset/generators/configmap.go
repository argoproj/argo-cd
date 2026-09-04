package generators

import (
	"context"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

var _ Generator = (*ConfigMapGenerator)(nil)

// ConfigMapGenerator generates a single set of parameters from the data of a referenced ConfigMap.
type ConfigMapGenerator struct {
	client    client.Client
	namespace string
}

func NewConfigMapGenerator(c client.Client, namespace string) Generator {
	return &ConfigMapGenerator{
		client:    c,
		namespace: namespace,
	}
}

func (g *ConfigMapGenerator) GetRequeueAfter(_ *argoprojiov1alpha1.ApplicationSetGenerator) time.Duration {
	return NoRequeueAfter
}

func (g *ConfigMapGenerator) GetTemplate(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) *argoprojiov1alpha1.ApplicationSetTemplate {
	return &appSetGenerator.ConfigMap.Template
}

func (g *ConfigMapGenerator) GenerateParams(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, appSet *argoprojiov1alpha1.ApplicationSet, _ client.Client) ([]map[string]any, error) {
	if appSetGenerator == nil {
		return nil, ErrEmptyAppSetGenerator
	}

	if appSetGenerator.ConfigMap == nil {
		return nil, ErrEmptyAppSetGenerator
	}

	if appSetGenerator.ConfigMap.ConfigMapRef == "" {
		return nil, errors.New("ConfigMap generator requires configMapRef to be set")
	}

	ctx := context.Background()

	cm := &corev1.ConfigMap{}
	err := g.client.Get(
		ctx,
		client.ObjectKey{
			Name:      appSetGenerator.ConfigMap.ConfigMapRef,
			Namespace: g.namespace,
		},
		cm)
	if err != nil {
		return nil, fmt.Errorf("error fetching ConfigMap %s/%s: %w", g.namespace, appSetGenerator.ConfigMap.ConfigMapRef, err)
	}

	// The ConfigMap data is a flat map of strings, so each key/value pair is exposed
	// directly as a parameter. This works for both the goTemplate and fasttemplate
	// rendering modes.
	params := map[string]any{}
	for k, v := range cm.Data {
		params[k] = v
	}

	if err := appendTemplatedValues(appSetGenerator.ConfigMap.Values, params, appSet.Spec.GoTemplate, appSet.Spec.GoTemplateOptions); err != nil {
		return nil, fmt.Errorf("failed to append templated values: %w", err)
	}

	return []map[string]any{params}, nil
}
