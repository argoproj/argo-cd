package discovery

import (
	"context"
	"testing"

	"github.com/argoproj/argo-cd/v2/cmpserver/apiclient"
	cmpmocks "github.com/argoproj/argo-cd/v2/cmpserver/apiclient/mocks"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/app/discovery/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDiscover(t *testing.T) {
	apps, err := Discover(context.Background(), "./testdata", "./testdata", map[string]bool{}, []string{}, []string{})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"foo": "Kustomize",
		"baz": "Helm",
	}, apps)
}

func TestAppType(t *testing.T) {
	appType, err := AppType(context.Background(), "./testdata/foo", "./testdata", map[string]bool{}, []string{}, []string{})
	require.NoError(t, err)
	assert.Equal(t, "Kustomize", appType)

	appType, err = AppType(context.Background(), "./testdata/baz", "./testdata", map[string]bool{}, []string{}, []string{})
	require.NoError(t, err)
	assert.Equal(t, "Helm", appType)

	appType, err = AppType(context.Background(), "./testdata", "./testdata", map[string]bool{}, []string{}, []string{})
	require.NoError(t, err)
	assert.Equal(t, "Directory", appType)
}

func TestAppType_Disabled(t *testing.T) {
	enableManifestGeneration := map[string]bool{
		string(v1alpha1.ApplicationSourceTypeKustomize): false,
		string(v1alpha1.ApplicationSourceTypeHelm):      false,
	}
	appType, err := AppType(context.Background(), "./testdata/foo", "./testdata", enableManifestGeneration, []string{}, []string{})
	require.NoError(t, err)
	assert.Equal(t, "Directory", appType)

	appType, err = AppType(context.Background(), "./testdata/baz", "./testdata", enableManifestGeneration, []string{}, []string{})
	require.NoError(t, err)
	assert.Equal(t, "Directory", appType)

	appType, err = AppType(context.Background(), "./testdata", "./testdata", enableManifestGeneration, []string{}, []string{})
	require.NoError(t, err)
	assert.Equal(t, "Directory", appType)
}

type nopCloser struct{}

func (c nopCloser) Close() error {
	return nil
}

func Test_cmpSupportsForClient(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                  string
		namedPlugin           bool
		isDiscoveryConfigured bool
		isSupported           bool
		expected              bool
	}{
		{
			name:                  "named plugin, no discovery",
			namedPlugin:           true,
			isDiscoveryConfigured: false,
			expected:              true,
		},
		{
			name:                  "discovery configured, repo not supported, not named plugin",
			namedPlugin:           false,
			isDiscoveryConfigured: true,
			isSupported:           false,
			expected:              false,
		},
		{
			// If it's a named plugin and discovery is configured, we want the discovery rules to apply.
			name:                  "discovery configured, repo not supported, named plugin",
			namedPlugin:           true,
			isDiscoveryConfigured: true,
			isSupported:           false,
			expected:              false,
		},
		{
			name:                  "plugin not named and discovery not configured",
			namedPlugin:           false,
			isDiscoveryConfigured: false,
			expected:              false,
		},
		{
			name:                  "discovery configured, not named plugin",
			namedPlugin:           false,
			isDiscoveryConfigured: true,
			isSupported:           true,
			expected:              true,
		},
		{
			name:                  "discovery configured and named plugin",
			namedPlugin:           true,
			isDiscoveryConfigured: true,
			isSupported:           true,
			expected:              true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rc := cmpmocks.NewConfigManagementPluginService_MatchRepositoryClient(t)
			rc.On("Send", mock.Anything).Maybe().Return(nil)
			rc.On("CloseAndRecv").Maybe().Return(&apiclient.RepositoryResponse{
				IsSupported: tc.isSupported,
			}, nil)

			c := cmpmocks.NewConfigManagementPluginServiceClient(t)
			c.On("CheckPluginConfiguration", mock.Anything, mock.Anything, mock.Anything).Return(&apiclient.CheckPluginConfigurationResponse{
				IsDiscoveryConfigured: tc.isDiscoveryConfigured,
			}, nil, nil)
			c.On("MatchRepository", mock.Anything, mock.Anything).Maybe().Return(rc, nil)

			cc := mocks.NewCMPClientConstructor(t)
			cc.On("NewConfigManagementPluginClient").Return(c, nopCloser{}, nil)

			var client apiclient.ConfigManagementPluginServiceClient
			if tc.namedPlugin {
				client, _ = namedCMPSupports(context.Background(), cc, "./testdata", "./testdata", nil, nil)
			} else {
				client, _ = unnamedCMPSupports(context.Background(), cc, "./testdata", "./testdata", nil, nil)
			}
			if tc.expected {
				assert.NotNil(t, client)
			} else {
				assert.Nil(t, client)
			}
		})
	}
}
