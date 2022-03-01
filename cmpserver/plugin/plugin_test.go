package plugin

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newService(configFilePath string) (*Service, error) {
	config, err := ReadPluginConfig(configFilePath)
	if err != nil {
		return nil, err
	}

	initConstants := CMPServerInitConstants{
		PluginConfig: *config,
	}

	service := &Service{
		initConstants: initConstants,
	}
	return service, nil
}

func TestMatchRepository(t *testing.T) {
	configFilePath := "./testdata/ksonnet/config"
	service, err := newService(configFilePath)
	require.NoError(t, err)

	path, err := os.Getwd()
	require.NoError(t, err)

	isSupported, err := service.matchRepository(context.Background(), path)
	require.NoError(t, err)
	require.True(t, isSupported)
}

func Test_Negative_ConfigFile_DoesnotExist(t *testing.T) {
	configFilePath := "./testdata/kustomize-neg/config"
	service, err := newService(configFilePath)
	require.Error(t, err)
	require.Nil(t, service)
}

func TestGenerateManifest(t *testing.T) {
	configFilePath := "./testdata/kustomize/config"
	service, err := newService(configFilePath)
	require.NoError(t, err)

	res1, err := service.generateManifest(context.Background(), "", nil)
	require.NoError(t, err)
	require.NotNil(t, res1)

	expectedOutput := "{\"apiVersion\":\"v1\",\"data\":{\"foo\":\"bar\"},\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"my-map\"}}"
	if res1 != nil {
		require.Equal(t, expectedOutput, res1.Manifests[0])
	}
}

// TestRunCommandContextTimeout makes sure the command dies at timeout rather than sleeping past the timeout.
func TestRunCommandContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 990*time.Millisecond)
	defer cancel()
	// Use a subshell so there's a child command.
	command := Command{
		Command: []string{"sh", "-c"},
		Args:    []string{"sleep 5"},
	}
	before := time.Now()
	_, err := runCommand(ctx, command, "", []string{})
	after := time.Now()
	assert.Error(t, err) // The command should time out, causing an error.
	assert.Less(t, after.Sub(before), 1*time.Second)
}
