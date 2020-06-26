package controller

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appsfake "github.com/argoproj/argo-cd/pkg/client/clientset/versioned/fake"
	appinformers "github.com/argoproj/argo-cd/pkg/client/informers/externalversions/application/v1alpha1"
	applisters "github.com/argoproj/argo-cd/pkg/client/listers/application/v1alpha1"
	cacheutil "github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/cache/appstate"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/settings"

	clustercache "github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

// Expect cluster cache update is persisted in cluster secret
func TestClusterSecretUpdater(t *testing.T) {
	const fakeNamespace = "fake-ns"
	const updatedK8sVersion = "1.0"
	now := time.Now()

	var tests = []struct {
		LastCacheSyncTime *time.Time
		SyncError         error
		ExpectedStatus    v1alpha1.ConnectionStatus
	}{
		{nil, nil, v1alpha1.ConnectionStatusUnknown},
		{&now, nil, v1alpha1.ConnectionStatusSuccessful},
		{&now, fmt.Errorf("sync failed"), v1alpha1.ConnectionStatusFailed},
	}

	kubeclientset := fake.NewSimpleClientset()
	appclientset := appsfake.NewSimpleClientset()
	appInformer := appinformers.NewApplicationInformer(appclientset, "", time.Minute, cache.Indexers{})
	settingsManager := settings.NewSettingsManager(context.Background(), kubeclientset, fakeNamespace)
	argoDB := db.NewDB(fakeNamespace, settingsManager, kubeclientset)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	appCache := appstate.NewCache(cacheutil.NewCache(cacheutil.NewInMemoryCache(time.Minute)), time.Minute)
	cluster, err := argoDB.CreateCluster(ctx, &v1alpha1.Cluster{Server: "http://minikube"})
	assert.NoError(t, err, "Test prepare test data create cluster failed")

	for _, test := range tests {
		info := &clustercache.ClusterInfo{
			Server:            cluster.Server,
			K8SVersion:        updatedK8sVersion,
			LastCacheSyncTime: test.LastCacheSyncTime,
			SyncError:         test.SyncError,
		}

		lister := applisters.NewApplicationLister(appInformer.GetIndexer()).Applications(fakeNamespace)
		updater := NewClusterInfoUpdater(nil, argoDB, lister, appCache, nil)

		err = updater.updateClusterInfo(*cluster, info)
		assert.NoError(t, err, "Invoking updateClusterInfo failed.")

		var clusterInfo v1alpha1.ClusterInfo
		err = appCache.GetClusterInfo(cluster.Server, &clusterInfo)
		assert.NoError(t, err)
		assert.Equal(t, updatedK8sVersion, clusterInfo.ServerVersion)
		assert.Equal(t, test.ExpectedStatus, clusterInfo.ConnectionState.Status)
	}
}
