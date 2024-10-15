package metrics

import (
	"errors"
	"testing"

	argoappv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	gitopsCache "github.com/argoproj/gitops-engine/pkg/cache"
)

func TestMetricClusterConnectivity(t *testing.T) {
	type testCases struct {
		testCombination
		skip          bool
		description   string
		metricLabels  []string
		clusterLabels []string
		clustersInfo  []gitopsCache.ClusterInfo
		clusters      argoappv1.ClusterList
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
			clusters: argoappv1.ClusterList{
				Items: []argoappv1.Cluster{
					{
						Server: "server1",
						Name:   "cluster1",
						Labels: map[string]string{"env": "dev"},
					}, {
						Server: "server2",
						Name:   "cluster2",
						Labels: map[string]string{"env": "staging"},
					},
				},
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
			clusters: argoappv1.ClusterList{
				Items: []argoappv1.Cluster{
					{
						Server: "server1",
						Name:   "cluster1",
						Labels: map[string]string{"env": "dev"},
					}, {
						Server: "server2",
						Name:   "cluster2",
						Labels: map[string]string{"env": "staging"},
					},
				},
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
			clusters: argoappv1.ClusterList{
				Items: []argoappv1.Cluster{
					{
						Server: "server1",
						Name:   "cluster1",
						Labels: map[string]string{"env": "dev", "team": "team1"},
					}, {
						Server: "server2",
						Name:   "cluster2",
						Labels: map[string]string{"env": "staging", "team": "team2"},
					}, {
						Server: "server3",
						Name:   "cluster3",
						Labels: map[string]string{"env": "production", "team": "team3"},
					},
				},
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
					Clusters:         c.clusters,
				}
				runTest(t, cfg)
			}
		})
	}
}
