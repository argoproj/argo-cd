package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clustercache "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/cache"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
)

func TestCustomParentResourceTree(t *testing.T) {
	ctx := Given(t)
	ctx.Path("custom-parent").When().CreateApp().Sync().Then().
		Expect(SyncStatusIs(appv1.SyncStatusCodeSynced)).
		When().And(func() {
		// This represents an operator-created resource, not a manifest managed by Argo CD.
		_, err := fixture.KubeClientset.CoreV1().ConfigMaps(ctx.DeploymentNamespace()).Create(t.Context(), &corev1.ConfigMap{
			Name: "custom-child",
			Annotations: map[string]string{
				clustercache.AnnotationKeyParentAPIVersion: "v1",
				clustercache.AnnotationKeyParentKind:       "ConfigMap",
				clustercache.AnnotationKeyParentName:       "custom-parent",
			},
		}, metav1.CreateOptions{})
		require.NoError(t, err)
	}).Then().Expect(ResourceTreeNode(func(node appv1.ResourceNode) bool {
		if node.Name != "custom-child" {
			return false
		}
		for _, parent := range node.ParentRefs {
			if parent.Kind == "ConfigMap" && parent.Name == "custom-parent" && parent.Namespace == ctx.DeploymentNamespace() && parent.UID != "" {
				return true
			}
		}
		return false
	})).Expect(SyncStatusIs(appv1.SyncStatusCodeSynced)).
		When().Sync("--prune").Then().And(func(_ *appv1.Application) {
		child, err := fixture.KubeClientset.CoreV1().ConfigMaps(ctx.DeploymentNamespace()).Get(t.Context(), "custom-child", metav1.GetOptions{})
		require.NoError(t, err, "a custom display relationship must not make the child prunable")
		assert.Empty(t, child.OwnerReferences, "Argo CD must not write synthetic Kubernetes ownership")
		child.Annotations = nil
		_, err = fixture.KubeClientset.CoreV1().ConfigMaps(ctx.DeploymentNamespace()).Update(t.Context(), child, metav1.UpdateOptions{})
		require.NoError(t, err)
	}).When().Refresh(appv1.RefreshTypeNormal).Then().
		Expect(NotResourceTreeNode(func(node appv1.ResourceNode) bool { return node.Name == "custom-child" }))
}
