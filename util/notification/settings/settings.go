package settings

import (
	"fmt"

	"github.com/argoproj/notifications-engine/pkg/api"
	"github.com/argoproj/notifications-engine/pkg/services"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v2/util/notification/expression"

	service "github.com/argoproj/argo-cd/v2/util/notification/argocd"
)

func GetFactorySettings(argocdService service.Service, secretName, configMapName string, selfServiceNotificationEnabled bool) api.Settings {
	return api.Settings{
		SecretName:    secretName,
		ConfigMapName: configMapName,
		InitGetVars: func(cfg *api.Config, configMap *v1.ConfigMap, secret *v1.Secret) (api.GetVars, error) {
			if selfServiceNotificationEnabled {
				return initGetVarsWithoutSecret(argocdService, cfg, configMap, secret)
			}
			return initGetVars(argocdService, cfg, configMap, secret)
		},
	}
}

// GetFactorySettingsForCLI allows the initialization of argocdService to be deferred until it is used, when InitGetVars is called.
func GetFactorySettingsForCLI(argocdService *service.Service, secretName, configMapName string, selfServiceNotificationEnabled bool) api.Settings {
	return api.Settings{
		SecretName:    secretName,
		ConfigMapName: configMapName,
		InitGetVars: func(cfg *api.Config, configMap *v1.ConfigMap, secret *v1.Secret) (api.GetVars, error) {
			if *argocdService == nil {
				return nil, fmt.Errorf("argocdService is not initialized")
			}

			if selfServiceNotificationEnabled {
				return initGetVarsWithoutSecret(*argocdService, cfg, configMap, secret)
			}
			return initGetVars(*argocdService, cfg, configMap, secret)
		},
	}
}

func getContext(cfg *api.Config, configMap *v1.ConfigMap, secret *v1.Secret) (map[string]string, error) {
	context := map[string]string{}
	if contextYaml, ok := configMap.Data["context"]; ok {
		if err := yaml.Unmarshal([]byte(contextYaml), &context); err != nil {
			return nil, err
		}
	}
	if err := ApplyLegacyConfig(cfg, context, configMap, secret); err != nil {
		return nil, err
	}
	return context, nil
}

func initGetVarsWithoutSecret(argocdService service.Service, cfg *api.Config, configMap *v1.ConfigMap, secret *v1.Secret) (api.GetVars, error) {
	context, err := getContext(cfg, configMap, secret)
	if err != nil {
		return nil, err
	}

	return func(obj map[string]interface{}, dest services.Destination) map[string]interface{} {
		return expression.Spawn(&unstructured.Unstructured{Object: obj}, argocdService, map[string]interface{}{
			"app":     obj,
			"context": injectLegacyVar(context, dest.Service),
		})
	}, nil
}

func initGetVars(argocdService service.Service, cfg *api.Config, configMap *v1.ConfigMap, secret *v1.Secret) (api.GetVars, error) {
	context, err := getContext(cfg, configMap, secret)
	if err != nil {
		return nil, err
	}

	return func(obj map[string]interface{}, dest services.Destination) map[string]interface{} {
		return expression.Spawn(&unstructured.Unstructured{Object: obj}, argocdService, map[string]interface{}{
			"app":     obj,
			"context": injectLegacyVar(context, dest.Service),
			"secrets": secret.Data,
		})
	}, nil
}
