package settings

import (
	"context"
	"errors"

	"github.com/argoproj/notifications-engine/pkg/api"
	"github.com/argoproj/notifications-engine/pkg/services"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v3/util/notification/expression"

	service "github.com/argoproj/argo-cd/v3/util/notification/argocd"
)

func GetFactorySettings(ctx context.Context, argocdService service.Service, secretName, configMapName string, selfServiceNotificationEnabled bool) api.Settings {
	return api.Settings{
		SecretName:    secretName,
		ConfigMapName: configMapName,
		InitGetVars: func(cfg *api.Config, configMap *corev1.ConfigMap, secret *corev1.Secret) (api.GetVars, error) {
			if selfServiceNotificationEnabled {
				return initGetVarsWithoutSecret(ctx, argocdService, cfg, configMap, secret)
			}
			return initGetVars(ctx, argocdService, cfg, configMap, secret)
		},
	}
}

// GetFactorySettingsForCLI allows the initialization of argocdService to be deferred until it is used, when InitGetVars is called.
func GetFactorySettingsForCLI(ctx context.Context, argocdService *service.Service, secretName, configMapName string, selfServiceNotificationEnabled bool) api.Settings {
	return api.Settings{
		SecretName:    secretName,
		ConfigMapName: configMapName,
		InitGetVars: func(cfg *api.Config, configMap *corev1.ConfigMap, secret *corev1.Secret) (api.GetVars, error) {
			if *argocdService == nil {
				return nil, errors.New("argocdService is not initialized")
			}

			if selfServiceNotificationEnabled {
				return initGetVarsWithoutSecret(ctx, *argocdService, cfg, configMap, secret)
			}
			return initGetVars(ctx, *argocdService, cfg, configMap, secret)
		},
	}
}

func getContext(cfg *api.Config, configMap *corev1.ConfigMap, secret *corev1.Secret) (map[string]string, error) {
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

func initGetVarsWithoutSecret(ctx context.Context, argocdService service.Service, cfg *api.Config, configMap *corev1.ConfigMap, secret *corev1.Secret) (api.GetVars, error) {
	context, err := getContext(cfg, configMap, secret)
	if err != nil {
		return nil, err
	}

	return func(obj map[string]any, dest services.Destination) map[string]any {
		return expression.Spawn(ctx, &unstructured.Unstructured{Object: obj}, argocdService, map[string]any{
			"app":     obj,
			"context": injectLegacyVar(context, dest.Service),
		})
	}, nil
}

func initGetVars(ctx context.Context, argocdService service.Service, cfg *api.Config, configMap *corev1.ConfigMap, secret *corev1.Secret) (api.GetVars, error) {
	context, err := getContext(cfg, configMap, secret)
	if err != nil {
		return nil, err
	}

	return func(obj map[string]any, dest services.Destination) map[string]any {
		return expression.Spawn(ctx, &unstructured.Unstructured{Object: obj}, argocdService, map[string]any{
			"app":     obj,
			"context": injectLegacyVar(context, dest.Service),
			"secrets": secret.Data,
		})
	}, nil
}
