package controller

import (
	"context"
	"fmt"
	"testing"
	"time"

	clustercache "github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/settings"
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
	settingsManager := settings.NewSettingsManager(context.Background(), kubeclientset, fakeNamespace)
	db := db.NewDB(fakeNamespace, settingsManager, kubeclientset)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cluster, err := db.CreateCluster(ctx, &v1alpha1.Cluster{Server: "http://minikube"})
	assert.NoError(t, err, "Test prepare test data create cluster failed")

	for _, test := range tests {
		info := &clustercache.ClusterInfo{
			Server:            cluster.Server,
			K8SVersion:        updatedK8sVersion,
			LastCacheSyncTime: test.LastCacheSyncTime,
			SyncError:         test.SyncError,
		}

		err = updateClusterConnectionState(db, info)
		assert.NoError(t, err, "Invoking updateClusterConnectionState failed.")

		cluster, err = db.GetCluster(ctx, cluster.Server)
		assert.NoError(t, err)
		assert.Equal(t, updatedK8sVersion, cluster.ServerVersion)
		assert.Equal(t, test.ExpectedStatus, cluster.ConnectionState.Status)
	}
}
