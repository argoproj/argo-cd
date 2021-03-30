package util

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func Test_newCluster(t *testing.T) {
	clusterWithData := NewCluster("test-cluster", []string{"test-namespace"}, &rest.Config{
		TLSClientConfig: rest.TLSClientConfig{
			Insecure:   false,
			ServerName: "test-endpoint.example.com",
			CAData:     []byte("test-ca-data"),
			CertData:   []byte("test-cert-data"),
			KeyData:    []byte("test-key-data"),
		},
		Host: "test-endpoint.example.com",
	},
		"test-bearer-token",
		&v1alpha1.AWSAuthConfig{},
		&v1alpha1.ExecProviderConfig{})

	assert.Equal(t, "test-cert-data", string(clusterWithData.Config.CertData))
	assert.Equal(t, "test-key-data", string(clusterWithData.Config.KeyData))
	assert.Equal(t, "", clusterWithData.Config.BearerToken)

	clusterWithFiles := NewCluster("test-cluster", []string{"test-namespace"}, &rest.Config{
		TLSClientConfig: rest.TLSClientConfig{
			Insecure:   false,
			ServerName: "test-endpoint.example.com",
			CAData:     []byte("test-ca-data"),
			CertFile:   "./testdata/test.cert.pem",
			KeyFile:    "./testdata/test.key.pem",
		},
		Host: "test-endpoint.example.com",
	},
		"test-bearer-token",
		&v1alpha1.AWSAuthConfig{},
		&v1alpha1.ExecProviderConfig{})

	assert.True(t, strings.Contains(string(clusterWithFiles.Config.CertData), "test-cert-data"))
	assert.True(t, strings.Contains(string(clusterWithFiles.Config.KeyData), "test-key-data"))
	assert.Equal(t, "", clusterWithFiles.Config.BearerToken)

	clusterWithBearerToken := NewCluster("test-cluster", []string{"test-namespace"}, &rest.Config{
		TLSClientConfig: rest.TLSClientConfig{
			Insecure:   false,
			ServerName: "test-endpoint.example.com",
			CAData:     []byte("test-ca-data"),
		},
		Host: "test-endpoint.example.com",
	},
		"test-bearer-token",
		&v1alpha1.AWSAuthConfig{},
		&v1alpha1.ExecProviderConfig{})

	assert.Equal(t, "test-bearer-token", clusterWithBearerToken.Config.BearerToken)
}
