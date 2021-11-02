package plugin

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v2/cmpserver/apiclient"
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

	q := apiclient.RepositoryRequest{}
	path, err := os.Getwd()
	require.NoError(t, err)
	q.Path = path

	res1, err := service.MatchRepository(context.Background(), &q)
	require.NoError(t, err)
	require.True(t, res1.IsSupported)
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

	q := apiclient.ManifestRequest{}
	res1, err := service.GenerateManifest(context.Background(), &q)
	require.NoError(t, err)
	require.NotNil(t, res1)

	expectedOutput := "{\"apiVersion\":\"v1\",\"data\":{\"foo\":\"bar\"},\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"my-map\"}}"
	if res1 != nil {
		require.Equal(t, expectedOutput, res1.Manifests[0])
	}
}
