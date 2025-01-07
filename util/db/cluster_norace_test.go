//go:build !race
// +build !race

package db

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/common"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

func TestWatchClusters_CreateRemoveCluster(t *testing.T) {
	// !race:
	// Intermittent failure when running TestWatchClusters_LocalClusterModifications with -race, likely due to race condition
	// https://github.com/argoproj/argo-cd/issues/4755
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
	settingsManager := settings.NewSettingsManager(context.Background(), kubeclientset, fakeNamespace)
	db := NewDB(fakeNamespace, settingsManager, kubeclientset)
	runWatchTest(t, db, []func(old *v1alpha1.Cluster, new *v1alpha1.Cluster){
		func(old *v1alpha1.Cluster, new *v1alpha1.Cluster) {
			assert.Nil(t, old)
			assert.Equal(t, v1alpha1.KubernetesInternalAPIServerAddr, new.Server)

			_, err := db.CreateCluster(context.Background(), &v1alpha1.Cluster{
				Server: "https://minikube",
				Name:   "minikube",
			})
			assert.NoError(t, err)
		},
		func(old *v1alpha1.Cluster, new *v1alpha1.Cluster) {
			assert.Nil(t, old)
			assert.Equal(t, "https://minikube", new.Server)
			assert.Equal(t, "minikube", new.Name)

			assert.NoError(t, db.DeleteCluster(context.Background(), "https://minikube"))
		},
		func(old *v1alpha1.Cluster, new *v1alpha1.Cluster) {
			assert.Nil(t, new)
			assert.Equal(t, "https://minikube", old.Server)
		},
	})
}

func TestWatchClusters_LocalClusterModifications(t *testing.T) {
	// !race:
	// Intermittent failure when running TestWatchClusters_LocalClusterModifications with -race, likely due to race condition
	// https://github.com/argoproj/argo-cd/issues/4755
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
	settingsManager := settings.NewSettingsManager(context.Background(), kubeclientset, fakeNamespace)
	db := NewDB(fakeNamespace, settingsManager, kubeclientset)
	runWatchTest(t, db, []func(old *v1alpha1.Cluster, new *v1alpha1.Cluster){
		func(old *v1alpha1.Cluster, new *v1alpha1.Cluster) {
			assert.Nil(t, old)
			assert.Equal(t, v1alpha1.KubernetesInternalAPIServerAddr, new.Server)

			_, err := db.CreateCluster(context.Background(), &v1alpha1.Cluster{
				Server: v1alpha1.KubernetesInternalAPIServerAddr,
				Name:   "some name",
			})
			assert.NoError(t, err)
		},
		func(old *v1alpha1.Cluster, new *v1alpha1.Cluster) {
			assert.NotNil(t, old)
			assert.Equal(t, v1alpha1.KubernetesInternalAPIServerAddr, new.Server)
			assert.Equal(t, "some name", new.Name)

			assert.NoError(t, db.DeleteCluster(context.Background(), v1alpha1.KubernetesInternalAPIServerAddr))
		},
		func(old *v1alpha1.Cluster, new *v1alpha1.Cluster) {
			assert.Equal(t, v1alpha1.KubernetesInternalAPIServerAddr, new.Server)
			assert.Equal(t, "in-cluster", new.Name)
		},
	})
}
