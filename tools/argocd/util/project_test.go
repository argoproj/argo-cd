package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestProjectOpts_ResourceLists(t *testing.T) {
	opts := ProjectOpts{
		allowedNamespacedResources: []string{"ConfigMap"},
		deniedNamespacedResources:  []string{"apps/DaemonSet"},
		allowedClusterResources:    []string{"apiextensions.k8s.io/CustomResourceDefinition"},
		deniedClusterResources:     []string{"rbac.authorization.k8s.io/ClusterRole"},
	}

	assert.ElementsMatch(t,
		[]v1.GroupKind{{Kind: "ConfigMap"}}, opts.GetAllowedNamespacedResources(),
		[]v1.GroupKind{{Group: "apps", Kind: "DaemonSet"}}, opts.GetDeniedNamespacedResources(),
		[]v1.GroupKind{{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition"}}, opts.GetAllowedClusterResources(),
		[]v1.GroupKind{{Group: "rbac.authorization.k8s.io", Kind: "ClusterRole"}}, opts.GetDeniedClusterResources(),
	)
}
