package controller

import (
	"context"
	"testing"

	clustercache "github.com/argoproj/gitops-engine/pkg/utils/kube/cache"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/settings"
)

// Test when cluster cache has newly updated K8SVersion.
// Expect this update is persisted in cluster secret
func TestClusterSecretUpdater(t *testing.T) {
	const fakeNamespace = "fake-ns"
	const updatedK8sVersion = "1.0"

	kubeclientset := fake.NewSimpleClientset()
	settingsManager := settings.NewSettingsManager(context.Background(), kubeclientset, fakeNamespace)
	db := db.NewDB(fakeNamespace, settingsManager, kubeclientset)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cluster, err := db.CreateCluster(ctx, &v1alpha1.Cluster{Server: "http://minikube"})
	if !assert.NoError(t, err) {
		return
	}
	//cluster cache's K8SVersion is changed to updatedK8sVersion
	info := &clustercache.ClusterInfo{
		Server:     cluster.Server,
		K8SVersion: updatedK8sVersion,
	}

	err = updateClusterFromClusterCache(db, info)
	cluster, err = db.GetCluster(ctx, cluster.Server)
	assert.NoError(t, err)
	assert.Equal(t, updatedK8sVersion, cluster.ServerVersion)
	assert.Equal(t, v1alpha1.ConnectionStatusUnknown, cluster.ConnectionState.Status)
}
