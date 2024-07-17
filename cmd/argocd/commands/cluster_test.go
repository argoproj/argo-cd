package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func Test_getQueryBySelector(t *testing.T) {
	query := getQueryBySelector("my-cluster")
	assert.Equal(t, "my-cluster", query.Name)
	assert.Equal(t, "", query.Server)

	query = getQueryBySelector("http://my-server")
	assert.Equal(t, "", query.Name)
	assert.Equal(t, "http://my-server", query.Server)

	query = getQueryBySelector("https://my-server")
	assert.Equal(t, "", query.Name)
	assert.Equal(t, "https://my-server", query.Server)
}

func Test_printClusterTable(t *testing.T) {
	printClusterTable([]v1alpha1.Cluster{
		{
			Server: "my-server",
			Name:   "my-name",
			Config: v1alpha1.ClusterConfig{
				Username:        "my-username",
				Password:        "my-password",
				BearerToken:     "my-bearer-token",
				TLSClientConfig: v1alpha1.TLSClientConfig{},
				AWSAuthConfig:   nil,
			},
			ConnectionState: v1alpha1.ConnectionState{
				Status:     "my-status",
				Message:    "my-message",
				ModifiedAt: &metav1.Time{},
			},
			ServerVersion: "my-version",
		},
	})
}

func Test_getRestConfig(t *testing.T) {
	type args struct {
		pathOpts *clientcmd.PathOptions
		ctxName  string
	}
	pathOpts := &clientcmd.PathOptions{
		GlobalFile:   "./testdata/config",
		LoadingRules: clientcmd.NewDefaultClientConfigLoadingRules(),
	}
	tests := []struct {
		name        string
		args        args
		expected    *rest.Config
		wantErr     bool
		expectedErr string
	}{
		{
			"Load config for context successfully",
			args{
				pathOpts,
				"argocd2.example.com:443",
			},
			&rest.Config{Host: "argocd2.example.com:443"},
			false,
			"",
		},
		{
			"Load config for current-context successfully",
			args{
				pathOpts,
				"localhost:8080",
			},
			&rest.Config{Host: "localhost:8080"},
			false,
			"",
		},
		{
			"Context not found",
			args{
				pathOpts,
				"not-exist",
			},
			nil,
			true,
			"Context not-exist does not exist in kubeconfig",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := getRestConfig(tt.args.pathOpts, tt.args.ctxName); err == nil {
				require.Equal(t, tt.expected, got)
			} else if tt.wantErr {
				require.Equal(t, tt.expectedErr, err.Error())
			} else {
				t.Errorf("An unexpected error occurred during test %s:\n%s", tt.name, err.Error())
			}
		})
	}
}
