package settings

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/argoproj/notifications-engine/pkg/api"
	"github.com/argoproj/notifications-engine/pkg/services"
	"github.com/argoproj/notifications-engine/pkg/subscriptions"
	"github.com/argoproj/notifications-engine/pkg/triggers"
	"github.com/argoproj/notifications-engine/pkg/util/text"
	jsonpatch "github.com/evanphx/json-patch"
	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

type legacyTemplate struct {
	Name  string `json:"name,omitempty"`
	Title string `json:"subject,omitempty"`
	Body  string `json:"body,omitempty"`
	services.Notification
}

type legacyTrigger struct {
	Name        string `json:"name,omitempty"`
	Condition   string `json:"condition,omitempty"`
	Description string `json:"description,omitempty"`
	Template    string `json:"template,omitempty"`
	Enabled     *bool  `json:"enabled,omitempty"`
}

type legacyConfig struct {
	Triggers      []legacyTrigger                    `json:"triggers,omitempty"`
	Templates     []legacyTemplate                   `json:"templates,omitempty"`
	Context       map[string]string                  `json:"context,omitempty"`
	Subscriptions subscriptions.DefaultSubscriptions `json:"subscriptions,omitempty"`
}

type legacyWebhookOptions struct {
	services.WebhookOptions
	Name string `json:"name"`
}

type legacyServicesConfig struct {
	Email    *services.EmailOptions    `json:"email"`
	Slack    *services.SlackOptions    `json:"slack"`
	Opsgenie *services.OpsgenieOptions `json:"opsgenie"`
	Grafana  *services.GrafanaOptions  `json:"grafana"`
	Webhook  []legacyWebhookOptions    `json:"webhook"`
}

func mergePatch(orig interface{}, other interface{}) error {
	origData, err := json.Marshal(orig)
	if err != nil {
		return err
	}
	otherData, err := json.Marshal(other)
	if err != nil {
		return err
	}

	if string(otherData) == "null" {
		return nil
	}

	mergedData, err := jsonpatch.MergePatch(origData, otherData)
	if err != nil {
		return err
	}
	return json.Unmarshal(mergedData, orig)
}

func (legacy legacyConfig) merge(cfg *api.Config, context map[string]string) error {
	if err := mergePatch(&context, &legacy.Context); err != nil {
		return err
	}
	if err := mergePatch(&cfg.Subscriptions, &legacy.Subscriptions); err != nil {
		return err
	}

	for _, template := range legacy.Templates {
		t, ok := cfg.Templates[template.Name]
		if ok {
			if err := mergePatch(&t, &template.Notification); err != nil {
				return err
			}
		}
		if template.Title != "" {
			if template.Notification.Email == nil {
				template.Notification.Email = &services.EmailNotification{}
			}
			template.Notification.Email.Subject = template.Title
		}
		if template.Body != "" {
			template.Notification.Message = template.Body
		}
		cfg.Templates[template.Name] = template.Notification
	}

	for _, trigger := range legacy.Triggers {
		if trigger.Enabled != nil && *trigger.Enabled {
			cfg.DefaultTriggers = append(cfg.DefaultTriggers, trigger.Name)
		}
		var firstCondition triggers.Condition
		t, ok := cfg.Triggers[trigger.Name]
		if !ok || len(t) == 0 {
			t = []triggers.Condition{firstCondition}
		} else {
			firstCondition = t[0]
		}

		if trigger.Condition != "" {
			firstCondition.When = trigger.Condition
		}
		if trigger.Template != "" {
			firstCondition.Send = []string{trigger.Template}
		}
		if trigger.Description != "" {
			firstCondition.Description = trigger.Description
		}
		t[0] = firstCondition
		cfg.Triggers[trigger.Name] = t
	}

	return nil
}

func (c *legacyServicesConfig) merge(cfg *api.Config) {
	if c.Email != nil {
		cfg.Services["email"] = func() (services.NotificationService, error) {
			return services.NewEmailService(*c.Email), nil
		}
	}
	if c.Slack != nil {
		cfg.Services["slack"] = func() (services.NotificationService, error) {
			return services.NewSlackService(*c.Slack), nil
		}
	}
	if c.Grafana != nil {
		cfg.Services["grafana"] = func() (services.NotificationService, error) {
			return services.NewGrafanaService(*c.Grafana), nil
		}
	}
	if c.Opsgenie != nil {
		cfg.Services["opsgenie"] = func() (services.NotificationService, error) {
			return services.NewOpsgenieService(*c.Opsgenie), nil
		}
	}
	for i := range c.Webhook {
		opts := c.Webhook[i]
		cfg.Services[fmt.Sprintf(opts.Name)] = func() (services.NotificationService, error) {
			return services.NewWebhookService(opts.WebhookOptions), nil
		}
	}
}

// ApplyLegacyConfig settings specified using deprecated config map and secret keys
func ApplyLegacyConfig(cfg *api.Config, context map[string]string, cm *v1.ConfigMap, secret *v1.Secret) error {
	if notifiersData, ok := secret.Data["notifiers.yaml"]; ok && len(notifiersData) > 0 {
		log.Warn("Key 'notifiers.yaml' in Secret is deprecated, please migrate to new settings")
		legacyServices := &legacyServicesConfig{}
		err := yaml.Unmarshal(notifiersData, legacyServices)
		if err != nil {
			return err
		}
		legacyServices.merge(cfg)
	}

	if configData, ok := cm.Data["config.yaml"]; ok && configData != "" {
		log.Warn("Key 'config.yaml' in ConfigMap is deprecated, please migrate to new settings")
		legacyCfg := &legacyConfig{}
		err := yaml.Unmarshal([]byte(configData), legacyCfg)
		if err != nil {
			return err
		}
		err = legacyCfg.merge(cfg, context)
		if err != nil {
			return err
		}
	}
	return nil
}

const (
	annotationKey = "recipients.argocd-notifications.argoproj.io"
)

func GetLegacyDestinations(annotations map[string]string, defaultTriggers []string, serviceDefaultTriggers map[string][]string) services.Destinations {
	dests := services.Destinations{}
	for k, v := range annotations {
		if !strings.HasSuffix(k, annotationKey) {
			continue
		}

		var triggerNames []string
		triggerName := strings.TrimRight(k[0:len(k)-len(annotationKey)], ".")
		if triggerName == "" {
			triggerNames = defaultTriggers
		} else {
			triggerNames = []string{triggerName}
		}

		for _, recipient := range text.SplitRemoveEmpty(v, ",") {
			if recipient = strings.TrimSpace(recipient); recipient != "" {
				parts := strings.Split(recipient, ":")
				dest := services.Destination{Service: parts[0]}
				if len(parts) > 1 {
					dest.Recipient = parts[1]
				}

				t := triggerNames
				if v, ok := serviceDefaultTriggers[dest.Service]; ok {
					t = v
				}
				for _, name := range t {
					dests[name] = append(dests[name], dest)
				}
			}
		}
	}
	return dests
}

// injectLegacyVar injects legacy variable into context
func injectLegacyVar(ctx map[string]string, serviceType string) map[string]string {
	res := map[string]string{
		"notificationType": serviceType,
	}
	for k, v := range ctx {
		res[k] = v
	}
	return res
}
