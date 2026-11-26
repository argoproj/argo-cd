package commands

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	applicationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	applicationmocks "github.com/argoproj/argo-cd/v3/pkg/apiclient/application/mocks"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestParseParamArgs(t *testing.T) {
	tests := []struct {
		name       string
		params     []string
		wantNames  []string
		wantValues []string
		wantErr    bool
	}{
		// --- nil / empty input ---
		{
			name:       "nil slice returns empty result",
			params:     nil,
			wantNames:  []string{},
			wantValues: []string{},
		},
		{
			name:       "empty slice returns empty result",
			params:     []string{},
			wantNames:  []string{},
			wantValues: []string{},
		},

		// --- valid happy-path cases ---
		{
			name:       "single valid param",
			params:     []string{"replica-count=3"},
			wantNames:  []string{"replica-count"},
			wantValues: []string{"3"},
		},
		{
			name:       "multiple valid params",
			params:     []string{"key1=value1", "key2=value2"},
			wantNames:  []string{"key1", "key2"},
			wantValues: []string{"value1", "value2"},
		},
		{
			name:       "value containing equals sign is preserved",
			params:     []string{"url=https://example.com?foo=bar"},
			wantNames:  []string{"url"},
			wantValues: []string{"https://example.com?foo=bar"},
		},
		{
			name:       "empty value is allowed",
			params:     []string{"key="},
			wantNames:  []string{"key"},
			wantValues: []string{""},
		},
		{
			name:       "numeric value",
			params:     []string{"replicas=0"},
			wantNames:  []string{"replicas"},
			wantValues: []string{"0"},
		},
		{
			name:       "boolean string value",
			params:     []string{"enabled=false"},
			wantNames:  []string{"enabled"},
			wantValues: []string{"false"},
		},
		{
			name:       "json value preserves structure",
			params:     []string{`config={"key":"value"}`},
			wantNames:  []string{"config"},
			wantValues: []string{`{"key":"value"}`},
		},
		{
			name:       "value with spaces is preserved",
			params:     []string{"message=hello world"},
			wantNames:  []string{"message"},
			wantValues: []string{"hello world"},
		},
		{
			name:       "unicode key and value",
			params:     []string{"名前=値"},
			wantNames:  []string{"名前"},
			wantValues: []string{"値"},
		},
		{
			name:    "duplicate keys returns error",
			params:  []string{"key=first", "key=second"},
			wantErr: true,
		},
		{
			name:       "kubernetes style dotted key",
			params:     []string{"app.kubernetes.io/name=myapp"},
			wantNames:  []string{"app.kubernetes.io/name"},
			wantValues: []string{"myapp"},
		},
		{
			name:       "key with surrounding whitespace is trimmed",
			params:     []string{" key =value"},
			wantNames:  []string{"key"},
			wantValues: []string{"value"},
		},
		{
			name:       "space after equals is preserved in value",
			params:     []string{"key= value"},
			wantNames:  []string{"key"},
			wantValues: []string{" value"},
		},
		{
			name:       "value with equals signs only",
			params:     []string{"key===="},
			wantNames:  []string{"key"},
			wantValues: []string{"==="},
		},

		// --- error cases ---
		{
			name:    "missing equals sign returns error",
			params:  []string{"invalidparam"},
			wantErr: true,
		},
		{
			name:    "empty string returns error",
			params:  []string{""},
			wantErr: true,
		},
		{
			name:    "empty key with value returns error",
			params:  []string{"=value"},
			wantErr: true,
		},
		{
			name:    "only equals sign returns error",
			params:  []string{"="},
			wantErr: true,
		},
		{
			name:    "whitespace-only key returns error",
			params:  []string{"  =value"},
			wantErr: true,
		},
		{
			name:    "first invalid in mixed list returns error",
			params:  []string{"key1=value1", "badparam", "key2=value2"},
			wantErr: true,
		},
		{
			name:    "invalid entry after valid ones returns error",
			params:  []string{"key1=value1", "key2=value2", "noequalssign"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseParamArgs(tt.params)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, got, len(tt.wantNames))
			for i, p := range got {
				assert.Equal(t, tt.wantNames[i], *p.Name)
				assert.Equal(t, tt.wantValues[i], *p.Value)
			}
		})
	}
}

func TestGetActionableResourcesForApplication(t *testing.T) {
	appName := "my-app"
	appNs := "argocd"

	t.Run("returns managed resources plus the Application resource itself", func(t *testing.T) {
		mockClient := applicationmocks.NewApplicationServiceClient(t)
		mockClient.On("ManagedResources", mock.Anything, mock.Anything).
			Return(&applicationpkg.ManagedResourcesResponse{
				Items: []*v1alpha1.ResourceDiff{
					{Group: "apps", Kind: "Deployment", Namespace: "default", Name: "my-deploy"},
				},
			}, nil)
		mockClient.On("Get", mock.Anything, mock.Anything).
			Return(&v1alpha1.Application{
				Name:      appName,
				Namespace: appNs,
			}, nil)

		result, err := getActionableResourcesForApplication(context.Background(), mockClient, &appNs, &appName)
		require.NoError(t, err)
		// one managed resource + the Application appended at the end
		require.Len(t, result, 2)
		assert.Equal(t, "Deployment", result[0].Kind)
		assert.Equal(t, "Application", result[1].Kind)
		assert.Equal(t, appName, result[1].Name)
	})

	t.Run("returns only the Application resource when no managed resources exist", func(t *testing.T) {
		mockClient := applicationmocks.NewApplicationServiceClient(t)
		mockClient.On("ManagedResources", mock.Anything, mock.Anything).
			Return(&applicationpkg.ManagedResourcesResponse{Items: nil}, nil)
		mockClient.On("Get", mock.Anything, mock.Anything).
			Return(&v1alpha1.Application{
				Name:      appName,
				Namespace: appNs,
			}, nil)

		result, err := getActionableResourcesForApplication(context.Background(), mockClient, &appNs, &appName)
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "Application", result[0].Kind)
	})

	t.Run("returns error when ManagedResources fails", func(t *testing.T) {
		mockClient := applicationmocks.NewApplicationServiceClient(t)
		mockClient.On("ManagedResources", mock.Anything, mock.Anything).
			Return(nil, errors.New("connection refused"))

		_, err := getActionableResourcesForApplication(context.Background(), mockClient, &appNs, &appName)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "connection refused")
	})

	t.Run("returns error when Get fails", func(t *testing.T) {
		mockClient := applicationmocks.NewApplicationServiceClient(t)
		mockClient.On("ManagedResources", mock.Anything, mock.Anything).
			Return(&applicationpkg.ManagedResourcesResponse{Items: nil}, nil)
		mockClient.On("Get", mock.Anything, mock.Anything).
			Return(nil, errors.New("application not found"))

		_, err := getActionableResourcesForApplication(context.Background(), mockClient, &appNs, &appName)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "application not found")
	})
}
