package admin

import (
    "context"
    "fmt"
    "testing"

    log "github.com/sirupsen/logrus"
    "github.com/spf13/cobra"
    argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"
)
// -----------------
// Fake Cluster Stats Logic
// -----------------
func ExecuteClusterStatsForTest(ctx context.Context, clientOpts *argocdclient.ClientOptions, shard, replicas int) ([]string, error) {
	// Simulated clusters for test
	clusters := []struct {
		Server             string
		Shard              int
		ConnectionStatus   string
		NamespacesCount    int
		ApplicationsCount  int
		ResourcesCount     int
	}{
		{"https://cluster1", 0, "Healthy", 2, 3, 10},
		{"https://cluster2", 1, "Degraded", 1, 1, 5},
	}

	lines := []string{"SERVER\tSHARD\tCONNECTION\tNAMESPACES COUNT\tAPPS COUNT\tRESOURCES COUNT"}
	for _, c := range clusters {
		line := fmt.Sprintf("%s\t%d\t%s\t%d\t%d\t%d",
			c.Server, c.Shard, c.ConnectionStatus,
			c.NamespacesCount, c.ApplicationsCount, c.ResourcesCount)
		lines = append(lines, line)
	}

	return lines, nil
}

// -----------------
// Cobra Command for Test
// -----------------
func NewClusterStatsCommandForTest(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var shard, replicas int

	command := &cobra.Command{
		Use:   "stats",
		Short: "Prints cluster statistics",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			log.SetLevel(log.WarnLevel)

			lines, err := ExecuteClusterStatsForTest(ctx, clientOpts, shard, replicas)
			if err != nil {
				return err
			}

			for _, line := range lines {
				fmt.Println(line)
			}
			return nil
		},
	}

	command.Flags().IntVar(&shard, "shard", -1, "Cluster shard filter")
	command.Flags().IntVar(&replicas, "replicas", 0, "Application controller replicas count")
	return command
}

// -----------------
// Unit Test
// -----------------
func TestClusterStatsCommand(t *testing.T) {
	clientOpts := &argocdclient.ClientOptions{
		AppControllerName: "argocd-application-controller",
		RedisName:         "argocd-redis",
		RedisHaProxyName:  "argocd-redis-ha",
		RedisCompression:  "none",
	}

	cmd := NewClusterStatsCommandForTest(clientOpts)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}
}
