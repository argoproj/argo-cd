package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	applicationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	clusterpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/cluster"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
	"github.com/argoproj/argo-cd/v3/util/io"
)

// TestCrossNamespaceOwnership tests that Argo CD correctly tracks parent-child relationships
// when a cluster-scoped resource (ClusterRole) owns namespaced resources (Roles) across different namespaces.
// This validates the fix for supporting cluster-scoped parents with namespaced children in resource trees.
func TestCrossNamespaceOwnership(t *testing.T) {
	var clusterRoleUID string

	Given(t).
		Path("cross-namespace-ownership").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(v1alpha1.SyncStatusCodeSynced)).
		And(func(app *v1alpha1.Application) {
			// Get the UID of the ClusterRole that was created
			output, err := Run("", "kubectl", "get", "clusterrole", "test-cluster-role",
				"-o", "jsonpath={.metadata.uid}")
			require.NoError(t, err)
			clusterRoleUID = output
			t.Logf("ClusterRole UID: %s", clusterRoleUID)
		}).
		When().
		And(func() {
			// Create a Role in the app's destination namespace with an ownerReference to the ClusterRole
			roleYaml := fmt.Sprintf(`apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: test-role-same-ns
  namespace: %s
  ownerReferences:
  - apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRole
    name: test-cluster-role
    uid: %s
rules:
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "list"]`, DeploymentNamespace(), clusterRoleUID)

			_, err := Run("", "sh", "-c", fmt.Sprintf("echo '%s' | kubectl apply -f -", roleYaml))
			require.NoError(t, err)
			t.Logf("Created Role in app namespace: %s", DeploymentNamespace())

			// Create another namespace for cross-namespace testing
			otherNamespace := fmt.Sprintf("%s-other", DeploymentNamespace())
			_, err = Run("", "kubectl", "create", "namespace", otherNamespace)
			if err != nil {
				// Namespace might already exist, that's ok
				t.Logf("Namespace %s may already exist: %v", otherNamespace, err)
			}

			// Create a Role in a different namespace with an ownerReference to the ClusterRole
			roleYaml2 := fmt.Sprintf(`apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: test-role-other-ns
  namespace: %s
  ownerReferences:
  - apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRole
    name: test-cluster-role
    uid: %s
rules:
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "list"]`, otherNamespace, clusterRoleUID)

			_, err = Run("", "sh", "-c", fmt.Sprintf("echo '%s' | kubectl apply -f -", roleYaml2))
			require.NoError(t, err)
			t.Logf("Created Role in other namespace: %s", otherNamespace)

			// Give the cache a moment to pick up the changes
			time.Sleep(2 * time.Second)

			// Invalidate the cluster cache to force rebuild of orphaned children index
			t.Log("Invalidating cluster cache to rebuild orphaned children index...")
			closer, clusterClient, err := ArgoCDClientset.NewClusterClient()
			require.NoError(t, err)
			defer io.Close(closer)

			// Invalidate cache for the default cluster (https://kubernetes.default.svc)
			cluster, err := clusterClient.InvalidateCache(context.Background(), &clusterpkg.ClusterQuery{
				Server: "https://kubernetes.default.svc",
			})
			if err != nil {
				t.Logf("Warning: Failed to invalidate cache: %v", err)
			} else {
				t.Logf("Cache invalidated successfully, cluster status: %s", cluster.ConnectionState.Status)
			}

			// Wait for cache to rebuild
			time.Sleep(3 * time.Second)
		}).
		Refresh(v1alpha1.RefreshTypeHard). // Now refresh to get the updated resource tree
		Then().
		And(func(app *v1alpha1.Application) {
			// Now check the resource tree to verify both Roles show up as children of the ClusterRole
			closer, cdClient := ArgoCDClientset.NewApplicationClientOrDie()
			defer io.Close(closer)

			tree, err := cdClient.ResourceTree(context.Background(), &applicationpkg.ResourcesQuery{
				ApplicationName: &app.Name,
				AppNamespace:    &app.Namespace,
			})
			require.NoError(t, err)
			require.NotNil(t, tree)

			// Find the ClusterRole in the tree
			var clusterRoleNode *v1alpha1.ResourceNode
			for _, node := range tree.Nodes {
				if node.Kind == "ClusterRole" && node.Name == "test-cluster-role" {
					clusterRoleNode = &node
					break
				}
			}
			require.NotNil(t, clusterRoleNode, "ClusterRole not found in resource tree")
			t.Logf("Found ClusterRole in tree: %s", clusterRoleNode.Name)

			// Find both Roles and verify they reference the ClusterRole as their parent
			var roleSameNs, roleOtherNs *v1alpha1.ResourceNode
			for _, node := range tree.Nodes {
				if node.Kind == "Role" {
					t.Logf("Found Role: %s in namespace %s with parent refs: %v",
						node.Name, node.Namespace, node.ParentRefs)

					if node.Name == "test-role-same-ns" {
						roleSameNs = &node
					} else if node.Name == "test-role-other-ns" {
						roleOtherNs = &node
					}
				}
			}

			// Verify both roles were found
			require.NotNil(t, roleSameNs, "Role in same namespace not found in resource tree")
			require.NotNil(t, roleOtherNs, "Role in other namespace not found in resource tree")

			// Verify both roles have the ClusterRole as their parent
			assert.Len(t, roleSameNs.ParentRefs, 1, "Role in same namespace should have one parent")
			assert.Equal(t, "ClusterRole", roleSameNs.ParentRefs[0].Kind)
			assert.Equal(t, "test-cluster-role", roleSameNs.ParentRefs[0].Name)
			assert.Equal(t, string(clusterRoleUID), roleSameNs.ParentRefs[0].UID)

			assert.Len(t, roleOtherNs.ParentRefs, 1, "Role in other namespace should have one parent")
			assert.Equal(t, "ClusterRole", roleOtherNs.ParentRefs[0].Kind)
			assert.Equal(t, "test-cluster-role", roleOtherNs.ParentRefs[0].Name)
			assert.Equal(t, string(clusterRoleUID), roleOtherNs.ParentRefs[0].UID)

			t.Log("✓ Both Roles correctly show ClusterRole as their parent in the resource tree")
		}).
		When().
		Delete(true).
		Then().
		Expect(DoesNotExist())
}

// TestCrossNamespaceOwnershipWithRefresh tests that cross-namespace relationships are maintained
// after a cluster cache refresh/invalidation
func TestCrossNamespaceOwnershipWithRefresh(t *testing.T) {
	var clusterRoleUID string

	Given(t).
		Path("cross-namespace-ownership").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(v1alpha1.SyncStatusCodeSynced)).
		And(func(app *v1alpha1.Application) {
			// Get the UID of the ClusterRole
			output, err := Run("", "kubectl", "get", "clusterrole", "test-cluster-role",
				"-o", "jsonpath={.metadata.uid}")
			require.NoError(t, err)
			clusterRoleUID = output
		}).
		When().
		And(func() {
			// Create a Role with an ownerReference to the ClusterRole
			roleYaml := fmt.Sprintf(`apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: test-role-refresh
  namespace: %s
  ownerReferences:
  - apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRole
    name: test-cluster-role
    uid: %s
rules:
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "list"]`, DeploymentNamespace(), clusterRoleUID)

			_, err := Run("", "sh", "-c", fmt.Sprintf("echo '%s' | kubectl apply -f -", roleYaml))
			require.NoError(t, err)

			// Give the cache a moment to pick up the changes
			time.Sleep(2 * time.Second)
		}).
		Refresh(v1alpha1.RefreshTypeHard). // Force a hard refresh to invalidate the cache
		Then().
		And(func(app *v1alpha1.Application) {
			// Verify the relationship is still tracked after refresh
			closer, cdClient := ArgoCDClientset.NewApplicationClientOrDie()
			defer io.Close(closer)

			tree, err := cdClient.ResourceTree(context.Background(), &applicationpkg.ResourcesQuery{
				ApplicationName: &app.Name,
				AppNamespace:    &app.Namespace,
			})
			require.NoError(t, err)

			// Find the Role and verify it still has the ClusterRole as parent
			var roleNode *v1alpha1.ResourceNode
			for _, node := range tree.Nodes {
				if node.Kind == "Role" && node.Name == "test-role-refresh" {
					roleNode = &node
					break
				}
			}

			require.NotNil(t, roleNode, "Role not found in resource tree after refresh")
			assert.Len(t, roleNode.ParentRefs, 1, "Role should have one parent after refresh")
			assert.Equal(t, "ClusterRole", roleNode.ParentRefs[0].Kind)
			assert.Equal(t, "test-cluster-role", roleNode.ParentRefs[0].Name)

			t.Log("✓ Cross-namespace relationship maintained after cache refresh")
		}).
		When().
		Delete(true).
		Then().
		Expect(DoesNotExist())
}