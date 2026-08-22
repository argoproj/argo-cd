package settings

import (
	"context"
	"errors"
	"time"

	"github.com/argoproj/notifications-engine/pkg/api"
	"github.com/argoproj/notifications-engine/pkg/services"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v3/util/notification/expression"

	service "github.com/argoproj/argo-cd/v3/util/notification/argocd"
)

// AppProjectGetter retrieves an AppProject as an unstructured object from a
// local cache, avoiding a live API call on every trigger evaluation. namespace
// is the Application namespace and name is the AppProject name. It returns a nil
// object without an error when the AppProject is not present in the cache.
type AppProjectGetter func(namespace, name string) (*unstructured.Unstructured, error)

func GetFactorySettings(argocdService service.Service, appProjectGetter AppProjectGetter, secretName, configMapName string, selfServiceNotificationEnabled bool) api.Settings {
	return api.Settings{
		SecretName:    secretName,
		ConfigMapName: configMapName,
		InitGetVars: func(cfg *api.Config, configMap *corev1.ConfigMap, secret *corev1.Secret) (api.GetVars, error) {
			if selfServiceNotificationEnabled {
				return initGetVarsWithoutSecret(argocdService, appProjectGetter, cfg, configMap, secret)
			}
			return initGetVars(argocdService, appProjectGetter, cfg, configMap, secret)
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

			// The CLI has no local AppProject cache, so fall back to a live lookup.
			if selfServiceNotificationEnabled {
				return initGetVarsWithoutSecret(argocdService, nil, cfg, configMap, secret)
			}
			return initGetVars(argocdService, nil, cfg, configMap, secret)
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

func initGetVarsWithoutSecret(argocdService service.Service, appProjectGetter AppProjectGetter, cfg *api.Config, configMap *corev1.ConfigMap, secret *corev1.Secret) (api.GetVars, error) {
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
		if appProject := getAppProjectForTemplate(argocdService, appProjectGetter, obj); appProject != nil {
			vars["appProject"] = appProject
		} else {
			vars["appProject"] = map[string]any{}
		}

		return expression.Spawn(&unstructured.Unstructured{Object: obj}, argocdService, vars)
	}, nil
}

func initGetVars(argocdService service.Service, appProjectGetter AppProjectGetter, cfg *api.Config, configMap *corev1.ConfigMap, secret *corev1.Secret) (api.GetVars, error) {
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
		if appProject := getAppProjectForTemplate(argocdService, appProjectGetter, obj); appProject != nil {
			vars["appProject"] = appProject
		} else {
			vars["appProject"] = map[string]any{}
		}

		return expression.Spawn(&unstructured.Unstructured{Object: obj}, argocdService, vars)
	}, nil
}

// getAppProjectForTemplate retrieves the AppProject as an unstructured object for an Application object.
// When an appProjectGetter is provided it reads from the local cache instead of
// making a live API call, which otherwise runs on every trigger evaluation.
// Returns nil if the project cannot be found or an error occurs.
func getAppProjectForTemplate(argocdService service.Service, appProjectGetter AppProjectGetter, obj map[string]any) map[string]any {
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

	logFields := log.Fields{
		"app":       appName,
		"project":   projectName,
		"namespace": namespace,
	}

	// Prefer the local cache when it is available to avoid a live API call on
	// every trigger evaluation.
	if appProjectGetter != nil {
		appProjectObj, err := appProjectGetter(namespace, projectName)
		if err != nil {
			log.WithFields(logFields).Warnf("Failed to get AppProject for notification template: %v", err)
			return nil
		}
		if appProjectObj == nil {
			return nil
		}
		return appProjectObj.Object
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Fetch the AppProject
	appProjectObj, err := argocdService.GetAppProject(ctx, projectName, namespace)
	if err != nil {
		log.WithFields(logFields).Warnf("Failed to get AppProject for notification template: %v", err)
		return nil
	}

	return appProjectObj.Object
}
