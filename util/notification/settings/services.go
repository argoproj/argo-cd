package settings

import (
	"strings"

	"github.com/argoproj/notifications-engine/pkg/api"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

// removeLegacyTeamsServices resolves services without retired Office 365 Connector entries, so they cannot
// overwrite supported services using the same custom name during migration.
func removeLegacyTeamsServices(cfg *api.Config, configMap *corev1.ConfigMap, secret *corev1.Secret) error {
	filtered := configMap.DeepCopy()
	removed := false
	for key := range filtered.Data {
		parts := strings.Split(key, ".")
		if (len(parts) == 2 || len(parts) == 3) && parts[0] == "service" && parts[1] == "teams" {
			delete(filtered.Data, key)
			removed = true
			log.Warnf("Ignoring %q: the teams notification service has been removed; migrate to teams-workflows", key)
		}
	}
	if removed {
		supported, err := api.ParseConfig(filtered, secret)
		if err != nil {
			return err
		}
		cfg.Services = supported.Services
	}
	return nil
}
