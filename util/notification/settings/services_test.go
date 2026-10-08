package settings

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/argoproj/notifications-engine/pkg/api"
	"github.com/argoproj/notifications-engine/pkg/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestRemoveLegacyTeamsServices(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		var payload struct {
			Type        string `json:"type"`
			Attachments []struct {
				ContentType string         `json:"contentType"`
				Content     map[string]any `json:"content"`
			} `json:"attachments"`
		}
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		assert.Equal(t, "message", payload.Type)
		if assert.Len(t, payload.Attachments, 1) {
			assert.Equal(t, "application/vnd.microsoft.card.adaptive", payload.Attachments[0].ContentType)
			assert.Equal(t, "AdaptiveCard", payload.Attachments[0].Content["type"])
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	opts := fmt.Sprintf("recipientUrls:\n  channel: %s/powerautomate/test", server.URL)
	cm := &corev1.ConfigMap{Data: map[string]string{
		"service.teams":                    "recipientUrls: {}",
		"service.teams.legacy":             "recipientUrls: {}",
		"service.teams-workflows":          opts,
		"service.teams-workflows.workflow": opts,
		"service.webhook.test":             "url: https://example.com",
		"template.test":                    "teams:\n  title: '{{ invalid'\nteams-workflows:\n  title: Migrated notification",
	}}
	cfg, err := api.ParseConfig(cm, &corev1.Secret{})
	require.NoError(t, err)
	original := cm.DeepCopy()
	_, err = getContext(cfg, cm, &corev1.Secret{})
	require.NoError(t, err)
	assert.Equal(t, original, cm)
	assert.Nil(t, cfg.Templates["test"].Teams)
	require.NotNil(t, cfg.Templates["test"].TeamsWorkflows)
	notificationAPI, err := api.NewAPI(*cfg, func(map[string]any, services.Destination) map[string]any { return nil })
	require.NoError(t, err)
	for _, name := range []string{"teams", "legacy"} {
		assert.NotContains(t, notificationAPI.GetNotificationServices(), name)
		require.ErrorContains(t, notificationAPI.Send(nil, nil, services.Destination{Service: name}), "is not supported")
	}
	assert.Zero(t, requests.Load())
	for _, name := range []string{"teams-workflows", "workflow", "test"} {
		assert.Contains(t, notificationAPI.GetNotificationServices(), name)
	}
	for _, name := range []string{"teams-workflows", "workflow"} {
		require.NoError(t, notificationAPI.Send(nil, []string{"test"}, services.Destination{Service: name, Recipient: "channel"}))
	}
	assert.EqualValues(t, 2, requests.Load())
}
