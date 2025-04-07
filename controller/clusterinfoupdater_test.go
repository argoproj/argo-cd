package controller

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/common"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appsfake "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/fake"
	appinformers "github.com/argoproj/argo-cd/v2/pkg/client/informers/externalversions/application/v1alpha1"
	applisters "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	cacheutil "github.com/argoproj/argo-cd/v2/util/cache"
	"github.com/argoproj/argo-cd/v2/util/cache/appstate"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/settings"

	clustercache "github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

// Expect cluster cache update is persisted in cluster secret
func TestClusterSecretUpdater(t *testing.T) {
	const fakeNamespace = "fake-ns"
	const updatedK8sVersion = "1.0"
	now := time.Now()

	tests := []struct {
		LastCacheSyncTime *time.Time
		SyncError         error
		ExpectedStatus    v1alpha1.ConnectionStatus
	}{
		{nil, nil, v1alpha1.ConnectionStatusUnknown},
		{&now, nil, v1alpha1.ConnectionStatusSuccessful},
		{&now, fmt.Errorf("sync failed"), v1alpha1.ConnectionStatusFailed},
	}

	emptyArgoCDConfigMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDConfigMapName,
			Namespace: fakeNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: map[string]string{},
	}
	argoCDSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDSecretName,
			Namespace: fakeNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: map[string][]byte{
			"admin.password":   nil,
			"server.secretkey": nil,
		},
	}
	kubeclientset := fake.NewSimpleClientset(emptyArgoCDConfigMap, argoCDSecret)
	appclientset := appsfake.NewSimpleClientset()
	appInformer := appinformers.NewApplicationInformer(appclientset, "", time.Minute, cache.Indexers{})
	settingsManager := settings.NewSettingsManager(context.Background(), kubeclientset, fakeNamespace)
	argoDB := db.NewDB(fakeNamespace, settingsManager, kubeclientset)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	appCache := appstate.NewCache(cacheutil.NewCache(cacheutil.NewInMemoryCache(time.Minute)), time.Minute)
	cluster, err := argoDB.CreateCluster(ctx, &v1alpha1.Cluster{Server: "http://minikube"})
	require.NoError(t, err, "Test prepare test data create cluster failed")

	for _, test := range tests {
		info := &clustercache.ClusterInfo{
			Server:            cluster.Server,
			K8SVersion:        updatedK8sVersion,
			LastCacheSyncTime: test.LastCacheSyncTime,
			SyncError:         test.SyncError,
		}

		lister := applisters.NewApplicationLister(appInformer.GetIndexer()).Applications(fakeNamespace)
		updater := NewClusterInfoUpdater(nil, argoDB, lister, appCache, nil, nil, fakeNamespace)

		err = updater.updateClusterInfo(context.Background(), *cluster, info)
		require.NoError(t, err, "Invoking updateClusterInfo failed.")

		var clusterInfo v1alpha1.ClusterInfo
		err = appCache.GetClusterInfo(cluster.Server, &clusterInfo)
		require.NoError(t, err)
		assert.Equal(t, updatedK8sVersion, clusterInfo.ServerVersion)
		assert.Equal(t, test.ExpectedStatus, clusterInfo.ConnectionState.Status)
	}
}

func TestUpdateClusterLabels(t *testing.T) {
	shouldNotBeInvoked := func(ctx context.Context, cluster *v1alpha1.Cluster) (*v1alpha1.Cluster, error) {
		shouldNotHappen := errors.New("if an error happens here, something's wrong")
		require.NoError(t, shouldNotHappen)
		return nil, shouldNotHappen
	}
	tests := []struct {
		name          string
		clusterInfo   *clustercache.ClusterInfo
		cluster       v1alpha1.Cluster
		updateCluster func(context.Context, *v1alpha1.Cluster) (*v1alpha1.Cluster, error)
		wantErr       assert.ErrorAssertionFunc
	}{
		{
			"enableClusterInfoLabels = false",
			&clustercache.ClusterInfo{
				Server:     "kubernetes.svc.local",
				K8SVersion: "1.28",
			},
			v1alpha1.Cluster{
				Server: "kubernetes.svc.local",
				Labels: nil,
			},
			shouldNotBeInvoked,
			assert.NoError,
		},
		{
			"clusterInfo = nil",
			nil,
			v1alpha1.Cluster{
				Server: "kubernetes.svc.local",
				Labels: map[string]string{"argocd.argoproj.io/auto-label-cluster-info": "true"},
			},
			shouldNotBeInvoked,
			assert.NoError,
		},
		{
			"clusterInfo.k8sversion == cluster k8s label",
			&clustercache.ClusterInfo{
				Server:     "kubernetes.svc.local",
				K8SVersion: "1.28",
			},
			v1alpha1.Cluster{
				Server: "kubernetes.svc.local",
				Labels: map[string]string{"argocd.argoproj.io/kubernetes-version": "1.28", "argocd.argoproj.io/auto-label-cluster-info": "true"},
			},
			shouldNotBeInvoked,
			assert.NoError,
		},
		{
			"clusterInfo.k8sversion != cluster k8s label, no error",
			&clustercache.ClusterInfo{
				Server:     "kubernetes.svc.local",
				K8SVersion: "1.28",
			},
			v1alpha1.Cluster{
				Server: "kubernetes.svc.local",
				Labels: map[string]string{"argocd.argoproj.io/kubernetes-version": "1.27", "argocd.argoproj.io/auto-label-cluster-info": "true"},
			},
			func(ctx context.Context, cluster *v1alpha1.Cluster) (*v1alpha1.Cluster, error) {
				assert.Equal(t, "1.28", cluster.Labels["argocd.argoproj.io/kubernetes-version"])
				return nil, nil
			},
			assert.NoError,
		},
		{
			"clusterInfo.k8sversion != cluster k8s label, some error",
			&clustercache.ClusterInfo{
				Server:     "kubernetes.svc.local",
				K8SVersion: "1.28",
			},
			v1alpha1.Cluster{
				Server: "kubernetes.svc.local",
				Labels: map[string]string{"argocd.argoproj.io/kubernetes-version": "1.27", "argocd.argoproj.io/auto-label-cluster-info": "true"},
			},
			func(ctx context.Context, cluster *v1alpha1.Cluster) (*v1alpha1.Cluster, error) {
				assert.Equal(t, "1.28", cluster.Labels["argocd.argoproj.io/kubernetes-version"])
				return nil, errors.New("some error happened while saving")
			},
			assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantErr(t, updateClusterLabels(context.Background(), tt.clusterInfo, tt.cluster, tt.updateCluster), fmt.Sprintf("updateClusterLabels(%v, %v, %v)", context.Background(), tt.clusterInfo, tt.cluster))
		})
	}
}
