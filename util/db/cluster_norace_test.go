//go:build !race
// +build !race

package db

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v3/common"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

func TestWatchClusters_CreateRemoveCluster(t *testing.T) {
	// !race:
	// Intermittent failure when running TestWatchClusters_LocalClusterModifications with -race, likely due to race condition
	// https://github.com/argoproj/argo-cd/issues/4755
	emptyArgoCDConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDConfigMapName,
			Namespace: fakeNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: map[string]string{},
	}
	argoCDSecret := &corev1.Secret{
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
	kubeclientset := fake.NewClientset(emptyArgoCDConfigMap, argoCDSecret)
	settingsManager := settings.NewSettingsManager(t.Context(), kubeclientset, fakeNamespace)
	db := NewDB(fakeNamespace, settingsManager, kubeclientset)
	completed := runWatchTest(t, db, []func(old *v1alpha1.Cluster, new *v1alpha1.Cluster){
		func(old *v1alpha1.Cluster, new *v1alpha1.Cluster) {
			assert.Nil(t, old)
			assert.Equal(t, v1alpha1.KubernetesInternalAPIServerAddr, new.Server)

			_, err := db.CreateCluster(t.Context(), &v1alpha1.Cluster{
				Server: "https://minikube",
				Name:   "minikube",
			})
			assert.NoError(t, err)
		},
		func(old *v1alpha1.Cluster, new *v1alpha1.Cluster) {
			assert.Nil(t, old)
			assert.Equal(t, "https://minikube", new.Server)
			assert.Equal(t, "minikube", new.Name)

			assert.NoError(t, db.DeleteCluster(t.Context(), "https://minikube"))
		},
		func(old *v1alpha1.Cluster, new *v1alpha1.Cluster) {
			assert.Nil(t, new)
			assert.Equal(t, "https://minikube", old.Server)
		},
	})
	assert.True(t, completed, "Failed due to timeout")
}

func TestWatchClusters_LocalClusterModifications(t *testing.T) {
	// !race:
	// Intermittent failure when running TestWatchClusters_LocalClusterModifications with -race, likely due to race condition
	// https://github.com/argoproj/argo-cd/issues/4755
	emptyArgoCDConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDConfigMapName,
			Namespace: fakeNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: map[string]string{},
	}
	argoCDSecret := &corev1.Secret{
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
	kubeclientset := fake.NewClientset(emptyArgoCDConfigMap, argoCDSecret)
	settingsManager := settings.NewSettingsManager(t.Context(), kubeclientset, fakeNamespace)
	db := NewDB(fakeNamespace, settingsManager, kubeclientset)
	completed := runWatchTest(t, db, []func(old *v1alpha1.Cluster, new *v1alpha1.Cluster){
		func(old *v1alpha1.Cluster, new *v1alpha1.Cluster) {
			assert.Nil(t, old)
			assert.Equal(t, v1alpha1.KubernetesInternalAPIServerAddr, new.Server)

			_, err := db.CreateCluster(t.Context(), &v1alpha1.Cluster{
				Server: v1alpha1.KubernetesInternalAPIServerAddr,
				Name:   "some name",
			})
			assert.NoError(t, err)
		},
		func(old *v1alpha1.Cluster, new *v1alpha1.Cluster) {
			assert.NotNil(t, old)
			assert.Equal(t, v1alpha1.KubernetesInternalAPIServerAddr, new.Server)
			assert.Equal(t, "some name", new.Name)

			assert.NoError(t, db.DeleteCluster(t.Context(), v1alpha1.KubernetesInternalAPIServerAddr))
		},
		func(_ *v1alpha1.Cluster, new *v1alpha1.Cluster) {
			assert.Equal(t, v1alpha1.KubernetesInternalAPIServerAddr, new.Server)
			assert.Equal(t, "in-cluster", new.Name)
		},
	})
	assert.True(t, completed, "Failed due to timeout")
}

func TestWatchClusters_LocalClusterModificationsWhenDisabled(t *testing.T) {
	// !race:
	// Intermittent failure when running TestWatchClusters_LocalClusterModifications with -race, likely due to race condition
	// https://github.com/argoproj/argo-cd/issues/4755
	argoCDConfigMapWithInClusterServerAddressDisabled := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDConfigMapName,
			Namespace: fakeNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: map[string]string{"cluster.inClusterEnabled": "false"},
	}
	argoCDSecret := &corev1.Secret{
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
	kubeclientset := fake.NewClientset(argoCDConfigMapWithInClusterServerAddressDisabled, argoCDSecret)
	settingsManager := settings.NewSettingsManager(t.Context(), kubeclientset, fakeNamespace)
	db := NewDB(fakeNamespace, settingsManager, kubeclientset)
	completed := runWatchTest(t, db, []func(_ *v1alpha1.Cluster, _ *v1alpha1.Cluster){
		func(_ *v1alpha1.Cluster, _ *v1alpha1.Cluster) {
			assert.Fail(t, "The in-cluster should not be added when disabled")
		},
	})
	assert.False(t, completed, "Expecting the method to never complete because no cluster is ever added")
}
