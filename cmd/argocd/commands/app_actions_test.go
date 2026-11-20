package commands

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v3/cmd/util"
	applicationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

type actionsAppServiceClient struct {
	applicationpkg.ApplicationServiceClient
	managedResources []*v1alpha1.ResourceDiff
	tree             *v1alpha1.ApplicationTree
}

func (c *actionsAppServiceClient) ManagedResources(_ context.Context, _ *applicationpkg.ResourcesQuery, _ ...grpc.CallOption) (*applicationpkg.ManagedResourcesResponse, error) {
	return &applicationpkg.ManagedResourcesResponse{Items: c.managedResources}, nil
}

func (c *actionsAppServiceClient) ResourceTree(_ context.Context, _ *applicationpkg.ResourcesQuery, _ ...grpc.CallOption) (*v1alpha1.ApplicationTree, error) {
	return c.tree, nil
}

func (c *actionsAppServiceClient) Get(_ context.Context, q *applicationpkg.ApplicationQuery, _ ...grpc.CallOption) (*v1alpha1.Application, error) {
	return &v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Name: q.GetName(), Namespace: "argocd"}}, nil
}

func newActionsAppServiceClient() *actionsAppServiceClient {
	return &actionsAppServiceClient{
		managedResources: []*v1alpha1.ResourceDiff{{
			Group:     "apps",
			Kind:      "Deployment",
			Namespace: "default",
			Name:      "from-git",
			LiveState: `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"from-git","namespace":"default"}}`,
		}},
		tree: &v1alpha1.ApplicationTree{
			Nodes: []v1alpha1.ResourceNode{
				{ResourceRef: v1alpha1.ResourceRef{Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "default", Name: "from-git", UID: "1"}},
				{ResourceRef: v1alpha1.ResourceRef{Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "default", Name: "from-controller", UID: "2"}},
				{ResourceRef: v1alpha1.ResourceRef{Group: "apps", Version: "v1", Kind: "ReplicaSet", Namespace: "default", Name: "from-controller-5d8f9", UID: "3"}},
				{ResourceRef: v1alpha1.ResourceRef{Version: "v1", Kind: "Pod", Namespace: "default", Name: "from-controller-5d8f9-x2kq7", UID: "4"}},
				{ResourceRef: v1alpha1.ResourceRef{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRole", Name: "from-controller", UID: "5"}},
				// A managed resource that hasn't been created yet.
				{ResourceRef: v1alpha1.ResourceRef{Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "default", Name: "missing"}},
			},
		},
	}
}

func deploymentNames(t *testing.T, resources []*v1alpha1.ResourceDiff) []string {
	t.Helper()
	deployments, err := util.FilterResources(false, resources, "", "Deployment", "", "", true)
	require.NoError(t, err)
	names := make([]string, 0, len(deployments))
	for _, deployment := range deployments {
		assert.Equal(t, "apps/v1", deployment.GetAPIVersion())
		assert.Equal(t, "default", deployment.GetNamespace())
		names = append(names, deployment.GetName())
	}
	return names
}

func TestGetActionableResourcesForApplication(t *testing.T) {
	appName := "my-app"
	appNs := "argocd"

	t.Run("managed resources by default", func(t *testing.T) {
		resources, err := getActionableResourcesForApplication(t.Context(), newActionsAppServiceClient(), &appNs, &appName, false)
		require.NoError(t, err)
		assert.Equal(t, []string{"from-git"}, deploymentNames(t, resources))

		app := resources[len(resources)-1]
		assert.Equal(t, "Application", app.Kind)
		assert.Equal(t, appName, app.Name)
	})

	t.Run("resources in the tree with child resources", func(t *testing.T) {
		resources, err := getActionableResourcesForApplication(t.Context(), newActionsAppServiceClient(), &appNs, &appName, true)
		require.NoError(t, err)
		assert.Equal(t, []string{"from-git", "from-controller"}, deploymentNames(t, resources))

		replicaSets, err := util.FilterResources(false, resources, "", "ReplicaSet", "", "", true)
		require.NoError(t, err)
		require.Len(t, replicaSets, 1)
		assert.Equal(t, "from-controller-5d8f9", replicaSets[0].GetName())

		pods, err := util.FilterResources(true, resources, "", "Pod", "default", "", true)
		require.NoError(t, err)
		require.Len(t, pods, 1)
		assert.Equal(t, "v1", pods[0].GetAPIVersion())
		assert.Equal(t, "from-controller-5d8f9-x2kq7", pods[0].GetName())

		clusterRoles, err := util.FilterResources(true, resources, "rbac.authorization.k8s.io", "ClusterRole", "", "", true)
		require.NoError(t, err)
		require.Len(t, clusterRoles, 1)
		assert.Equal(t, "rbac.authorization.k8s.io/v1", clusterRoles[0].GetAPIVersion())
		assert.Empty(t, clusterRoles[0].GetNamespace())

		app := resources[len(resources)-1]
		assert.Equal(t, "Application", app.Kind)
		assert.Equal(t, appName, app.Name)
	})
}
