package controller

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/argoproj/argo-cd/v3/common"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	appsfake "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/fake"
	appinformers "github.com/argoproj/argo-cd/v3/pkg/client/informers/externalversions/application/v1alpha1"
	applisters "github.com/argoproj/argo-cd/v3/pkg/client/listers/application/v1alpha1"
	cacheutil "github.com/argoproj/argo-cd/v3/util/cache"
	"github.com/argoproj/argo-cd/v3/util/cache/appstate"
	"github.com/argoproj/argo-cd/v3/util/db"
	"github.com/argoproj/argo-cd/v3/util/settings"

	clustercache "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/cache"
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
		{&now, errors.New("sync failed"), v1alpha1.ConnectionStatusFailed},
	}

	emptyArgoCDConfigMap := &corev1.ConfigMap{
		Name:      common.ArgoCDConfigMapName,
		Namespace: fakeNamespace,
		Labels: map[string]string{
			"app.kubernetes.io/part-of": "argocd",
		},
		Data: map[string]string{},
	}
	argoCDSecret := &corev1.Secret{
		Name:      common.ArgoCDSecretName,
		Namespace: fakeNamespace,
		Labels: map[string]string{
			"app.kubernetes.io/part-of": "argocd",
		},
		Data: map[string][]byte{
			"admin.password":   nil,
			"server.secretkey": nil,
		},
	}
	kubeclientset := fake.NewClientset(emptyArgoCDConfigMap, argoCDSecret)
	appclientset := appsfake.NewSimpleClientset()
	appInformer := appinformers.NewApplicationInformer(appclientset, "", time.Minute, cache.Indexers{})
	settingsManager := settings.NewSettingsManager(t.Context(), kubeclientset, fakeNamespace)
	argoDB := db.NewDB(fakeNamespace, settingsManager, kubeclientset)
	ctx, cancel := context.WithCancel(t.Context())
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

		err = updater.updateClusterInfo(*cluster, info, 0)
		require.NoError(t, err, "Invoking updateClusterInfo failed.")

		var clusterInfo v1alpha1.ClusterInfo
		err = appCache.GetClusterInfo(cluster.Server, &clusterInfo)
		require.NoError(t, err)
		assert.Equal(t, updatedK8sVersion, clusterInfo.ServerVersion)
		assert.Equal(t, test.ExpectedStatus, clusterInfo.ConnectionState.Status)
	}
}

func TestGetUpdatedClusterInfo_AppCount(t *testing.T) {
	const fakeNamespace = "fake-ns"
	const clusterServer = "https://prod.example.com"
	const clusterName = "prod"

	emptyArgoCDConfigMap := &corev1.ConfigMap{
		Name:      common.ArgoCDConfigMapName,
		Namespace: fakeNamespace,
		Labels:    map[string]string{"app.kubernetes.io/part-of": "argocd"},
		Data:      map[string]string{},
	}
	argoCDSecret := &corev1.Secret{
		Name:      common.ArgoCDSecretName,
		Namespace: fakeNamespace,
		Labels:    map[string]string{"app.kubernetes.io/part-of": "argocd"},
		Data:      map[string][]byte{"admin.password": nil, "server.secretkey": nil},
	}
	clusterSecret := &corev1.Secret{
		Name:      "prod-cluster",
		Namespace: fakeNamespace,
		Labels:    map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeCluster},
		Annotations: map[string]string{
			common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
		},
		Data: map[string][]byte{
			"name":   []byte(clusterName),
			"server": []byte(clusterServer),
			"config": []byte("{}"),
		},
	}

	kubeclientset := fake.NewClientset(emptyArgoCDConfigMap, argoCDSecret, clusterSecret)
	settingsManager := settings.NewSettingsManager(t.Context(), kubeclientset, fakeNamespace)
	argoDB := db.NewDB(fakeNamespace, settingsManager, kubeclientset)

	apps := []*v1alpha1.Application{
		{Spec: v1alpha1.ApplicationSpec{Destination: v1alpha1.ApplicationDestination{Name: clusterName}}},
		{Spec: v1alpha1.ApplicationSpec{Destination: v1alpha1.ApplicationDestination{Server: clusterServer}}},
		{Spec: v1alpha1.ApplicationSpec{Destination: v1alpha1.ApplicationDestination{Server: "https://other.example.com"}}},
	}

	updater := &clusterInfoUpdater{db: argoDB, namespace: fakeNamespace}
	cluster := v1alpha1.Cluster{Server: clusterServer}

	appCountByServer := updater.countAppsByDestinationServer(t.Context(), apps)
	info := getUpdatedClusterInfo(appCountByServer[cluster.Server], nil, metav1.Now())

	assert.Equal(t, int64(2), info.ApplicationsCount)
}

func TestGetUpdatedClusterInfo_AmbiguousName(t *testing.T) {
	const fakeNamespace = "fake-ns"
	const clusterServer = "https://prod.example.com"
	const clusterName = "prod"

	emptyArgoCDConfigMap := &corev1.ConfigMap{
		Name:      common.ArgoCDConfigMapName,
		Namespace: fakeNamespace,
		Labels:    map[string]string{"app.kubernetes.io/part-of": "argocd"},
		Data:      map[string]string{},
	}
	argoCDSecret := &corev1.Secret{
		Name:      common.ArgoCDSecretName,
		Namespace: fakeNamespace,
		Labels:    map[string]string{"app.kubernetes.io/part-of": "argocd"},
		Data:      map[string][]byte{"admin.password": nil, "server.secretkey": nil},
	}
	makeClusterSecret := func(secretName, server string) *corev1.Secret {
		return &corev1.Secret{
			Name:      secretName,
			Namespace: fakeNamespace,
			Labels:    map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeCluster},
			Annotations: map[string]string{
				common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
			},
			Data: map[string][]byte{
				"name":   []byte(clusterName),
				"server": []byte(server),
				"config": []byte("{}"),
			},
		}
	}

	// Two secrets share the same cluster name
	kubeclientset := fake.NewClientset(
		emptyArgoCDConfigMap, argoCDSecret,
		makeClusterSecret("prod-cluster-1", clusterServer),
		makeClusterSecret("prod-cluster-2", "https://prod2.example.com"),
	)
	settingsManager := settings.NewSettingsManager(t.Context(), kubeclientset, fakeNamespace)
	argoDB := db.NewDB(fakeNamespace, settingsManager, kubeclientset)

	apps := []*v1alpha1.Application{
		{Spec: v1alpha1.ApplicationSpec{Destination: v1alpha1.ApplicationDestination{Name: clusterName}}},
	}

	updater := &clusterInfoUpdater{db: argoDB, namespace: fakeNamespace}
	cluster := v1alpha1.Cluster{Server: clusterServer}

	appCountByServer := updater.countAppsByDestinationServer(t.Context(), apps)
	info := getUpdatedClusterInfo(appCountByServer[cluster.Server], nil, metav1.Now())

	assert.Equal(t, int64(0), info.ApplicationsCount, "ambiguous name should not count app")
}

func TestUpdateClusterLabels(t *testing.T) {
	shouldNotBeInvoked := func(_ context.Context, _ *v1alpha1.Cluster) (*v1alpha1.Cluster, error) {
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
			func(_ context.Context, cluster *v1alpha1.Cluster) (*v1alpha1.Cluster, error) {
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
			func(_ context.Context, cluster *v1alpha1.Cluster) (*v1alpha1.Cluster, error) {
				assert.Equal(t, "1.28", cluster.Labels["argocd.argoproj.io/kubernetes-version"])
				return nil, errors.New("some error happened while saving")
			},
			assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantErr(t, updateClusterLabels(t.Context(), tt.clusterInfo, tt.cluster, tt.updateCluster), fmt.Sprintf("updateClusterLabels(%v, %v, %v)", t.Context(), tt.clusterInfo, tt.cluster))
		})
	}
}

// TestCountAppsByDestinationServer covers the destination shapes and the skip rules: server and
// name based destinations, a trailing slash, a namespace the project denies, a project lookup that
// fails, and destinations that don't resolve.
func TestCountAppsByDestinationServer(t *testing.T) {
	const (
		fakeNamespace  = "fake-ns"
		allowedNs      = "allowed-ns"
		deniedNs       = "denied-ns"
		prodServer     = "https://prod.example.com"
		stagingServer  = "https://staging.example.com"
		ambiguous1     = "https://ambiguous-1.example.com"
		ambiguous2     = "https://ambiguous-2.example.com"
		emptyServer    = "https://empty.example.com"
		unknownServer  = "https://unknown.example.com"
		prodName       = "prod"
		stagingName    = "staging"
		ambiguousName  = "ambiguous"
		missingProject = "missing"
	)

	emptyArgoCDConfigMap := &corev1.ConfigMap{
		Name:      common.ArgoCDConfigMapName,
		Namespace: fakeNamespace,
		Labels:    map[string]string{"app.kubernetes.io/part-of": "argocd"},
		Data:      map[string]string{},
	}
	argoCDSecret := &corev1.Secret{
		Name:      common.ArgoCDSecretName,
		Namespace: fakeNamespace,
		Labels:    map[string]string{"app.kubernetes.io/part-of": "argocd"},
		Data:      map[string][]byte{"admin.password": nil, "server.secretkey": nil},
	}
	clusterSecret := func(secretName, name, server string) *corev1.Secret {
		return &corev1.Secret{
			Name:      secretName,
			Namespace: fakeNamespace,
			Labels:    map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeCluster},
			Annotations: map[string]string{
				common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
			},
			Data: map[string][]byte{
				"name":   []byte(name),
				"server": []byte(server),
				"config": []byte("{}"),
			},
		}
	}

	kubeclientset := fake.NewClientset(
		emptyArgoCDConfigMap, argoCDSecret,
		clusterSecret("prod-cluster", prodName, prodServer),
		clusterSecret("staging-cluster", stagingName, stagingServer),
		// Two secrets share a name, so that name doesn't resolve.
		clusterSecret("ambiguous-cluster-1", ambiguousName, ambiguous1),
		clusterSecret("ambiguous-cluster-2", ambiguousName, ambiguous2),
		clusterSecret("empty-cluster", "empty", emptyServer),
	)
	settingsManager := settings.NewSettingsManager(t.Context(), kubeclientset, fakeNamespace)
	argoDB := db.NewDB(fakeNamespace, settingsManager, kubeclientset)

	app := func(name, namespace, project string, dest v1alpha1.ApplicationDestination) *v1alpha1.Application {
		return &v1alpha1.Application{
			Name:      name,
			Namespace: namespace,
			Spec:      v1alpha1.ApplicationSpec{Project: project, Destination: dest},
		}
	}
	apps := []*v1alpha1.Application{
		// Server based destinations.
		app("server-prod", fakeNamespace, "default", v1alpha1.ApplicationDestination{Server: prodServer}),
		app("server-prod-trailing-slash", fakeNamespace, "default", v1alpha1.ApplicationDestination{Server: prodServer + "/"}),
		app("server-staging", fakeNamespace, "default", v1alpha1.ApplicationDestination{Server: stagingServer}),
		// Name based destinations.
		app("name-prod", fakeNamespace, "default", v1alpha1.ApplicationDestination{Name: prodName}),
		app("name-staging", fakeNamespace, "default", v1alpha1.ApplicationDestination{Name: stagingName}),
		// Permitted source namespace.
		app("allowed-ns-prod", allowedNs, "default", v1alpha1.ApplicationDestination{Server: prodServer}),
		// Project does not permit the application namespace.
		app("denied-ns-prod", deniedNs, "default", v1alpha1.ApplicationDestination{Server: prodServer}),
		// projGetter returns an error.
		app("missing-project-prod", fakeNamespace, missingProject, v1alpha1.ApplicationDestination{Server: prodServer}),
		// Unresolvable destinations.
		app("both-name-and-server", fakeNamespace, "default", v1alpha1.ApplicationDestination{Name: prodName, Server: prodServer}),
		app("empty-destination", fakeNamespace, "default", v1alpha1.ApplicationDestination{}),
		app("unknown-name", fakeNamespace, "default", v1alpha1.ApplicationDestination{Name: "does-not-exist"}),
		app("ambiguous-name", fakeNamespace, "default", v1alpha1.ApplicationDestination{Name: ambiguousName}),
	}

	syncTime := time.Now()
	syncErr := errors.New("sync failed")
	clusters := []v1alpha1.Cluster{
		{Server: prodServer, Name: prodName},
		{Server: stagingServer, Name: stagingName},
		{Server: ambiguous1, Name: ambiguousName},
		{Server: emptyServer, Name: "empty"},
		{Server: unknownServer, Name: "unknown"},
	}
	infoByServer := map[string]*clustercache.ClusterInfo{
		prodServer:    {Server: prodServer, K8SVersion: "1.31", LastCacheSyncTime: &syncTime, APIsCount: 3, ResourcesCount: 7},
		stagingServer: {Server: stagingServer, K8SVersion: "1.30", LastCacheSyncTime: &syncTime, SyncError: syncErr},
		ambiguous1:    {Server: ambiguous1, K8SVersion: "1.29"},
		// emptyServer and unknownServer have no cluster cache info.
	}

	projGetter := func(a *v1alpha1.Application) (*v1alpha1.AppProject, error) {
		if a.Spec.Project == missingProject {
			return nil, errors.New("project not found")
		}
		return &v1alpha1.AppProject{
			Name:      a.Spec.Project,
			Namespace: fakeNamespace,
			Spec:      v1alpha1.AppProjectSpec{SourceNamespaces: []string{allowedNs}},
		}, nil
	}

	tests := []struct {
		name       string
		projGetter func(app *v1alpha1.Application) (*v1alpha1.AppProject, error)
		wantCounts map[string]int64
	}{
		{
			name:       "with project getter",
			projGetter: projGetter,
			wantCounts: map[string]int64{
				prodServer:    4,
				stagingServer: 2,
				ambiguous1:    0,
				emptyServer:   0,
				unknownServer: 0,
			},
		},
		{
			// A nil projGetter skips the project and namespace checks, so those apps get counted.
			name:       "nil project getter",
			projGetter: nil,
			wantCounts: map[string]int64{
				prodServer:    6,
				stagingServer: 2,
				ambiguous1:    0,
				emptyServer:   0,
				unknownServer: 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updater := &clusterInfoUpdater{db: argoDB, namespace: fakeNamespace, projGetter: tt.projGetter}
			now := metav1.Now()

			appCountByServer := updater.countAppsByDestinationServer(t.Context(), apps)

			for _, cluster := range clusters {
				got := getUpdatedClusterInfo(appCountByServer[cluster.Server], infoByServer[cluster.Server], now)

				assert.Equal(t, tt.wantCounts[cluster.Server], got.ApplicationsCount, "unexpected app count for %s", cluster.Server)
			}
		})
	}
}

type fakeClusterInfoSource struct {
	infos []clustercache.ClusterInfo
}

func (f *fakeClusterInfoSource) GetClustersInfo() []clustercache.ClusterInfo {
	return f.infos
}

// listOnlyDB implements just enough of db.ArgoDB for the benchmark, everything else panics.
// GetClusterServersByName does what the real one does for an ordinary name: a by-name index
// lookup plus a slice of the matches.
type listOnlyDB struct {
	db.ArgoDB
	clusters      *v1alpha1.ClusterList
	byNameIndexer cache.Indexer
}

func (l *listOnlyDB) ListClusters(_ context.Context) (*v1alpha1.ClusterList, error) {
	return l.clusters, nil
}

func (l *listOnlyDB) GetClusterServersByName(_ context.Context, name string) ([]string, error) {
	items, err := l.byNameIndexer.ByIndex(benchClusterByNameIndexer, name)
	if err != nil {
		return nil, err
	}
	servers := make([]string, 0, len(items))
	for _, item := range items {
		cluster, ok := item.(*v1alpha1.Cluster)
		if !ok {
			continue
		}
		servers = append(servers, cluster.Server)
	}
	return servers, nil
}

const benchClusterByNameIndexer = "byClusterName"

// BenchmarkClusterInfoUpdater_updateClusters measures one full tick. Both destination shapes
// are covered since a server resolves with a strings.TrimRight and a name costs an index lookup.
func BenchmarkClusterInfoUpdater_updateClusters(b *testing.B) {
	for _, byName := range []bool{false, true} {
		shape := "destinationByServer"
		if byName {
			shape = "destinationByName"
		}
		b.Run(shape, func(b *testing.B) {
			benchmarkUpdateClusters(b, byName)
		})
	}
}

func benchmarkUpdateClusters(b *testing.B, byName bool) {
	b.Helper()
	const (
		benchNamespace   = "argocd"
		benchNumClusters = 50
		benchNumApps     = 35000
	)

	syncTime := time.Now()
	clusters := &v1alpha1.ClusterList{}
	infos := make([]clustercache.ClusterInfo, 0, benchNumClusters)
	clusterIndexer := cache.NewIndexer(
		func(obj any) (string, error) { return obj.(*v1alpha1.Cluster).Server, nil },
		cache.Indexers{benchClusterByNameIndexer: func(obj any) ([]string, error) {
			return []string{obj.(*v1alpha1.Cluster).Name}, nil
		}},
	)
	for i := range benchNumClusters {
		server := fmt.Sprintf("https://cluster-%d.example.com", i)
		name := fmt.Sprintf("cluster-%d", i)
		clusters.Items = append(clusters.Items, v1alpha1.Cluster{Server: server, Name: name})
		require.NoError(b, clusterIndexer.Add(&v1alpha1.Cluster{Server: server, Name: name}))
		infos = append(infos, clustercache.ClusterInfo{
			Server:            server,
			K8SVersion:        "1.31",
			LastCacheSyncTime: &syncTime,
			APIsCount:         100,
			ResourcesCount:    5000,
		})
	}

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	for i := range benchNumApps {
		dest := v1alpha1.ApplicationDestination{}
		if byName {
			dest.Name = fmt.Sprintf("cluster-%d", i%benchNumClusters)
		} else {
			dest.Server = fmt.Sprintf("https://cluster-%d.example.com", i%benchNumClusters)
		}
		app := &v1alpha1.Application{
			Name:      fmt.Sprintf("app-%d", i),
			Namespace: benchNamespace,
			Spec: v1alpha1.ApplicationSpec{
				Project:     "default",
				Destination: dest,
			},
		}
		require.NoError(b, indexer.Add(app))
	}
	lister := applisters.NewApplicationLister(indexer).Applications(benchNamespace)

	proj := &v1alpha1.AppProject{Name: "default", Namespace: benchNamespace}
	projGetter := func(_ *v1alpha1.Application) (*v1alpha1.AppProject, error) { return proj, nil }

	appCache := appstate.NewCache(cacheutil.NewCache(cacheutil.NewInMemoryCache(time.Hour)), time.Hour)
	updater := NewClusterInfoUpdater(
		&fakeClusterInfoSource{infos: infos},
		&listOnlyDB{clusters: clusters, byNameIndexer: clusterIndexer},
		lister,
		appCache,
		nil,
		projGetter,
		benchNamespace,
	)

	b.ReportAllocs()
	for b.Loop() {
		// updateClusters is a no-op unless a full interval has elapsed since the last tick.
		updater.lastUpdated = time.Time{}
		updater.updateClusters()
	}
}

// countingAppLister records how many times updateClusters lists Applications.
type countingAppLister struct {
	applisters.ApplicationNamespaceLister
	calls int
}

func (l *countingAppLister) List(selector labels.Selector) ([]*v1alpha1.Application, error) {
	l.calls++
	return l.ApplicationNamespaceLister.List(selector)
}

// TestUpdateClusters_EmptyShardDoesNotListApplications asserts a shard managing no clusters never
// lists the applications. updateClusters runs every 10s.
func TestUpdateClusters_EmptyShardDoesNotListApplications(t *testing.T) {
	const (
		fakeNamespace = "fake-ns"
		shardServer   = "https://shard.example.com"
		otherServer   = "https://other.example.com"
	)

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for i := range 3 {
		require.NoError(t, indexer.Add(&v1alpha1.Application{
			Name:      fmt.Sprintf("app-%d", i),
			Namespace: fakeNamespace,
			Spec: v1alpha1.ApplicationSpec{
				Project:     "default",
				Destination: v1alpha1.ApplicationDestination{Server: shardServer},
			},
		}))
	}

	tests := []struct {
		name          string
		clusters      []v1alpha1.Cluster
		clusterFilter func(cluster *v1alpha1.Cluster) bool
		wantListCalls int
	}{
		{
			name:          "no clusters at all",
			clusters:      nil,
			wantListCalls: 0,
		},
		{
			name:          "every cluster filtered out",
			clusters:      []v1alpha1.Cluster{{Server: shardServer}, {Server: otherServer}},
			clusterFilter: func(_ *v1alpha1.Cluster) bool { return false },
			wantListCalls: 0,
		},
		{
			// Positive control, otherwise the two cases above pass with the listing removed.
			name:          "one cluster owned by this shard",
			clusters:      []v1alpha1.Cluster{{Server: shardServer}, {Server: otherServer}},
			clusterFilter: func(c *v1alpha1.Cluster) bool { return c.Server == shardServer },
			wantListCalls: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lister := &countingAppLister{
				ApplicationNamespaceLister: applisters.NewApplicationLister(indexer).Applications(fakeNamespace),
			}
			appCache := appstate.NewCache(cacheutil.NewCache(cacheutil.NewInMemoryCache(time.Minute)), time.Minute)
			updater := NewClusterInfoUpdater(
				&fakeClusterInfoSource{},
				&listOnlyDB{clusters: &v1alpha1.ClusterList{Items: tt.clusters}},
				lister,
				appCache,
				tt.clusterFilter,
				nil,
				fakeNamespace,
			)

			updater.updateClusters()

			assert.Equal(t, tt.wantListCalls, lister.calls)
		})
	}
}
