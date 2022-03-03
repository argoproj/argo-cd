package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func Test_getQueryBySelector(t *testing.T) {
	query := getQueryBySelector("my-cluster")
	assert.Equal(t, query.Name, "my-cluster")
	assert.Equal(t, query.Server, "")

	query = getQueryBySelector("http://my-server")
	assert.Equal(t, query.Name, "")
	assert.Equal(t, query.Server, "http://my-server")

	query = getQueryBySelector("https://my-server")
	assert.Equal(t, query.Name, "")
	assert.Equal(t, query.Server, "https://my-server")
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
