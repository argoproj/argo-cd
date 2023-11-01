package settings

import (
	"github.com/argoproj/notifications-engine/pkg/api"
	"github.com/argoproj/notifications-engine/pkg/services"
	"github.com/ghodss/yaml"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v2/util/notification/expression"

	service "github.com/argoproj/argo-cd/v2/util/notification/argocd"
)

func GetFactorySettings(argocdService service.Service, secretName, configMapName string) api.Settings {
	return api.Settings{
		SecretName:    secretName,
		ConfigMapName: configMapName,
		InitGetVars: func(cfg *api.Config, configMap *v1.ConfigMap, secret *v1.Secret) (api.GetVars, error) {
			return initGetVars(argocdService, cfg, configMap, secret)
		},
	}
}

func initGetVars(argocdService service.Service, cfg *api.Config, configMap *v1.ConfigMap, secret *v1.Secret) (api.GetVars, error) {
	context := map[string]string{}
	if contextYaml, ok := configMap.Data["context"]; ok {
		if err := yaml.Unmarshal([]byte(contextYaml), &context); err != nil {
			return nil, err
		}
	}
	if err := ApplyLegacyConfig(cfg, context, configMap, secret); err != nil {
		return nil, err
	}

	return func(obj map[string]interface{}, dest services.Destination) map[string]interface{} {
		return expression.Spawn(&unstructured.Unstructured{Object: obj}, argocdService, map[string]interface{}{
			"app":     obj,
			"context": injectLegacyVar(context, dest.Service),
		})
	}, nil
}
