package settings

import (
	"testing"

	"github.com/argoproj/notifications-engine/pkg/api"
	"github.com/argoproj/notifications-engine/pkg/services"
	"github.com/argoproj/notifications-engine/pkg/subscriptions"
	"github.com/argoproj/notifications-engine/pkg/triggers"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func TestMergeLegacyConfig_DefaultTriggers(t *testing.T) {
	cfg := api.Config{
		Services: map[string]api.ServiceFactory{},
		Triggers: map[string][]triggers.Condition{
			"my-trigger1": {{
				When: "true",
				Send: []string{"my-template1"},
			}},
			"my-trigger2": {{
				When: "false",
				Send: []string{"my-template2"},
			}},
		},
	}
	context := map[string]string{}
	configYAML := `
config.yaml:
triggers:
- name: my-trigger1
  enabled: true
`
	err := ApplyLegacyConfig(&cfg,
		context,
		&v1.ConfigMap{Data: map[string]string{"config.yaml": configYAML}},
		&v1.Secret{Data: map[string][]byte{}},
	)
	assert.NoError(t, err)
	assert.Equal(t, []string{"my-trigger1"}, cfg.DefaultTriggers)
}

func TestMergeLegacyConfig(t *testing.T) {
	cfg := api.Config{
		Templates: map[string]services.Notification{"my-template1": {Message: "foo"}},
		Triggers: map[string][]triggers.Condition{
			"my-trigger1": {{
				When: "true",
				Send: []string{"my-template1"},
			}},
		},
		Services:      map[string]api.ServiceFactory{},
		Subscriptions: []subscriptions.DefaultSubscription{{Triggers: []string{"my-trigger1"}}},
	}
	context := map[string]string{"some": "value"}

	configYAML := `
triggers:
- name: my-trigger1
  enabled: true
- name: my-trigger2
  condition: false
  template: my-template2
  enabled: true
templates:
- name: my-template1
  body: bar
- name: my-template2
  body: foo
context:
  other: value2
subscriptions:
- triggers:
  - my-trigger2
  selector: test=true
`
	notifiersYAML := `
slack:
  token: my-token
`
	err := ApplyLegacyConfig(&cfg, context,
		&v1.ConfigMap{Data: map[string]string{"config.yaml": configYAML}},
		&v1.Secret{Data: map[string][]byte{"notifiers.yaml": []byte(notifiersYAML)}},
	)

	assert.NoError(t, err)
	assert.Equal(t, map[string]services.Notification{
		"my-template1": {Message: "bar"},
		"my-template2": {Message: "foo"},
	}, cfg.Templates)

	assert.Equal(t, []triggers.Condition{{
		When: "true",
		Send: []string{"my-template1"},
	}}, cfg.Triggers["my-trigger1"])
	assert.Equal(t, []triggers.Condition{{
		When: "false",
		Send: []string{"my-template2"},
	}}, cfg.Triggers["my-trigger2"])

	label, err := labels.Parse("test=true")
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, subscriptions.DefaultSubscriptions([]subscriptions.DefaultSubscription{
		{Triggers: []string{"my-trigger2"}, Selector: label},
	}), cfg.Subscriptions)
	assert.NotNil(t, cfg.Services["slack"])
}

func TestGetDestinations(t *testing.T) {
	res := GetLegacyDestinations(map[string]string{
		"my-trigger.recipients.argocd-notifications.argoproj.io": "slack:my-channel",
	}, []string{}, nil)
	assert.Equal(t, services.Destinations{
		"my-trigger": []services.Destination{{
			Recipient: "my-channel",
			Service:   "slack",
		},
		}}, res)
}

func TestGetDestinations_DefaultTrigger(t *testing.T) {
	res := GetLegacyDestinations(map[string]string{
		"recipients.argocd-notifications.argoproj.io": "slack:my-channel",
	}, []string{"my-trigger"}, nil)
	assert.Equal(t, services.Destinations{
		"my-trigger": []services.Destination{{
			Recipient: "my-channel",
			Service:   "slack",
		}},
	}, res)
}

func TestGetDestinations_ServiceDefaultTriggers(t *testing.T) {
	res := GetLegacyDestinations(map[string]string{
		"recipients.argocd-notifications.argoproj.io": "slack:my-channel",
	}, []string{}, map[string][]string{
		"slack": {
			"trigger-a",
			"trigger-b",
		},
	})
	assert.Equal(t, services.Destinations{
		"trigger-a": []services.Destination{{
			Recipient: "my-channel",
			Service:   "slack",
		}},
		"trigger-b": []services.Destination{{
			Recipient: "my-channel",
			Service:   "slack",
		}},
	}, res)
}
