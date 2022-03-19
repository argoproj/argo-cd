package admin

import (
	"testing"

	clustermocks "github.com/argoproj/gitops-engine/pkg/cache/mocks"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	statecache "github.com/argoproj/argo-cd/v2/controller/cache"
	cachemocks "github.com/argoproj/argo-cd/v2/controller/cache/mocks"
	"github.com/argoproj/argo-cd/v2/controller/metrics"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appfake "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/fake"
	argocdclient "github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient/mocks"
	"github.com/argoproj/argo-cd/v2/test"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

func TestGetReconcileResults(t *testing.T) {
	appClientset := appfake.NewSimpleClientset(&v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Status: v1alpha1.ApplicationStatus{
			Health: v1alpha1.HealthStatus{Status: health.HealthStatusHealthy},
			Sync:   v1alpha1.SyncStatus{Status: v1alpha1.SyncStatusCodeOutOfSync},
		},
	})

	result, err := getReconcileResults(appClientset, "default", "")
	if !assert.NoError(t, err) {
		return
	}

	expectedResults := []appReconcileResult{{
		Name:   "test",
		Health: &v1alpha1.HealthStatus{Status: health.HealthStatusHealthy},
		Sync:   &v1alpha1.SyncStatus{Status: v1alpha1.SyncStatusCodeOutOfSync},
	}}
	assert.ElementsMatch(t, expectedResults, result)
}

func TestGetReconcileResults_Refresh(t *testing.T) {
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-cm",
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
	}
	proj := &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Spec: v1alpha1.AppProjectSpec{Destinations: []v1alpha1.ApplicationDestination{{Namespace: "*", Server: "*"}}},
	}

	app := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Destination: v1alpha1.ApplicationDestination{
				Server:    v1alpha1.KubernetesInternalAPIServerAddr,
				Namespace: "default",
			},
		},
	}

	appClientset := appfake.NewSimpleClientset(app, proj)
	deployment := test.NewDeployment()
	kubeClientset := kubefake.NewSimpleClientset(deployment, &cm)
	clusterCache := clustermocks.ClusterCache{}
	clusterCache.On("IsNamespaced", mock.Anything).Return(true, nil)
	repoServerClient := mocks.RepoServerServiceClient{}
	repoServerClient.On("GenerateManifest", mock.Anything, mock.Anything).Return(&argocdclient.ManifestResponse{
		Manifests: []string{test.DeploymentManifest},
	}, nil)
	repoServerClientset := mocks.Clientset{RepoServerServiceClient: &repoServerClient}
	liveStateCache := cachemocks.LiveStateCache{}
	liveStateCache.On("GetManagedLiveObjs", mock.Anything, mock.Anything).Return(map[kube.ResourceKey]*unstructured.Unstructured{
		kube.GetResourceKey(deployment): deployment,
	}, nil)
	liveStateCache.On("GetVersionsInfo", mock.Anything).Return("v1.2.3", nil, nil)
	liveStateCache.On("Init").Return(nil, nil)
	liveStateCache.On("GetClusterCache", mock.Anything).Return(&clusterCache, nil)
	liveStateCache.On("IsNamespaced", mock.Anything, mock.Anything).Return(true, nil)

	result, err := reconcileApplications(kubeClientset, appClientset, "default", &repoServerClientset, "",
		func(argoDB db.ArgoDB, appInformer cache.SharedIndexInformer, settingsMgr *settings.SettingsManager, server *metrics.MetricsServer) statecache.LiveStateCache {
			return &liveStateCache
		},
	)

	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, result[0].Health.Status, health.HealthStatusMissing)
	assert.Equal(t, result[0].Sync.Status, v1alpha1.SyncStatusCodeOutOfSync)
}

func TestDiffReconcileResults_NoDifferences(t *testing.T) {
	logs, err := captureStdout(func() {
		assert.NoError(t, diffReconcileResults(
			reconcileResults{Applications: []appReconcileResult{{
				Name: "app1",
				Sync: &v1alpha1.SyncStatus{Status: v1alpha1.SyncStatusCodeOutOfSync},
			}}},
			reconcileResults{Applications: []appReconcileResult{{
				Name: "app1",
				Sync: &v1alpha1.SyncStatus{Status: v1alpha1.SyncStatusCodeOutOfSync},
			}}},
		))
	})
	assert.NoError(t, err)
	assert.Equal(t, "app1\n", logs)
}

func TestDiffReconcileResults_DifferentApps(t *testing.T) {
	logs, err := captureStdout(func() {
		assert.NoError(t, diffReconcileResults(
			reconcileResults{Applications: []appReconcileResult{{
				Name: "app1",
				Sync: &v1alpha1.SyncStatus{Status: v1alpha1.SyncStatusCodeOutOfSync},
			}, {
				Name: "app2",
				Sync: &v1alpha1.SyncStatus{Status: v1alpha1.SyncStatusCodeOutOfSync},
			}}},
			reconcileResults{Applications: []appReconcileResult{{
				Name: "app1",
				Sync: &v1alpha1.SyncStatus{Status: v1alpha1.SyncStatusCodeOutOfSync},
			}, {
				Name: "app3",
				Sync: &v1alpha1.SyncStatus{Status: v1alpha1.SyncStatusCodeOutOfSync},
			}}},
		))
	})
	assert.NoError(t, err)
	assert.Equal(t, `app1
app2
1,9d0
< conditions: null
< health: null
< name: app2
< sync:
<   comparedTo:
<     destination: {}
<     source:
<       repoURL: ""
<   status: OutOfSync
app3
0a1,9
> conditions: null
> health: null
> name: app3
> sync:
>   comparedTo:
>     destination: {}
>     source:
>       repoURL: ""
>   status: OutOfSync
`, logs)
}
