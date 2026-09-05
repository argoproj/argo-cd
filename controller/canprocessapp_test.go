package controller

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/controller/sharding"
	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test"
)

// testCluster describes a cluster secret to seed the fake API server with.
type testCluster struct {
	secretName  string
	clusterName string
	server      string
	shard       *int
	annotations map[string]string
	// config overrides the cluster secret's config blob. The benchmarks use a realistic size since
	// that is what Cluster.DeepCopy charges for.
	config []byte
}

func (c testCluster) secret() *corev1.Secret {
	config := c.config
	if config == nil {
		config = []byte(`{"bearerToken":"fake","tlsClientConfig":{"insecure":true}}`)
	}
	data := map[string][]byte{
		"name":   []byte(c.clusterName),
		"server": []byte(c.server),
		"config": config,
	}
	if c.shard != nil {
		data["shard"] = []byte(strconv.Itoa(*c.shard))
	}
	return &corev1.Secret{
		Name:        c.secretName,
		Namespace:   test.FakeArgoCDNamespace,
		UID:         types.UID(c.secretName),
		Labels:      map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeCluster},
		Annotations: c.annotations,
		Data:        data,
	}
}

// newShardedController builds a controller backed by the given cluster secrets, with the sharding
// cache initialized from them the way ApplicationController.Run does.
func newShardedController(ctx context.Context, tb testing.TB, shard, replicas int, inClusterEnabled bool, clusters []testCluster, apps []runtime.Object) *ApplicationController {
	tb.Helper()

	additionalObjs := make([]runtime.Object, 0, len(clusters))
	for _, c := range clusters {
		additionalObjs = append(additionalObjs, c.secret())
	}
	configMapData := map[string]string{}
	if !inClusterEnabled {
		configMapData["cluster.inClusterEnabled"] = "false"
	}

	ctrl := newFakeController(ctx, &fakeData{
		apps:           apps,
		additionalObjs: additionalObjs,
		configMapData:  configMapData,
	}, nil)

	ctrl.clusterSharding = sharding.NewClusterSharding(nil, shard, replicas, common.DefaultShardingAlgorithm)
	clusterList, err := ctrl.db.ListClusters(ctx)
	require.NoError(tb, err)
	ctrl.clusterSharding.Init(clusterList, &appv1.ApplicationList{})
	return ctrl
}

func appWithDestination(name string, dest appv1.ApplicationDestination) *appv1.Application {
	return &appv1.Application{
		Name:      name,
		Namespace: test.FakeArgoCDNamespace,
		Spec: appv1.ApplicationSpec{
			Project:     "default",
			Destination: dest,
			Source:      &appv1.ApplicationSource{RepoURL: "https://github.com/argoproj/argocd-example-apps.git", Path: "some/path"},
		},
	}
}

// canProcessWant is the answer canProcessAppWithDestination is expected to give. server is only
// checked when process is true, which is when the collector uses it for the dest_server label.
type canProcessWant struct {
	process bool
	server  string
}

// Test_canProcessApp_ShardAssignment covers every destination shape against a two shard layout:
// resolvable by name and by server, a trailing slash, an ambiguous name, a cluster with no secret,
// a cluster the sharding cache has not caught up with, the skip-reconcile annotation on both the
// cluster and the Application, a disallowed namespace, and an object that is not an Application.
func Test_canProcessApp_ShardAssignment(t *testing.T) {
	shard0, shard1 := 0, 1
	clusters := []testCluster{
		{secretName: "cluster-a", clusterName: "cluster-a", server: "https://cluster-a", shard: &shard0},
		{secretName: "cluster-b", clusterName: "cluster-b", server: "https://cluster-b", shard: &shard1},
		{secretName: "cluster-skipped", clusterName: "cluster-skipped", server: "https://cluster-skipped", shard: &shard0, annotations: map[string]string{common.AnnotationKeyAppSkipReconcile: "true"}},
		// Two secrets share a name, so the name is ambiguous.
		{secretName: "dupe-1", clusterName: "dupe", server: "https://dupe-1", shard: &shard0},
		{secretName: "dupe-2", clusterName: "dupe", server: "https://dupe-2", shard: &shard1},
		{secretName: "cluster-forgotten", clusterName: "cluster-forgotten", server: "https://cluster-forgotten", shard: &shard1},
	}

	cases := []struct {
		name   string
		obj    any
		shard0 canProcessWant
		shard1 canProcessWant
	}{
		{
			"destination by name",
			appWithDestination("by-name-shard0", appv1.ApplicationDestination{Name: "cluster-a", Namespace: "default"}),
			canProcessWant{true, "https://cluster-a"},
			canProcessWant{false, ""},
		},
		{
			"destination by name, other shard",
			appWithDestination("by-name-shard1", appv1.ApplicationDestination{Name: "cluster-b", Namespace: "default"}),
			canProcessWant{false, ""},
			canProcessWant{true, "https://cluster-b"},
		},
		{
			"destination by server",
			appWithDestination("by-server", appv1.ApplicationDestination{Server: "https://cluster-b", Namespace: "default"}),
			canProcessWant{false, ""},
			canProcessWant{true, "https://cluster-b"},
		},
		{
			"destination by server with trailing slash",
			appWithDestination("by-server-slash", appv1.ApplicationDestination{Server: "https://cluster-b/", Namespace: "default"}),
			canProcessWant{false, ""},
			canProcessWant{true, "https://cluster-b"},
		},
		{
			// An unresolvable destination is processed by every shard.
			"both server and name set",
			appWithDestination("both-set", appv1.ApplicationDestination{Server: "https://cluster-a", Name: "cluster-a", Namespace: "default"}),
			canProcessWant{true, ""},
			canProcessWant{true, ""},
		},
		{
			"neither server nor name set",
			appWithDestination("neither-set", appv1.ApplicationDestination{Namespace: "default"}),
			canProcessWant{true, ""},
			canProcessWant{true, ""},
		},
		{
			"name of a cluster with no secret",
			appWithDestination("unknown-name", appv1.ApplicationDestination{Name: "no-such-cluster", Namespace: "default"}),
			canProcessWant{true, ""},
			canProcessWant{true, ""},
		},
		{
			"server of a cluster with no secret",
			appWithDestination("unknown-server", appv1.ApplicationDestination{Server: "https://no-such-cluster", Namespace: "default"}),
			canProcessWant{true, ""},
			canProcessWant{true, ""},
		},
		{
			"ambiguous name matching two clusters",
			appWithDestination("ambiguous", appv1.ApplicationDestination{Name: "dupe", Namespace: "default"}),
			canProcessWant{true, ""},
			canProcessWant{true, ""},
		},
		{
			"cluster with skip-reconcile annotation",
			appWithDestination("skipped-cluster", appv1.ApplicationDestination{Name: "cluster-skipped", Namespace: "default"}),
			canProcessWant{false, ""},
			canProcessWant{false, ""},
		},
		{
			// The sharding cache is made to forget this one below, which is the fallback path.
			"cluster the sharding cache does not know about",
			appWithDestination("forgotten-cluster", appv1.ApplicationDestination{Name: "cluster-forgotten", Namespace: "default"}),
			canProcessWant{true, "https://cluster-forgotten"},
			canProcessWant{false, ""},
		},
		{
			"application with skip-reconcile annotation",
			func() *appv1.Application {
				app := appWithDestination("skipped-app", appv1.ApplicationDestination{Name: "cluster-a", Namespace: "default"})
				app.Annotations = map[string]string{common.AnnotationKeyAppSkipReconcile: "true"}
				return app
			}(),
			canProcessWant{false, ""},
			canProcessWant{false, ""},
		},
		{
			"application with unparsable skip-reconcile annotation",
			func() *appv1.Application {
				app := appWithDestination("bad-annotation-app", appv1.ApplicationDestination{Name: "cluster-a", Namespace: "default"})
				app.Annotations = map[string]string{common.AnnotationKeyAppSkipReconcile: "not-a-bool"}
				return app
			}(),
			canProcessWant{true, "https://cluster-a"},
			canProcessWant{false, ""},
		},
		{
			"application in a namespace that is not allowed",
			func() *appv1.Application {
				app := appWithDestination("wrong-namespace-app", appv1.ApplicationDestination{Name: "cluster-a", Namespace: "default"})
				app.Namespace = "some-other-namespace"
				return app
			}(),
			canProcessWant{false, ""},
			canProcessWant{false, ""},
		},
		{
			"object that is not an application",
			&corev1.Secret{Name: "not-an-app"},
			canProcessWant{false, ""},
			canProcessWant{false, ""},
		},
	}

	for _, shard := range []int{0, 1} {
		ctrl := newShardedController(t.Context(), t, shard, 2, true, clusters, nil)
		ctrl.clusterSharding.Delete("https://cluster-forgotten")
		for _, tc := range cases {
			t.Run(fmt.Sprintf("shard=%d/%s", shard, tc.name), func(t *testing.T) {
				want := tc.shard0
				if shard == 1 {
					want = tc.shard1
				}

				got, gotServer := ctrl.canProcessAppWithDestination(tc.obj)
				assert.Equal(t, want.process, got)
				assert.Equal(t, want.process, ctrl.canProcessApp(tc.obj), "canProcessApp disagrees with canProcessAppWithDestination")
				if want.process {
					assert.Equal(t, want.server, gotServer, "dest_server label")
				}
			})
		}
	}
}

// Test_canProcessApp_InCluster covers the in-cluster destination, whose answer depends on both the
// shard and whether in-cluster is enabled.
func Test_canProcessApp_InCluster(t *testing.T) {
	shard0 := 0
	clusters := []testCluster{
		{secretName: "cluster-a", clusterName: "cluster-a", server: "https://cluster-a", shard: &shard0},
	}

	dests := map[string]appv1.ApplicationDestination{
		"by server": {Server: appv1.KubernetesInternalAPIServerAddr, Namespace: "default"},
		"by name":   {Name: appv1.KubernetesInClusterName, Namespace: "default"},
	}
	cases := []struct {
		shard            int
		inClusterEnabled bool
		want             canProcessWant
	}{
		{0, true, canProcessWant{true, appv1.KubernetesInternalAPIServerAddr}},
		{0, false, canProcessWant{true, ""}},
		{1, true, canProcessWant{false, ""}},
		{1, false, canProcessWant{true, ""}},
	}

	for _, tc := range cases {
		ctrl := newShardedController(t.Context(), t, tc.shard, 2, tc.inClusterEnabled, clusters, nil)
		for name, dest := range dests {
			t.Run(fmt.Sprintf("shard=%d/inClusterEnabled=%t/%s", tc.shard, tc.inClusterEnabled, name), func(t *testing.T) {
				got, gotServer := ctrl.canProcessAppWithDestination(appWithDestination("in-cluster", dest))
				assert.Equal(t, tc.want.process, got)
				assert.Equal(t, tc.want.process, ctrl.canProcessApp(appWithDestination("in-cluster", dest)))
				if tc.want.process {
					assert.Equal(t, tc.want.server, gotServer, "dest_server label")
				}
			})
		}
	}
}

// perfClusters and perfApps build the 35k Application, 500 cluster fixture for the benchmarks.
// Destinations are name based, the more expensive shape: a name index lookup on top of the
// cluster lookup.
const (
	benchmarkClusters = 500
	benchmarkApps     = 35000
)

// benchmarkClusterConfig returns a realistically sized config blob, a bearer token plus client
// certs. The size is what Cluster.DeepCopy charged for on every canProcessApp call.
func benchmarkClusterConfig() []byte {
	pem := func(kind string, size int) string {
		return base64.StdEncoding.EncodeToString([]byte("-----BEGIN " + kind + "-----\n" + strings.Repeat("A", size) + "\n-----END " + kind + "-----\n"))
	}
	return fmt.Appendf(nil, `{"bearerToken":%q,"tlsClientConfig":{"caData":%q,"certData":%q,"keyData":%q}}`,
		strings.Repeat("t", 900), pem("CERTIFICATE", 1200), pem("CERTIFICATE", 1200), pem("RSA PRIVATE KEY", 1600))
}

func benchmarkFixture(b *testing.B) *ApplicationController {
	b.Helper()

	// Building the fixture logs a line per cluster, which drowns out the benchmark output.
	previousLevel := log.GetLevel()
	log.SetLevel(log.ErrorLevel)
	b.Cleanup(func() { log.SetLevel(previousLevel) })

	clusters := make([]testCluster, 0, benchmarkClusters)
	for i := range benchmarkClusters {
		name := fmt.Sprintf("cluster-%d", i)
		clusters = append(clusters, testCluster{
			secretName:  name,
			clusterName: name,
			server:      fmt.Sprintf("https://cluster-%d.example.com", i),
			config:      benchmarkClusterConfig(),
		})
	}

	apps := make([]runtime.Object, 0, benchmarkApps)
	for i := range benchmarkApps {
		apps = append(apps, appWithDestination(fmt.Sprintf("app-%d", i), appv1.ApplicationDestination{
			Name:      fmt.Sprintf("cluster-%d", i%benchmarkClusters),
			Namespace: "default",
		}))
	}

	return newShardedController(b.Context(), b, 0, 1, true, clusters, apps)
}

// BenchmarkCanProcessApp measures the filter alone, one call per iteration, cycling through the
// Applications so every cluster gets hit.
func BenchmarkCanProcessApp(b *testing.B) {
	ctrl := benchmarkFixture(b)

	apps := make([]*appv1.Application, 0, benchmarkClusters)
	for i := range benchmarkClusters {
		apps = append(apps, appWithDestination(fmt.Sprintf("app-%d", i), appv1.ApplicationDestination{
			Name:      fmt.Sprintf("cluster-%d", i),
			Namespace: "default",
		}))
	}

	b.ReportAllocs()
	b.ResetTimer()
	i := 0
	for b.Loop() {
		if !ctrl.canProcessApp(apps[i%len(apps)]) {
			b.Fatal("expected application to be processed by this shard")
		}
		i++
	}
}

// BenchmarkAppCollector_Collect measures a full /metrics scrape over the 35k Application fixture,
// which is what Prometheus does every 30s per shard.
func BenchmarkAppCollector_Collect(b *testing.B) {
	ctrl := benchmarkFixture(b)

	req, err := http.NewRequestWithContext(b.Context(), http.MethodGet, "/metrics", http.NoBody)
	require.NoError(b, err)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		ctrl.metricsServer.Handler.ServeHTTP(&discardResponseWriter{}, req)
	}
}

// discardResponseWriter drops the scrape body so the benchmark measures collection and encoding,
// not the response buffer.
type discardResponseWriter struct {
	header http.Header
}

func (w *discardResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = http.Header{}
	}
	return w.header
}

func (w *discardResponseWriter) Write(b []byte) (int, error) { return len(b), nil }

func (w *discardResponseWriter) WriteHeader(int) {}
