package controller

import (
	"context"
	"testing"
	"time"

	mockstatecache "github.com/argoproj/argo-cd/controller/cache/mocks"
	clustercache "github.com/argoproj/gitops-engine/pkg/utils/kube/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/settings"
)

const (
	fakeNamespace = "fake-ns"
)

// Test when clusterInfo has newly updated K8SVersion
// Expect this update is persisted in cluster secret
func TestClusterSecretUpdater(t *testing.T) {
	const updatedK8sVersion = "1.0"

	kubeclientset := fake.NewSimpleClientset()
	settingsManager := settings.NewSettingsManager(context.Background(), kubeclientset, fakeNamespace)
	db := db.NewDB(fakeNamespace, settingsManager, kubeclientset)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	//secretUpdateInterval = 30 * time.Second, so this has to be larger than that
	timeout := time.Second * 36
	cluster, err := db.CreateCluster(ctx, &v1alpha1.Cluster{Server: "http://minikube"})
	if !assert.NoError(t, err) {
		return
	}
	//mocking clusterInfo K8SVersion changed to updatedK8sVersion
	clusterInfos := make([]clustercache.ClusterInfo, 1)
	info := clustercache.ClusterInfo{
		Server:     cluster.Server,
		K8SVersion: updatedK8sVersion,
	}
	clusterInfos = append(clusterInfos, info)
	var mockedLiveStateCache = mockstatecache.LiveStateCache{}
	mockedLiveStateCache.On("GetClustersInfo", mock.Anything, mock.Anything).Return(clusterInfos, nil)

	updater := &clusterSecretUpdater{infoSource: &mockedLiveStateCache, db: db}

	go func() {
		updater.Run(ctx)
	}()

	select {
	case <-ctx.Done():
		assert.Fail(t, "clusterSecretUpdater is cancelled")
	case <-time.After(timeout):
		cluster, err = db.GetCluster(ctx, cluster.Server)
		assert.NoError(t, err)
		assert.Equal(t, updatedK8sVersion, cluster.ServerVersion)
	}
}
