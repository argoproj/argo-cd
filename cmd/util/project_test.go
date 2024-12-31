package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func TestProjectOpts_ResourceLists(t *testing.T) {
	opts := ProjectOpts{
		allowedNamespacedResources: []string{"ConfigMap"},
		deniedNamespacedResources:  []string{"apps/DaemonSet"},
		allowedClusterResources:    []string{"apiextensions.k8s.io/CustomResourceDefinition"},
		deniedClusterResources:     []string{"rbac.authorization.k8s.io/ClusterRole"},
	}

	assert.ElementsMatch(t,
		[]metav1.GroupKind{{Kind: "ConfigMap"}}, opts.GetAllowedNamespacedResources(),
		[]metav1.GroupKind{{Group: "apps", Kind: "DaemonSet"}}, opts.GetDeniedNamespacedResources(),
		[]metav1.GroupKind{{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition"}}, opts.GetAllowedClusterResources(),
		[]metav1.GroupKind{{Group: "rbac.authorization.k8s.io", Kind: "ClusterRole"}}, opts.GetDeniedClusterResources(),
	)
}

func TestProjectOpts_GetDestinationServiceAccounts(t *testing.T) {
	opts := ProjectOpts{
		destinationServiceAccounts: []string{
			"https://192.168.99.100:8443,test-ns,test-sa",
			"https://kubernetes.default.svc.local:6443,guestbook,guestbook-sa",
		},
	}

	assert.ElementsMatch(t,
		[]v1alpha1.ApplicationDestinationServiceAccount{
			{
				Server:                "https://192.168.99.100:8443",
				Namespace:             "test-ns",
				DefaultServiceAccount: "test-sa",
			},
			{
				Server:                "https://kubernetes.default.svc.local:6443",
				Namespace:             "guestbook",
				DefaultServiceAccount: "guestbook-sa",
			},
		}, opts.GetDestinationServiceAccounts(),
	)
}
