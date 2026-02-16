package settings

import (
	"context"
	"errors"

	"github.com/argoproj/notifications-engine/pkg/api"
	"github.com/argoproj/notifications-engine/pkg/services"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v3/util/notification/expression"

	service "github.com/argoproj/argo-cd/v3/util/notification/argocd"
)

func GetFactorySettings(argocdService service.Service, secretName, configMapName string, selfServiceNotificationEnabled bool) api.Settings {
	return api.Settings{
		SecretName:    secretName,
		ConfigMapName: configMapName,
		InitGetVars: func(cfg *api.Config, configMap *corev1.ConfigMap, secret *corev1.Secret) (api.GetVars, error) {
			if selfServiceNotificationEnabled {
				return initGetVarsWithoutSecret(argocdService, cfg, configMap, secret)
			}
			return initGetVars(argocdService, cfg, configMap, secret)
		},
	}
}

// GetFactorySettingsForCLI allows the initialization of argocdService to be deferred until it is used, when InitGetVars is called.
func GetFactorySettingsForCLI(serviceGetter func() service.Service, secretName, configMapName string, selfServiceNotificationEnabled bool) api.Settings {
	return api.Settings{
		SecretName:    secretName,
		ConfigMapName: configMapName,
		InitGetVars: func(cfg *api.Config, configMap *corev1.ConfigMap, secret *corev1.Secret) (api.GetVars, error) {
			argocdService := serviceGetter()
			if argocdService == nil {
				return nil, errors.New("argocdService is not initialized")
			}

			if selfServiceNotificationEnabled {
				return initGetVarsWithoutSecret(argocdService, cfg, configMap, secret)
			}
			return initGetVars(argocdService, cfg, configMap, secret)
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

func initGetVarsWithoutSecret(argocdService service.Service, cfg *api.Config, configMap *corev1.ConfigMap, secret *corev1.Secret) (api.GetVars, error) {
	context, err := getContext(cfg, configMap, secret)
	if err != nil {
		return nil, err
	}

	return func(obj map[string]any, dest services.Destination) map[string]any {
		vars := map[string]any{
			"app":     obj,
			"context": injectLegacyVar(context, dest.Service),
		}

		// Add AppProject to template variables
		if appProject := getAppProjectForTemplate(argocdService, obj); appProject != nil {
			vars["appProject"] = appProject
		}

		return expression.Spawn(&unstructured.Unstructured{Object: obj}, argocdService, vars)
	}, nil
}

func initGetVars(argocdService service.Service, cfg *api.Config, configMap *corev1.ConfigMap, secret *corev1.Secret) (api.GetVars, error) {
	context, err := getContext(cfg, configMap, secret)
	if err != nil {
		return nil, err
	}

	return func(obj map[string]any, dest services.Destination) map[string]any {
		vars := map[string]any{
			"app":     obj,
			"context": injectLegacyVar(context, dest.Service),
			"secrets": secret.Data,
		}

		// Add AppProject to template variables
		if appProject := getAppProjectForTemplate(argocdService, obj); appProject != nil {
			vars["appProject"] = appProject
		}

		return expression.Spawn(&unstructured.Unstructured{Object: obj}, argocdService, vars)
	}, nil
}

// getAppProjectForTemplate retrieves the AppProject for an Application object
// Returns nil if the project cannot be found or an error occurs
func getAppProjectForTemplate(argocdService service.Service, obj map[string]any) map[string]any {
	ctx := context.Background()

	// Extract project name from app.spec.project
	spec, ok := obj["spec"].(map[string]any)
	if !ok {
		return nil
	}

	projectName, ok := spec["project"].(string)
	if !ok || projectName == "" {
		projectName = "default"
	}

	// Extract namespace from app.metadata.namespace
	metadata, ok := obj["metadata"].(map[string]any)
	if !ok {
		return nil
	}

	namespace, ok := metadata["namespace"].(string)
	if !ok || namespace == "" {
		return nil
	}

	// Extract app name for logging context
	appName, _ := metadata["name"].(string)

	// Fetch the AppProject
	appProject, err := argocdService.GetAppProject(ctx, projectName, namespace)
	if err != nil {
		log.WithFields(log.Fields{
			"app":       appName,
			"project":   projectName,
			"namespace": namespace,
		}).Warnf("Failed to get AppProject for notification template: %v", err)
		return nil
	}

	// Convert AppProject to unstructured for template access
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(appProject)
	if err != nil {
		log.WithFields(log.Fields{
			"app":       appName,
			"project":   projectName,
			"namespace": namespace,
		}).Warnf("Failed to convert AppProject to unstructured: %v", err)
		return nil
	}

	return unstructuredObj
}
