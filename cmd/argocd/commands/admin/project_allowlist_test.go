package admin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestProjectAllowListGen(t *testing.T) {
	res := metav1.APIResource{
		Name: "services",
		Kind: "Service",
	}
	resourceList := []*metav1.APIResourceList{{APIResources: []metav1.APIResource{res}}}

	globalProj, err := generateProjectAllowList(resourceList, "testdata/test_clusterrole.yaml", "testproj")
	assert.NoError(t, err)
	assert.True(t, len(globalProj.Spec.NamespaceResourceWhitelist) > 0)
}
