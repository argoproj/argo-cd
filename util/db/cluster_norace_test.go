// +build !race

package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/settings"
)

func TestWatchClusters_CreateRemoveCluster(t *testing.T) {

	// !race:
	// Intermittent failure when running TestWatchClusters_LocalClusterModifications with -race, likely due to race condition
	// https://github.com/argoproj/argo-cd/issues/4755

	kubeclientset := fake.NewSimpleClientset()
	settingsManager := settings.NewSettingsManager(context.Background(), kubeclientset, fakeNamespace)
	db := NewDB(fakeNamespace, settingsManager, kubeclientset)
	runWatchTest(t, db, []func(old *v1alpha1.Cluster, new *v1alpha1.Cluster){
		func(old *v1alpha1.Cluster, new *v1alpha1.Cluster) {
			assert.Nil(t, old)
			assert.Equal(t, new.Server, common.KubernetesInternalAPIServerAddr)

			_, err := db.CreateCluster(context.Background(), &v1alpha1.Cluster{
				Server: "https://minikube",
				Name:   "minikube",
			})
			assert.NoError(t, err)
		},
		func(old *v1alpha1.Cluster, new *v1alpha1.Cluster) {
			assert.Nil(t, old)
			assert.Equal(t, new.Server, "https://minikube")
			assert.Equal(t, new.Name, "minikube")

			assert.NoError(t, db.DeleteCluster(context.Background(), "https://minikube"))
		},
		func(old *v1alpha1.Cluster, new *v1alpha1.Cluster) {
			assert.Nil(t, new)
			assert.Equal(t, old.Server, "https://minikube")
		},
	})
}

func TestWatchClusters_LocalClusterModifications(t *testing.T) {

	// !race:
	// Intermittent failure when running TestWatchClusters_LocalClusterModifications with -race, likely due to race condition
	// https://github.com/argoproj/argo-cd/issues/4755

	kubeclientset := fake.NewSimpleClientset()
	settingsManager := settings.NewSettingsManager(context.Background(), kubeclientset, fakeNamespace)
	db := NewDB(fakeNamespace, settingsManager, kubeclientset)
	runWatchTest(t, db, []func(old *v1alpha1.Cluster, new *v1alpha1.Cluster){
		func(old *v1alpha1.Cluster, new *v1alpha1.Cluster) {
			assert.Nil(t, old)
			assert.Equal(t, new.Server, common.KubernetesInternalAPIServerAddr)

			_, err := db.CreateCluster(context.Background(), &v1alpha1.Cluster{
				Server: common.KubernetesInternalAPIServerAddr,
				Name:   "some name",
			})
			assert.NoError(t, err)
		},
		func(old *v1alpha1.Cluster, new *v1alpha1.Cluster) {
			assert.NotNil(t, old)
			assert.Equal(t, new.Server, common.KubernetesInternalAPIServerAddr)
			assert.Equal(t, new.Name, "some name")

			assert.NoError(t, db.DeleteCluster(context.Background(), common.KubernetesInternalAPIServerAddr))
		},
		func(old *v1alpha1.Cluster, new *v1alpha1.Cluster) {
			assert.Equal(t, new.Server, common.KubernetesInternalAPIServerAddr)
			assert.Equal(t, new.Name, "in-cluster")
		},
	})
}
