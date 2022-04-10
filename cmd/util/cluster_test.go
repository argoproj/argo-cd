package util

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func Test_newCluster(t *testing.T) {
	labels := map[string]string{"key1": "val1"}
	annotations := map[string]string{"key2": "val2"}
	clusterWithData := NewCluster("test-cluster", []string{"test-namespace"}, false, &rest.Config{
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
		&v1alpha1.ExecProviderConfig{}, labels, annotations)

	assert.Equal(t, "test-cert-data", string(clusterWithData.Config.CertData))
	assert.Equal(t, "test-key-data", string(clusterWithData.Config.KeyData))
	assert.Equal(t, "", clusterWithData.Config.BearerToken)
	assert.Equal(t, labels, clusterWithData.Labels)
	assert.Equal(t, annotations, clusterWithData.Annotations)

	clusterWithFiles := NewCluster("test-cluster", []string{"test-namespace"}, false, &rest.Config{
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
		&v1alpha1.ExecProviderConfig{}, labels, nil)

	assert.True(t, strings.Contains(string(clusterWithFiles.Config.CertData), "test-cert-data"))
	assert.True(t, strings.Contains(string(clusterWithFiles.Config.KeyData), "test-key-data"))
	assert.Equal(t, "", clusterWithFiles.Config.BearerToken)
	assert.Equal(t, labels, clusterWithFiles.Labels)
	assert.Nil(t, clusterWithFiles.Annotations)

	clusterWithBearerToken := NewCluster("test-cluster", []string{"test-namespace"}, false, &rest.Config{
		TLSClientConfig: rest.TLSClientConfig{
			Insecure:   false,
			ServerName: "test-endpoint.example.com",
			CAData:     []byte("test-ca-data"),
		},
		Host: "test-endpoint.example.com",
	},
		"test-bearer-token",
		&v1alpha1.AWSAuthConfig{},
		&v1alpha1.ExecProviderConfig{}, nil, nil)

	assert.Equal(t, "test-bearer-token", clusterWithBearerToken.Config.BearerToken)
	assert.Nil(t, clusterWithBearerToken.Labels)
	assert.Nil(t, clusterWithBearerToken.Annotations)
}
