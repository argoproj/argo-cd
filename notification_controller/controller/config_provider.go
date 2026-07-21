package controller

import (
	"github.com/argoproj/argo-cd/v3/util/configbus"
)

// InitConfigProvider wires the configbus provider after durable fields are
// stored on the controller.
func (c *notificationController) InitConfigProvider() {
	//nolint:staticcheck // SA1019: StaticFields capture construction-time opts once at wire-up
	c.configProvider = configbus.NewChainProvider(
		&configbus.StaticProvider{Fields: configbus.StaticFields{
			NotificationsAppLabelSelector:        configbus.Ptr(c.appLabelSelector),
			ApplicationNamespaces:                 configbus.Ptr(c.applicationNamespaces),
			NotificationsConfigMapName:           configbus.Ptr(c.configMapName),
			NotificationsSecretName:              configbus.Ptr(c.secretName),
			NotificationsSelfserviceEnabled:      configbus.Ptr(c.selfServiceNotificationEnabled),
		}},
		configbus.NewEnvProvider(),
	)
}
