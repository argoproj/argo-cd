package metrics

import (
	"errors"
	"testing"

	gitopsCache "github.com/argoproj/gitops-engine/pkg/cache"
)

func TestMetricClusterConnectivity(t *testing.T) {
	type testCases struct {
		testCombination
		skip         bool
		description  string
		metricLabels []string
		clustersInfo []gitopsCache.ClusterInfo
	}
	cases := []testCases{
		{
			description:  "metric will have value 1 if connected with the cluster",
			skip:         false,
			metricLabels: []string{"non-existing"},
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
			description:  "metric will have value 0 if not connected with the cluster",
			skip:         false,
			metricLabels: []string{"non-existing"},
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
			description:  "will have one metric per cluster",
			skip:         false,
			metricLabels: []string{"non-existing"},
			testCombination: testCombination{
				applications: []string{fakeApp},
				responseContains: `
# TYPE argocd_cluster_connection_status gauge
argocd_cluster_connection_status{k8s_version="1.21",server="server1"} 1
argocd_cluster_connection_status{k8s_version="1.21",server="server2"} 1
argocd_cluster_connection_status{k8s_version="1.21",server="server3"} 1
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
					ClustersInfo:     c.clustersInfo,
				}
				runTest(t, cfg)
			}
		})
	}
}
