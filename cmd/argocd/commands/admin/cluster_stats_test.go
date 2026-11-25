package admin

import (
    "bytes"
    "context"
    "fmt"
    "testing"

    "github.com/spf13/cobra"
    "github.com/stretchr/testify/assert"
)

// -----------------------------------------------------
// Mocked operational function
// -----------------------------------------------------
func executeClusterStats(_ context.Context, _ interface{}, shard, replicas int) ([]string, error) {
    clusters := []struct {
        Server           string
        Shard            int
        ConnectionStatus string
        NamespacesCount  int
        AppsCount        int
        ResourcesCount   int
    }{
        {"https://cluster1", 0, "Healthy", 2, 3, 10},
        {"https://cluster2", 1, "Degraded", 1, 1, 5},
    }

    lines := []string{"SERVER\tSHARD\tCONNECTION\tNAMESPACES COUNT\tAPPS COUNT\tRESOURCES COUNT"}
    for _, c := range clusters {
        line := fmt.Sprintf("%s\t%d\t%s\t%d\t%d\t%d",
            c.Server, c.Shard, c.ConnectionStatus, c.NamespacesCount, c.AppsCount, c.ResourcesCount)
        lines = append(lines, line)
    }

    return lines, nil
}

// -----------------------------------------------------
// Unit test for the operational function
// -----------------------------------------------------
func TestExecuteClusterStats(t *testing.T) {
    lines, err := executeClusterStats(context.Background(), nil, 0, 1)
    assert.NoError(t, err)
    assert.GreaterOrEqual(t, len(lines), 2)
    assert.Contains(t, lines[0], "SERVER")
    assert.Contains(t, lines[1], "cluster1")
}

// -----------------------------------------------------
// Fake Cobra command for testing RunE
// -----------------------------------------------------
func fakeClusterStatsCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use: "cluster-stats",
        RunE: func(cmd *cobra.Command, args []string) error {
            lines, err := executeClusterStats(context.Background(), nil, 0, 1)
            if err != nil {
                return err
            }
            for _, line := range lines {
                fmt.Fprintln(cmd.OutOrStdout(), line)
            }
            return nil
        },
    }
    return cmd
}

// -----------------------------------------------------
// Integration test for the RunE CLI behavior
// -----------------------------------------------------
func TestClusterStatsCommand_RunE_Mock(t *testing.T) {
    cmd := fakeClusterStatsCommand()

    buf := new(bytes.Buffer)
    cmd.SetOut(buf)
    cmd.SetErr(buf)
    cmd.SetArgs([]string{})

    err := cmd.Execute()
    assert.NoError(t, err)

    output := buf.String()
    assert.Contains(t, output, "SERVER")
    assert.Contains(t, output, "Healthy")
    assert.Contains(t, output, "Degraded")
}
