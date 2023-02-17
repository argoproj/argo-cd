package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

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
	"github.com/argoproj/argo-cd/v2/util/db"
	dbmocks "github.com/argoproj/argo-cd/v2/util/db/mocks"
	"github.com/argoproj/argo-cd/v2/util/rbac"
	"github.com/argoproj/argo-cd/v2/util/settings"
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

func TestDeleteClusterByName(t *testing.T) {
	testNamespace := "default"
	clientset := getClientset(nil, testNamespace, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-cluster-secret",
			Namespace: testNamespace,
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
			},
			Annotations: map[string]string{
				common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
			},
		},
		Data: map[string][]byte{
			"name":   []byte("my-cluster-name"),
			"server": []byte("https://my-cluster-server"),
			"config": []byte("{}"),
		},
	})
	db := db.NewDB(testNamespace, settings.NewSettingsManager(context.Background(), clientset, testNamespace), clientset)
	server := NewServer(db, newNoopEnforcer(), newServerInMemoryCache(), &kubetest.MockKubectlCmd{})

	t.Run("Delete Fails When Deleting by Unknown Name", func(t *testing.T) {
		_, err := server.Delete(context.Background(), &clusterapi.ClusterQuery{
			Name: "foo",
		})

		assert.EqualError(t, err, `rpc error: code = PermissionDenied desc = permission denied`)
	})

	t.Run("Delete Succeeds When Deleting by Name", func(t *testing.T) {
		_, err := server.Delete(context.Background(), &clusterapi.ClusterQuery{
			Name: "my-cluster-name",
		})
		assert.Nil(t, err)

		_, err = db.GetCluster(context.Background(), "https://my-cluster-server")
		assert.EqualError(t, err, `rpc error: code = NotFound desc = cluster "https://my-cluster-server" not found`)
	})
}

func TestRotateAuth(t *testing.T) {
	testNamespace := "kube-system"
	token := "eyJhbGciOiJSUzI1NiIsImtpZCI6IiJ9.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJrdWJlLXN5c3RlbSIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VjcmV0Lm5hbWUiOiJhcmdvY2QtbWFuYWdlci10b2tlbi10ajc5ciIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VydmljZS1hY2NvdW50Lm5hbWUiOiJhcmdvY2QtbWFuYWdlciIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VydmljZS1hY2NvdW50LnVpZCI6IjkxZGQzN2NmLThkOTItMTFlOS1hMDkxLWQ2NWYyYWU3ZmE4ZCIsInN1YiI6InN5c3RlbTpzZXJ2aWNlYWNjb3VudDprdWJlLXN5c3RlbTphcmdvY2QtbWFuYWdlciJ9.ytZjt2pDV8-A7DBMR06zQ3wt9cuVEfq262TQw7sdra-KRpDpMPnziMhc8bkwvgW-LGhTWUh5iu1y-1QhEx6mtbCt7vQArlBRxfvM5ys6ClFkplzq5c2TtZ7EzGSD0Up7tdxuG9dvR6TGXYdfFcG779yCdZo2H48sz5OSJfdEriduMEY1iL5suZd3ebOoVi1fGflmqFEkZX6SvxkoArl5mtNP6TvZ1eTcn64xh4ws152hxio42E-eSnl_CET4tpB5vgP5BVlSKW2xB7w2GJxqdETA5LJRI_OilY77dTOp8cMr_Ck3EOeda3zHfh4Okflg8rZFEeAuJYahQNeAILLkcA"
	config := v1alpha1.ClusterConfig{
		BearerToken: token,
	}

	configMarshal, err := json.Marshal(config)
	if err != nil {
		t.Errorf("failed to marshal config for test: %v", err)
	}

	clientset := getClientset(nil, testNamespace,
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-cluster-secret",
				Namespace: testNamespace,
				Labels: map[string]string{
					common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
				},
				Annotations: map[string]string{
					common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
				},
			},
			Data: map[string][]byte{
				"name":   []byte("my-cluster-name"),
				"server": []byte("https://my-cluster-name"),
				"config": configMarshal,
			},
		},
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "kube-system",
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-manager-token-tj79r",
				Namespace: "kube-system",
			},
			Data: map[string][]byte{
				"token": []byte(token),
			},
		},
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-manager",
				Namespace: "kube-system",
			},
			Secrets: []corev1.ObjectReference{
				{
					Kind: "Secret",
					Name: "argocd-manager-token-tj79r",
				},
			},
		})

	db := db.NewDB(testNamespace, settings.NewSettingsManager(context.Background(), clientset, testNamespace), clientset)
	server := NewServer(db, newNoopEnforcer(), newServerInMemoryCache(), &kubetest.MockKubectlCmd{})

	t.Run("RotateAuth by Unknown Name", func(t *testing.T) {
		_, err := server.RotateAuth(context.Background(), &clusterapi.ClusterQuery{
			Name: "foo",
		})

		assert.EqualError(t, err, `rpc error: code = PermissionDenied desc = permission denied`)
	})

	// While the tests results for the next two tests result in an error, they do
	// demonstrate the proper mapping of cluster names/server to server info (i.e. my-cluster-name
	// results in https://my-cluster-name info being used and https://my-cluster-name results in https://my-cluster-name).
	t.Run("RotateAuth by Name - Error from no such host", func(t *testing.T) {
		_, err := server.RotateAuth(context.Background(), &clusterapi.ClusterQuery{
			Name: "my-cluster-name",
		})

		require.NotNil(t, err)
		assert.Contains(t, err.Error(), "Get \"https://my-cluster-name/")
	})

	t.Run("RotateAuth by Server - Error from no such host", func(t *testing.T) {
		_, err := server.RotateAuth(context.Background(), &clusterapi.ClusterQuery{
			Server: "https://my-cluster-name",
		})

		require.NotNil(t, err)
		assert.Contains(t, err.Error(), "Get \"https://my-cluster-name/")
	})
}

func getClientset(config map[string]string, ns string, objects ...runtime.Object) *fake.Clientset {
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-secret",
			Namespace: ns,
		},
		Data: map[string][]byte{
			"admin.password":   []byte("test"),
			"server.secretkey": []byte("test"),
		},
	}
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-cm",
			Namespace: ns,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: config,
	}
	return fake.NewSimpleClientset(append(objects, &cm, &secret)...)
}
