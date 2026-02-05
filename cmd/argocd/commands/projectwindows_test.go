package commands

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrintSyncWindows(t *testing.T) {
	// Mock AppProject with SyncWindows
	proj := &v1alpha1.AppProject{
		Spec: v1alpha1.AppProjectSpec{
			SyncWindows: v1alpha1.SyncWindows{
				{
					Kind:         "deny",
					Schedule:     "* * * * *",
					Duration:     "30m",
					Applications: []string{"app1", "app2"},
					Namespaces:   []string{"namespace1"},
					Clusters:     []string{"cluster1"},
					ManualSync:   true,
					TimeZone:     "UTC",
				},
			},
		},
	}

	// Test wide output
	output, err := captureTestOutput(func() error {
		printSyncWindows(proj)
		return nil
	})
	fmt.Println("Wide Output:\n", output)
	require.NoError(t, err)
	assert.Contains(t, output, "ID")
	assert.Contains(t, output, "deny")
	assert.Contains(t, output, "30m")

	// Test json output
	jsonOutput, err := captureTestOutput(func() error {
		outputs := collectSyncWindowData(proj)
		return PrintResourceList(outputs, "json", false)
	})
	fmt.Println("JSON Output:\n", jsonOutput)
	require.NoError(t, err)
	assert.Contains(t, jsonOutput, `"Kind": "deny"`)
	assert.Contains(t, jsonOutput, `"Duration": "30m"`)

	// Test yaml output
	yamlOutput, err := captureTestOutput(func() error {
		outputs := collectSyncWindowData(proj)
		return PrintResourceList(outputs, "yaml", false)
	})
	fmt.Println("YAML Output:\n", yamlOutput)
	require.NoError(t, err)
	assert.Contains(t, yamlOutput, "Kind: deny")
	assert.Contains(t, yamlOutput, "Duration: 30m")
}

// Helper function to capture stdout
func captureTestOutput(f func() error) (string, error) {
	var buf bytes.Buffer
	stdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := f()

	_ = w.Close()
	os.Stdout = stdout
	_, _ = buf.ReadFrom(r)
	return buf.String(), err
}
