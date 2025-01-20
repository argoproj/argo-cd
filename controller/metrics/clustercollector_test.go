package metrics

import (
	"errors"
	"testing"

	gitopsCache "github.com/argoproj/gitops-engine/pkg/cache"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"

	argocommon "github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/test"
)

func createSecret(name, namespace, env, team, server, config string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				argocommon.LabelKeySecretType: argocommon.LabelValueSecretTypeCluster,
				"env":                         env,
				"team":                        team,
			},
		},
		Data: map[string][]byte{
			"name":   []byte(name),
			"server": []byte(server),
			"config": []byte(config),
		},
	}
}

func TestMetricClusterConnectivity(t *testing.T) {
	argoCDNamespace := test.FakeArgoCDNamespace

	cm := test.NewFakeConfigMap()
	secret := test.NewFakeSecret()

	secret1 := createSecret("cluster1", argoCDNamespace, "dev", "team1", "server1", "{\"username\":\"foo\",\"password\":\"foo\"}")
	secret2 := createSecret("cluster2", argoCDNamespace, "staging", "team2", "server2", "{\"username\":\"bar\",\"password\":\"bar\"}")
	secret3 := createSecret("cluster3", argoCDNamespace, "production", "team3", "server3", "{\"username\":\"baz\",\"password\":\"baz\"}")
	objects := append([]runtime.Object{}, cm, secret, secret1, secret2, secret3)
	kubeClientset := kubefake.NewClientset(objects...)

	type testCases struct {
		testCombination
		skip          bool
		description   string
		metricLabels  []string
		clusterLabels []string
		clustersInfo  []gitopsCache.ClusterInfo
	}

	cases := []testCases{
		{
			description:   "metric will have value 1 if connected with the cluster",
			skip:          false,
			metricLabels:  []string{"non-existing"},
			clusterLabels: []string{"env"},
			testCombination: testCombination{
				applications: []string{fakeApp},
				responseContains: `
# TYPE argocd_cluster_connection_status gauge
argocd_cluster_connection_status{k8s_version="1.21",server="server1"} 1
`,
			},
			clustersInfo: []gitopsCache.ClusterInfo{
				{
					Server:     "server1",
					K8SVersion: "1.21",
					SyncError:  nil,
				},
			},
		},
		{
			description:   "metric will have value 0 if not connected with the cluster",
			skip:          false,
			metricLabels:  []string{"non-existing"},
			clusterLabels: []string{"env"},
			testCombination: testCombination{
				applications: []string{fakeApp},
				responseContains: `
# TYPE argocd_cluster_connection_status gauge
argocd_cluster_connection_status{k8s_version="1.21",server="server1"} 0
`,
			},
			clustersInfo: []gitopsCache.ClusterInfo{
				{
					Server:     "server1",
					K8SVersion: "1.21",
					SyncError:  errors.New("error connecting with cluster"),
				},
			},
		},
		{
			description:   "will have one metric per cluster",
			skip:          false,
			metricLabels:  []string{"non-existing"},
			clusterLabels: []string{"env", "team"},
			testCombination: testCombination{
				applications: []string{fakeApp},
				responseContains: `
# TYPE argocd_cluster_connection_status gauge
argocd_cluster_connection_status{k8s_version="1.21",server="server1"} 1
argocd_cluster_connection_status{k8s_version="1.21",server="server2"} 1
argocd_cluster_connection_status{k8s_version="1.21",server="server3"} 1

# TYPE argocd_cluster_info gauge
argocd_cluster_info{k8s_version="1.21",name="cluster1",server="server1"} 1
argocd_cluster_info{k8s_version="1.21",name="cluster2",server="server2"} 1
argocd_cluster_info{k8s_version="1.21",name="cluster3",server="server3"} 1

# TYPE argocd_cluster_labels gauge
argocd_cluster_labels{label_env="dev",label_team="team1",name="cluster1",server="server1"} 1
argocd_cluster_labels{label_env="staging",label_team="team2",name="cluster2",server="server2"} 1
argocd_cluster_labels{label_env="production",label_team="team3",name="cluster3",server="server3"} 1
`,
			},
			clustersInfo: []gitopsCache.ClusterInfo{
				{
					Server:     "server1",
					K8SVersion: "1.21",
					SyncError:  nil,
				},
				{
					Server:     "server2",
					K8SVersion: "1.21",
					SyncError:  nil,
				},
				{
					Server:     "server3",
					K8SVersion: "1.21",
					SyncError:  nil,
				},
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.description, func(t *testing.T) {
			if !c.skip {
				cfg := TestMetricServerConfig{
					FakeAppYAMLs:     c.applications,
					ExpectedResponse: c.responseContains,
					AppLabels:        c.metricLabels,
					ClusterLabels:    c.clusterLabels,
					ClustersInfo:     c.clustersInfo,
					KubeClientset:    kubeClientset,
					ArgoCDNamespace:  argoCDNamespace,
				}
				runTest(t, cfg)
			}
		})
	}
}
