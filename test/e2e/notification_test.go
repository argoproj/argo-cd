package e2e

import (
	"testing"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/notification"
	notifFixture "github.com/argoproj/argo-cd/v2/test/e2e/fixture/notification"
	"github.com/stretchr/testify/assert"
)

func TestNotificationsListServices(t *testing.T) {
	ctx := notifFixture.Given(t)
	ctx.When().
		SetParamInNotificationConfigMap("service.webhook.test", "url: https://test.com").
		Then().Services(func(services *notification.Services, err error) {
		assert.Nil(t, err)
		assert.Equal(t, []string{"test"}, services.Services)
	})
}

func TestNotificationsListTemplates(t *testing.T) {
	ctx := notifFixture.Given(t)
	ctx.When().
		SetParamInNotificationConfigMap("template.app-created", "email:\n  subject: Application {{.app.metadata.name}} has been created.\nmessage: Application {{.app.metadata.name}} has been created.\nteams:\n  title: Application {{.app.metadata.name}} has been created.\n").
		Then().Templates(func(templates *notification.Templates, err error) {
		assert.Nil(t, err)
		assert.Equal(t, []string{"app-created"}, templates.Templates)
	})
}

func TestNotificationsListTriggers(t *testing.T) {
	ctx := notifFixture.Given(t)
	ctx.When().
		SetParamInNotificationConfigMap("trigger.on-created", "- description: Application is created.\n  oncePer: app.metadata.name\n  send:\n  - app-created\n  when: \"true\"\n").
		Then().Triggers(func(triggers *notification.Triggers, err error) {
		assert.Nil(t, err)
		assert.Equal(t, []string{"on-created"}, triggers.Triggers)
	})
}
