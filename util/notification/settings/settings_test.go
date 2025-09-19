package settings

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"

	"github.com/argoproj/notifications-engine/pkg/api"
	service "github.com/argoproj/argo-cd/v3/util/notification/argocd"
)

func TestGetFactorySettingsDeferred_ServiceNotInitialized(t *testing.T) {
	var argocdService service.Service // nil service

	settings := GetFactorySettingsDeferred(
		func() service.Service { return argocdService },
		"test-secret",
		"test-configmap",
		false,
	)

	cfg := &api.Config{}
	configMap := &corev1.ConfigMap{}
	secret := &corev1.Secret{}

	_, err := settings.InitGetVars(cfg, configMap, secret)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "argocdService is not initialized")
}
