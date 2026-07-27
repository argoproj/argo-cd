package util

import (
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v3/util/test"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestProjectOpts_ResourceLists(t *testing.T) {
	t.Parallel()
	opts := ProjectOpts{
		allowedNamespacedResources: []string{"ConfigMap"},
		deniedNamespacedResources:  []string{"apps/DaemonSet"},
		allowedClusterResources:    []string{"apiextensions.k8s.io/CustomResourceDefinition"},
		deniedClusterResources:     []string{"rbac.authorization.k8s.io/ClusterRole"},
	}

	assert.ElementsMatch(t, []metav1.GroupKind{{Kind: "ConfigMap"}}, opts.GetAllowedNamespacedResources())
	assert.ElementsMatch(t, []metav1.GroupKind{{Group: "apps", Kind: "DaemonSet"}}, opts.GetDeniedNamespacedResources())
	assert.ElementsMatch(t, []v1alpha1.ClusterResourceRestrictionItem{{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition"}}, opts.GetAllowedClusterResources())
	assert.ElementsMatch(t, []v1alpha1.ClusterResourceRestrictionItem{{Group: "rbac.authorization.k8s.io", Kind: "ClusterRole"}}, opts.GetDeniedClusterResources())
}

func TestProjectOpts_GetDestinationServiceAccounts(t *testing.T) {
	t.Parallel()
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

// TODO: Remove deprecated https://github.com/argoproj/argo-cd/issues/27695
func TestProjectOpts_SetProjSpecOptions_SignatureKeysWarns(t *testing.T) {
	spec := &v1alpha1.AppProjectSpec{}
	opts := ProjectOpts{}

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.StringSliceVar(&opts.SignatureKeys, "signature-keys", []string{}, "")
	require.NoError(t, flags.Set("signature-keys", "1234ABCD1234ABCD"))

	hook := test.NewLogHook(log.WarnLevel)
	log.AddHook(hook)
	t.Cleanup(hook.CleanupHook)

	visited := SetProjSpecOptions(flags, spec, &opts)

	assert.Equal(t, 1, visited)
	assert.Equal(t, []v1alpha1.SignatureKey{{KeyID: "1234ABCD1234ABCD"}}, spec.SignatureKeys) // nolint:staticcheck
	assert.Equal(t, []string{"Warning: SignatureKeys are deprecated. Use SourceIntegrity instead."}, hook.GetEntries())
}
