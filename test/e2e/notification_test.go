package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/notification"
	notifFixture "github.com/argoproj/argo-cd/v2/test/e2e/fixture/notification"
)

func TestNotificationsListServices(t *testing.T) {
	ctx := notifFixture.Given(t)
	ctx.When().
		SetParamInNotificationConfigMap("service.webhook.test", "url: https://test.example.com").
		Then().Services(func(services *notification.ServiceList, err error) {
		require.NoError(t, err)
		assert.Equal(t, []*notification.Service{{Name: ptr.To("test")}}, services.Items)
	})
}

func TestNotificationsListTemplates(t *testing.T) {
	ctx := notifFixture.Given(t)
	ctx.When().
		SetParamInNotificationConfigMap("template.app-created", "email:\n  subject: Application {{.app.metadata.name}} has been created.\nmessage: Application {{.app.metadata.name}} has been created.\nteams:\n  title: Application {{.app.metadata.name}} has been created.\n").
		Then().Templates(func(templates *notification.TemplateList, err error) {
		require.NoError(t, err)
		assert.Equal(t, []*notification.Template{{Name: ptr.To("app-created")}}, templates.Items)
	})
}

func TestNotificationsListTriggers(t *testing.T) {
	ctx := notifFixture.Given(t)
	ctx.When().
		SetParamInNotificationConfigMap("trigger.on-created", "- description: Application is created.\n  oncePer: app.metadata.name\n  send:\n  - app-created\n  when: \"true\"\n").
		Then().Triggers(func(triggers *notification.TriggerList, err error) {
		require.NoError(t, err)
		assert.Equal(t, []*notification.Trigger{{Name: ptr.To("on-created")}}, triggers.Items)
	})
}
