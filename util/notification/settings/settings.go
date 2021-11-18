package settings

import (
	"github.com/argoproj-labs/argocd-notifications/expr"
	"github.com/argoproj-labs/argocd-notifications/shared/argocd"
	"github.com/argoproj-labs/argocd-notifications/shared/k8s"
	"github.com/argoproj/notifications-engine/pkg/api"
	"github.com/argoproj/notifications-engine/pkg/services"
	"github.com/ghodss/yaml"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func GetFactorySettings(argocdService argocd.Service) api.Settings {
	return api.Settings{
		SecretName:    k8s.SecretName,
		ConfigMapName: k8s.ConfigMapName,
		InitGetVars: func(cfg *api.Config, configMap *v1.ConfigMap, secret *v1.Secret) (api.GetVars, error) {
			return initGetVars(argocdService, cfg, configMap, secret)
		},
	}
}

func initGetVars(argocdService argocd.Service, cfg *api.Config, configMap *v1.ConfigMap, secret *v1.Secret) (api.GetVars, error) {
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
		return expr.Spawn(&unstructured.Unstructured{Object: obj}, argocdService, map[string]interface{}{
			"app":     obj,
			"context": injectLegacyVar(context, dest.Service),
		})
	}, nil
}
