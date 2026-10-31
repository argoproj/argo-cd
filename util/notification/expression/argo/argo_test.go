package argo

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNewExprs(t *testing.T) {
	t.Parallel()

	exprs := NewExprs(&unstructured.Unstructured{})

	require.Len(t, exprs, 4)
	for _, name := range []string{
		"StatusCard",
		"HeaderPanel",
		"SummaryPanel",
		"ApplicationLink",
	} {
		require.Contains(t, exprs, name)
	}
}

func TestStatusCard(t *testing.T) {
	t.Parallel()

	app := testApplication(map[string]any{
		"status":   "Synced",
		"revision": "abc123",
	})
	app.Object["status"].(map[string]any)["operationState"] = map[string]any{
		"phase": "Succeeded",
	}

	exprs := NewExprs(app)

	statusCard, ok := exprs["StatusCard"].(func(string) string)
	require.True(t, ok)

	actual := statusCard("https://argocd.example.com/")

	require.JSONEq(t, `{
		"type": "AdaptiveCard",
		"version": "1.4",
		"body": [
			{
				"type": "TextBlock",
				"text": "✅ Deployment succeeded",
				"size": "Large",
				"weight": "Bolder",
				"wrap": true
			},
			{
				"type": "FactSet",
				"facts": [
					{"title": "Application", "value": "guestbook"},
					{"title": "Destination Namespace", "value": "guestbook"},
					{"title": "Sync Status", "value": "Synced"},
					{"title": "Health Status", "value": "Healthy"},
					{"title": "Revision", "value": "abc123"}
				]
			}
		],
		"actions": [
			{
				"type": "Action.OpenUrl",
				"title": "View in Argo CD",
				"url": "https://argocd.example.com/applications/guestbook"
			}
		]
	}`, actual)
}

func TestStatusCardWithoutArgoCDURL(t *testing.T) {
	t.Parallel()

	app := testApplication(map[string]any{
		"status":   "Synced",
		"revision": "abc123",
	})
	app.Object["status"].(map[string]any)["operationState"] = map[string]any{
		"phase": "Succeeded",
	}

	exprs := NewExprs(app)
	statusCard, ok := exprs["StatusCard"].(func(string) string)
	require.True(t, ok)

	var card adaptiveCard
	require.NoError(t, json.Unmarshal([]byte(statusCard("")), &card))
	require.Empty(t, card.Actions)
}

func TestHeaderPanel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		phase    string
		sync     string
		health   string
		expected string
	}{
		{
			name:     "sync failed",
			phase:    "Failed",
			sync:     "OutOfSync",
			health:   "Healthy",
			expected: "❌ Sync failed",
		},
		{
			name:     "sync error",
			phase:    "Error",
			sync:     "Unknown",
			health:   "Healthy",
			expected: "❌ Sync failed",
		},
		{
			name:     "application degraded",
			phase:    "Running",
			sync:     "OutOfSync",
			health:   "Degraded",
			expected: "🔴 Application degraded",
		},
		{
			name:     "operation running",
			phase:    "Running",
			sync:     "OutOfSync",
			health:   "Healthy",
			expected: "⏳ Deployment progressing",
		},
		{
			name:     "health progressing",
			sync:     "OutOfSync",
			health:   "Progressing",
			expected: "⏳ Deployment progressing",
		},
		{
			name:     "deployment succeeded",
			phase:    "Succeeded",
			sync:     "Synced",
			health:   "Healthy",
			expected: "✅ Deployment succeeded",
		},
		{
			name:     "application out of sync",
			sync:     "OutOfSync",
			health:   "Healthy",
			expected: "🔄 Application out of sync",
		},
		{
			name:     "sync status unknown",
			sync:     "Unknown",
			health:   "Healthy",
			expected: "⚠️ Sync status unknown",
		},
		{
			name:     "default",
			health:   "Healthy",
			expected: "Argo CD application status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			app := testApplication(map[string]any{
				"status":   tt.sync,
				"revision": "abc123",
			})

			status := app.Object["status"].(map[string]any)
			status["health"] = map[string]any{
				"status": tt.health,
			}
			if tt.phase != "" {
				status["operationState"] = map[string]any{
					"phase": tt.phase,
				}
			}

			exprs := NewExprs(app)
			headerPanel, ok := exprs["HeaderPanel"].(func() string)
			require.True(t, ok)

			var panel adaptiveCardElement
			require.NoError(t, json.Unmarshal([]byte(headerPanel()), &panel))
			require.Equal(t, tt.expected, panel.Text)
		})
	}
}

func TestSummaryPanel(t *testing.T) {
	t.Parallel()

	exprs := NewExprs(testApplication(map[string]any{
		"status":   "Synced",
		"revision": "abc123",
	}))

	summaryPanel, ok := exprs["SummaryPanel"].(func() string)
	require.True(t, ok)

	require.JSONEq(t, `{
		"type": "FactSet",
		"facts": [
			{"title": "Application", "value": "guestbook"},
			{"title": "Destination Namespace", "value": "guestbook"},
			{"title": "Sync Status", "value": "Synced"},
			{"title": "Health Status", "value": "Healthy"},
			{"title": "Revision", "value": "abc123"}
		]
	}`, summaryPanel())
}

func TestSummaryPanelMultiSource(t *testing.T) {
	t.Parallel()

	exprs := NewExprs(testApplication(map[string]any{
		"status": "Synced",
		"revisions": []any{
			"abc123",
			"def456",
		},
	}))

	summaryPanel, ok := exprs["SummaryPanel"].(func() string)
	require.True(t, ok)

	require.JSONEq(t, `{
		"type": "FactSet",
		"facts": [
			{"title": "Application", "value": "guestbook"},
			{"title": "Destination Namespace", "value": "guestbook"},
			{"title": "Sync Status", "value": "Synced"},
			{"title": "Health Status", "value": "Healthy"},
			{"title": "Revisions", "value": "abc123, def456"}
		]
	}`, summaryPanel())
}

func TestApplicationLink(t *testing.T) {
	t.Parallel()

	exprs := NewExprs(testApplication(nil))

	applicationLink, ok := exprs["ApplicationLink"].(func(string) string)
	require.True(t, ok)

	require.JSONEq(t, `{
		"type": "Action.OpenUrl",
		"title": "View in Argo CD",
		"url": "https://argocd.example.com/applications/guestbook"
	}`, applicationLink("https://argocd.example.com/"))
}

func TestApplicationLinkWithApplicationNamespace(t *testing.T) {
	t.Parallel()

	app := testApplication(nil)
	app.SetNamespace("team-a")

	exprs := NewExprs(app)
	applicationLink, ok := exprs["ApplicationLink"].(func(string) string)
	require.True(t, ok)

	require.JSONEq(t, `{
		"type": "Action.OpenUrl",
		"title": "View in Argo CD",
		"url": "https://argocd.example.com/applications/team-a/guestbook"
	}`, applicationLink("https://argocd.example.com/"))
}

func TestApplicationLinkWithoutArgoCDURL(t *testing.T) {
	t.Parallel()

	exprs := NewExprs(testApplication(nil))

	applicationLink, ok := exprs["ApplicationLink"].(func(string) string)
	require.True(t, ok)

	require.Empty(t, applicationLink(""))
}

func testApplication(syncStatus map[string]any) *unstructured.Unstructured {
	status := map[string]any{
		"health": map[string]any{
			"status": "Healthy",
		},
	}

	if syncStatus != nil {
		status["sync"] = syncStatus
	}

	return &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name": "guestbook",
			},
			"spec": map[string]any{
				"destination": map[string]any{
					"namespace": "guestbook",
				},
			},
			"status": status,
		},
	}
}
