package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/argoproj/notifications-engine/pkg/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestCatalogTeamsWorkflows(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("../../notifications_catalog/templates/*.yaml")
	require.NoError(t, err)
	require.NotEmpty(t, files)
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile(file)
			require.NoError(t, err)
			var notification services.Notification
			require.NoError(t, yaml.Unmarshal(data, &notification))
			assert.Nil(t, notification.Teams)
			require.NotNil(t, notification.TeamsWorkflows)
			assert.NotEmpty(t, notification.TeamsWorkflows.Title)
		})
	}
}
