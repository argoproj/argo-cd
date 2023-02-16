package cluster

import (
	"context"
	"fmt"
	"testing"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/gitops-engine/pkg/utils/kube/kubetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/pointer"

	"github.com/argoproj/argo-cd/v2/common"
	clusterapi "github.com/argoproj/argo-cd/v2/pkg/apiclient/cluster"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	servercache "github.com/argoproj/argo-cd/v2/server/cache"
	"github.com/argoproj/argo-cd/v2/test"
	cacheutil "github.com/argoproj/argo-cd/v2/util/cache"
	appstatecache "github.com/argoproj/argo-cd/v2/util/cache/appstate"
	dbmocks "github.com/argoproj/argo-cd/v2/util/db/mocks"
	"github.com/argoproj/argo-cd/v2/util/rbac"
)

func newServerInMemoryCache() *servercache.Cache {
	return servercache.NewCache(
		appstatecache.NewCache(
			cacheutil.NewCache(cacheutil.NewInMemoryCache(1*time.Hour)),
			1*time.Minute,
		),
		1*time.Minute,
		1*time.Minute,
		1*time.Minute,
	)
}

func newNoopEnforcer() *rbac.Enforcer {
	enf := rbac.NewEnforcer(fake.NewSimpleClientset(test.NewFakeConfigMap()), test.FakeArgoCDNamespace, common.ArgoCDConfigMapName, nil)
	enf.EnableEnforce(false)
	return enf
}

func TestUpdateCluster_RejectInvalidParams(t *testing.T) {
	testCases := []struct {
		name    string
		request clusterapi.ClusterUpdateRequest
	}{
		{
			name:    "allowed cluster URL in body, disallowed cluster URL in query",
			request: clusterapi.ClusterUpdateRequest{Cluster: &v1alpha1.Cluster{Name: "", Server: "https://127.0.0.1", Project: "", ClusterResources: true}, Id: &clusterapi.ClusterID{Type: "", Value: "https://127.0.0.2"}, UpdatedFields: []string{"clusterResources", "project"}},
		},
		{
			name:    "allowed cluster URL in body, disallowed cluster name in query",
			request: clusterapi.ClusterUpdateRequest{Cluster: &v1alpha1.Cluster{Name: "", Server: "https://127.0.0.1", Project: "", ClusterResources: true}, Id: &clusterapi.ClusterID{Type: "name", Value: "disallowed-unscoped"}, UpdatedFields: []string{"clusterResources", "project"}},
		},
		{
			name:    "allowed cluster URL in body, disallowed cluster name in query, changing unscoped to scoped",
			request: clusterapi.ClusterUpdateRequest{Cluster: &v1alpha1.Cluster{Name: "", Server: "https://127.0.0.1", Project: "allowed-project", ClusterResources: true}, Id: &clusterapi.ClusterID{Type: "", Value: "https://127.0.0.2"}, UpdatedFields: []string{"clusterResources", "project"}},
		},
		{
			name:    "allowed cluster URL in body, disallowed cluster URL in query, changing unscoped to scoped",
			request: clusterapi.ClusterUpdateRequest{Cluster: &v1alpha1.Cluster{Name: "", Server: "https://127.0.0.1", Project: "allowed-project", ClusterResources: true}, Id: &clusterapi.ClusterID{Type: "name", Value: "disallowed-unscoped"}, UpdatedFields: []string{"clusterResources", "project"}},
		},
	}

	db := &dbmocks.ArgoDB{}

	clusters := []v1alpha1.Cluster{
		{
			Name:   "allowed-unscoped",
			Server: "https://127.0.0.1",
		},
		{
			Name:   "disallowed-unscoped",
			Server: "https://127.0.0.2",
		},
		{
			Name:    "allowed-scoped",
			Server:  "https://127.0.0.3",
			Project: "allowed-project",
		},
		{
			Name:    "disallowed-scoped",
			Server:  "https://127.0.0.4",
			Project: "disallowed-project",
		},
	}

	db.On("ListClusters", mock.Anything).Return(
		func(ctx context.Context) *v1alpha1.ClusterList {
			return &v1alpha1.ClusterList{
				ListMeta: v1.ListMeta{},
				Items:    clusters,
			}
		},
		func(ctx context.Context) error {
			return nil
		},
	)
	db.On("UpdateCluster", mock.Anything, mock.Anything).Return(
		func(ctx context.Context, c *v1alpha1.Cluster) *v1alpha1.Cluster {
			for _, cluster := range clusters {
				if c.Server == cluster.Server {
					return c
				}
			}
			return nil
		},
		func(ctx context.Context, c *v1alpha1.Cluster) error {
			for _, cluster := range clusters {
				if c.Server == cluster.Server {
					return nil
				}
			}
			return fmt.Errorf("cluster '%s' not found", c.Server)
		},
	)
	db.On("GetCluster", mock.Anything, mock.Anything).Return(
		func(ctx context.Context, server string) *v1alpha1.Cluster {
			for _, cluster := range clusters {
				if server == cluster.Server {
					return &cluster
				}
			}
			return nil
		},
		func(ctx context.Context, server string) error {
			for _, cluster := range clusters {
				if server == cluster.Server {
					return nil
				}
			}
			return fmt.Errorf("cluster '%s' not found", server)
		},
	)

	enf := rbac.NewEnforcer(fake.NewSimpleClientset(test.NewFakeConfigMap()), test.FakeArgoCDNamespace, common.ArgoCDConfigMapName, nil)
	_ = enf.SetBuiltinPolicy(`p, role:test, clusters, *, https://127.0.0.1, allow
p, role:test, clusters, *, allowed-project/*, allow`)
	enf.SetDefaultRole("role:test")
	server := NewServer(db, enf, newServerInMemoryCache(), &kubetest.MockKubectlCmd{})

	for _, c := range testCases {
		cc := c
		t.Run(cc.name, func(t *testing.T) {
			t.Parallel()
			out, err := server.Update(context.Background(), &cc.request)
			require.Nil(t, out)
			assert.ErrorIs(t, err, common.PermissionDeniedAPIError)
		})
	}
}

func TestGetCluster_UrlEncodedName(t *testing.T) {
	db := &dbmocks.ArgoDB{}

	mockCluster := v1alpha1.Cluster{
		Name:       "test/ing",
		Server:     "https://127.0.0.1",
		Namespaces: []string{"default", "kube-system"},
	}
	mockClusterList := v1alpha1.ClusterList{
		ListMeta: v1.ListMeta{},
		Items: []v1alpha1.Cluster{
			mockCluster,
		},
	}

	db.On("ListClusters", mock.Anything).Return(&mockClusterList, nil)

	server := NewServer(db, newNoopEnforcer(), newServerInMemoryCache(), &kubetest.MockKubectlCmd{})

	cluster, err := server.Get(context.Background(), &clusterapi.ClusterQuery{
		Id: &clusterapi.ClusterID{
			Type:  "name_escaped",
			Value: "test%2fing",
		},
	})
	require.NoError(t, err)

	assert.Equal(t, cluster.Name, "test/ing")
}

func TestGetCluster_NameWithUrlEncodingButShouldNotBeUnescaped(t *testing.T) {
	db := &dbmocks.ArgoDB{}

	mockCluster := v1alpha1.Cluster{
		Name:       "test%2fing",
		Server:     "https://127.0.0.1",
		Namespaces: []string{"default", "kube-system"},
	}
	mockClusterList := v1alpha1.ClusterList{
		ListMeta: v1.ListMeta{},
		Items: []v1alpha1.Cluster{
			mockCluster,
		},
	}

	db.On("ListClusters", mock.Anything).Return(&mockClusterList, nil)

	server := NewServer(db, newNoopEnforcer(), newServerInMemoryCache(), &kubetest.MockKubectlCmd{})

	cluster, err := server.Get(context.Background(), &clusterapi.ClusterQuery{
		Id: &clusterapi.ClusterID{
			Type:  "name",
			Value: "test%2fing",
		},
	})
	require.NoError(t, err)

	assert.Equal(t, cluster.Name, "test%2fing")
}

func TestUpdateCluster_NoFieldsPaths(t *testing.T) {
	db := &dbmocks.ArgoDB{}
	var updated *v1alpha1.Cluster

	clusters := []v1alpha1.Cluster{
		{
			Name:       "minikube",
			Server:     "https://127.0.0.1",
			Namespaces: []string{"default", "kube-system"},
		},
	}

	clusterList := v1alpha1.ClusterList{
		ListMeta: v1.ListMeta{},
		Items:    clusters,
	}

	db.On("ListClusters", mock.Anything).Return(&clusterList, nil)
	db.On("UpdateCluster", mock.Anything, mock.MatchedBy(func(c *v1alpha1.Cluster) bool {
		updated = c
		return true
	})).Return(&v1alpha1.Cluster{}, nil)

	server := NewServer(db, newNoopEnforcer(), newServerInMemoryCache(), &kubetest.MockKubectlCmd{})

	_, err := server.Update(context.Background(), &clusterapi.ClusterUpdateRequest{
		Cluster: &v1alpha1.Cluster{
			Name:       "minikube",
			Namespaces: []string{"default", "kube-system"},
		},
	})

	require.NoError(t, err)

	assert.Equal(t, updated.Name, "minikube")
	assert.Equal(t, updated.Namespaces, []string{"default", "kube-system"})
}

func TestUpdateCluster_FieldsPathSet(t *testing.T) {
	db := &dbmocks.ArgoDB{}
	var updated *v1alpha1.Cluster
	db.On("GetCluster", mock.Anything, "https://127.0.0.1").Return(&v1alpha1.Cluster{
		Name:       "minikube",
		Server:     "https://127.0.0.1",
		Namespaces: []string{"default", "kube-system"},
	}, nil)
	db.On("UpdateCluster", mock.Anything, mock.MatchedBy(func(c *v1alpha1.Cluster) bool {
		updated = c
		return true
	})).Return(&v1alpha1.Cluster{}, nil)

	server := NewServer(db, newNoopEnforcer(), newServerInMemoryCache(), &kubetest.MockKubectlCmd{})

	_, err := server.Update(context.Background(), &clusterapi.ClusterUpdateRequest{
		Cluster: &v1alpha1.Cluster{
			Server: "https://127.0.0.1",
			Shard:  pointer.Int64Ptr(1),
		},
		UpdatedFields: []string{"shard"},
	})

	require.NoError(t, err)

	assert.Equal(t, updated.Name, "minikube")
	assert.Equal(t, updated.Namespaces, []string{"default", "kube-system"})
	assert.Equal(t, *updated.Shard, int64(1))

	labelEnv := map[string]string{
		"env": "qa",
	}
	_, err = server.Update(context.Background(), &clusterapi.ClusterUpdateRequest{
		Cluster: &v1alpha1.Cluster{
			Server: "https://127.0.0.1",
			Labels: labelEnv,
		},
		UpdatedFields: []string{"labels"},
	})

	require.NoError(t, err)

	assert.Equal(t, updated.Name, "minikube")
	assert.Equal(t, updated.Namespaces, []string{"default", "kube-system"})
	assert.Equal(t, updated.Labels, labelEnv)

	annotationEnv := map[string]string{
		"env": "qa",
	}
	_, err = server.Update(context.Background(), &clusterapi.ClusterUpdateRequest{
		Cluster: &v1alpha1.Cluster{
			Server:      "https://127.0.0.1",
			Annotations: annotationEnv,
		},
		UpdatedFields: []string{"annotations"},
	})

	require.NoError(t, err)

	assert.Equal(t, updated.Name, "minikube")
	assert.Equal(t, updated.Namespaces, []string{"default", "kube-system"})
	assert.Equal(t, updated.Annotations, annotationEnv)

	_, err = server.Update(context.Background(), &clusterapi.ClusterUpdateRequest{
		Cluster: &v1alpha1.Cluster{
			Server:  "https://127.0.0.1",
			Project: "new-project",
		},
		UpdatedFields: []string{"project"},
	})

	require.NoError(t, err)

	assert.Equal(t, updated.Name, "minikube")
	assert.Equal(t, updated.Namespaces, []string{"default", "kube-system"})
	assert.Equal(t, updated.Project, "new-project")
}
