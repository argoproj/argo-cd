package commands

import (
<<<<<<< HEAD
=======
	"bytes"
	"io"
	"os"
	"regexp"
	"strings"
>>>>>>> 4fc69c5276 (feat: add sync overrun option to sync windows)
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestPrintSyncWindows(t *testing.T) {
<<<<<<< HEAD
	proj := &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: "test-project"},
		Spec: v1alpha1.AppProjectSpec{
			SyncWindows: v1alpha1.SyncWindows{
				{
					Kind:           "allow",
					Schedule:       "* * * * *",
					Duration:       "1h",
					Applications:   []string{"app1"},
					Namespaces:     []string{"ns1"},
					Clusters:       []string{"cluster1"},
					ManualSync:     true,
					UseAndOperator: true,
				},
			},
		},
	}

	output, err := captureOutput(func() error {
		printSyncWindows(proj)
		return nil
	})
	require.NoError(t, err)
	t.Log(output)
	assert.Contains(t, output, "ID  STATUS  KIND   SCHEDULE   DURATION  APPLICATIONS  NAMESPACES  CLUSTERS  MANUALSYNC  TIMEZONE  USEANDOPERATOR")
	assert.Contains(t, output, "0   Active  allow  * * * * *  1h        app1          ns1         cluster1  Enabled               Enabled")
=======
	tests := []struct {
		name           string
		project        *v1alpha1.AppProject
		expectedHeader []string
		expectedRows   [][]string
	}{
		{
			name: "Project with multiple sync windows including syncOverrun",
			project: &v1alpha1.AppProject{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-project",
				},
				Spec: v1alpha1.AppProjectSpec{
					SyncWindows: v1alpha1.SyncWindows{
						{
							Kind:           "allow",
							Schedule:       "0 0 * * *",
							Duration:       "1h",
							Applications:   []string{"app1", "app2"},
							Namespaces:     []string{"default"},
							Clusters:       []string{"cluster1"},
							ManualSync:     false,
							SyncOverrun:    false,
							TimeZone:       "UTC",
							UseAndOperator: false,
						},
						{
							Kind:           "deny",
							Schedule:       "0 12 * * *",
							Duration:       "2h",
							Applications:   []string{"*"},
							Namespaces:     []string{"production"},
							Clusters:       []string{"*"},
							ManualSync:     true,
							SyncOverrun:    true,
							TimeZone:       "America/New_York",
							UseAndOperator: true,
						},
					},
				},
			},
			expectedHeader: []string{"ID", "STATUS", "KIND", "SCHEDULE", "DURATION", "APPLICATIONS", "NAMESPACES", "CLUSTERS", "MANUALSYNC", "SYNCOVERRUN", "TIMEZONE", "USEANDOPERATOR"},
			expectedRows: [][]string{
				{"0", "Inactive", "allow", "0 0 * * *", "1h", "app1,app2", "default", "cluster1", "Disabled", "Disabled", "UTC", "Disabled"},
				{"1", "Inactive", "deny", "0 12 * * *", "2h", "*", "production", "*", "Enabled", "Enabled", "America/New_York", "Enabled"},
			},
		},
		{
			name: "Project with empty sync window lists",
			project: &v1alpha1.AppProject{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-project",
				},
				Spec: v1alpha1.AppProjectSpec{
					SyncWindows: v1alpha1.SyncWindows{
						{
							Kind:           "allow",
							Schedule:       "0 1 * * *",
							Duration:       "30m",
							Applications:   []string{},
							Namespaces:     []string{},
							Clusters:       []string{},
							ManualSync:     false,
							SyncOverrun:    false,
							TimeZone:       "UTC",
							UseAndOperator: false,
						},
					},
				},
			},
			expectedHeader: []string{"ID", "STATUS", "KIND", "SCHEDULE", "DURATION", "APPLICATIONS", "NAMESPACES", "CLUSTERS", "MANUALSYNC", "SYNCOVERRUN", "TIMEZONE", "USEANDOPERATOR"},
			expectedRows: [][]string{
				{"0", "Inactive", "allow", "0 1 * * *", "30m", "-", "-", "-", "Disabled", "Disabled", "UTC", "Disabled"},
			},
		},
		{
			name: "Project with no sync windows",
			project: &v1alpha1.AppProject{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-project",
				},
				Spec: v1alpha1.AppProjectSpec{
					SyncWindows: v1alpha1.SyncWindows{},
				},
			},
			expectedHeader: []string{"ID", "STATUS", "KIND", "SCHEDULE", "DURATION", "APPLICATIONS", "NAMESPACES", "CLUSTERS", "MANUALSYNC", "SYNCOVERRUN", "TIMEZONE", "USEANDOPERATOR"},
			expectedRows:   [][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Call the function
			printSyncWindows(tt.project)

			// Restore stdout
			w.Close()
			os.Stdout = oldStdout

			// Read captured output
			var buf bytes.Buffer
			_, err := io.Copy(&buf, r)
			require.NoError(t, err)
			output := buf.String()

			// Parse the table output
			lines := strings.Split(strings.TrimSpace(output), "\n")
			assert.GreaterOrEqual(t, len(lines), 1, "Should have at least a header line")

			// Parse header line (split by whitespace for headers since they don't contain spaces)
			headerLine := lines[0]
			headerFields := strings.Fields(headerLine)
			assert.Len(t, headerFields, len(tt.expectedHeader), "Header should have correct number of columns")
			assert.Equal(t, tt.expectedHeader, headerFields, "Header columns should match expected")

			// Parse data rows
			dataLines := lines[1:]
			assert.Len(t, dataLines, len(tt.expectedRows), "Should have expected number of data rows")

			for i, dataLine := range dataLines {
				// Split by 2 or more spaces (tabwriter output uses multiple spaces as separators)
				re := regexp.MustCompile(`\s{2,}`)
				fields := re.Split(strings.TrimSpace(dataLine), -1)

				assert.Len(t, fields, len(tt.expectedRows[i]), "Row %d should have correct number of columns", i)

				for j, expectedValue := range tt.expectedRows[i] {
					assert.Equal(t, expectedValue, fields[j], "Row %d, column %d should match expected value", i, j)
				}
			}
		})
	}
}

func TestFormatListOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected string
	}{
		{
			name:     "Empty list",
			input:    []string{},
			expected: "-",
		},
		{
			name:     "Single item",
			input:    []string{"app1"},
			expected: "app1",
		},
		{
			name:     "Multiple items",
			input:    []string{"app1", "app2", "app3"},
			expected: "app1,app2,app3",
		},
		{
			name:     "Wildcard",
			input:    []string{"*"},
			expected: "*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatListOutput(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatBoolOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    bool
		expected string
	}{
		{
			name:     "Active",
			input:    true,
			expected: "Active",
		},
		{
			name:     "Inactive",
			input:    false,
			expected: "Inactive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatBoolOutput(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatBoolEnabledOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    bool
		expected string
	}{
		{
			name:     "Enabled",
			input:    true,
			expected: "Enabled",
		},
		{
			name:     "Disabled",
			input:    false,
			expected: "Disabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatBoolEnabledOutput(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
>>>>>>> 4fc69c5276 (feat: add sync overrun option to sync windows)
}
