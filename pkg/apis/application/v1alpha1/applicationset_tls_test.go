package v1alpha1

import (
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

func TestPullRequestGeneratorGitLabTLSClientCertSecretRef(t *testing.T) {
	t.Parallel()
	original := &PullRequestGeneratorGitLab{Project: "42", TLSClientCertSecretRef: &corev1.LocalObjectReference{Name: "gitlab-client-tls"}}
	data, err := proto.Marshal(original)
	require.NoError(t, err)
	var decoded PullRequestGeneratorGitLab
	require.NoError(t, proto.Unmarshal(data, &decoded))
	assert.Equal(t, original, &decoded)
	copied := original.DeepCopy()
	copied.TLSClientCertSecretRef.Name = "renewed"
	assert.Equal(t, "gitlab-client-tls", original.TLSClientCertSecretRef.Name)
	definitions := GetOpenAPIDefinitions(func(path string) spec.Ref { return spec.MustCreateRef("#/definitions/" + path) })
	schema := definitions["github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1.PullRequestGeneratorGitLab"]
	property, ok := schema.Schema.Properties["tlsClientCertSecretRef"]
	require.True(t, ok)
	assert.Equal(t, "#/definitions/k8s.io/api/core/v1.LocalObjectReference", property.Ref.String())
	assert.Contains(t, schema.Dependencies, "k8s.io/api/core/v1.LocalObjectReference")
}
