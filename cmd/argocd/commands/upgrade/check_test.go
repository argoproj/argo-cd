package upgrade

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func captureOutput(f func()) string {
	r, w, _ := os.Pipe()
	originalStdout := os.Stdout
	os.Stdout = w
	f()
	_ = w.Close()
	os.Stdout = originalStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestGetCheck_ValidType(t *testing.T) {
	check, err := getCheck("v2v3")

	require.NoError(t, err)
	assert.NotNil(t, check)

	_, ok := check.(*V2V3Check)
	assert.True(t, ok, "expected check to be of type *V2V3Check")
}

func TestGetCheck_InvalidType(t *testing.T) {
	check, err := getCheck("nonexistent")

	require.Error(t, err)
	assert.Nil(t, check)
	assert.Equal(t, "no checks found for this upgrade", err.Error())
}

func TestGetResult(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{checkPass, "PASS"},
		{checkFail, "FAIL"},
		{checkWarn, "WARN"},
		{checkInfo, "INFO"},
		{999, "UNKNOWN"}, // Unexpected value
		{-1, "UNKNOWN"},  // Negative value
	}

	for _, tt := range tests {
		result := getResult(tt.input)
		assert.Equal(t, tt.expected, result, "input: %d", tt.input)
	}
}

func TestPrintChecks(t *testing.T) {
	checks := []CheckResult{
		{
			title:       "Test Check",
			description: "Just a test",
			rules: []Rule{
				{
					title:   "Rule 1",
					actions: []string{"R1 test"},
					result:  checkPass,
				},
				{
					title:   "Rule 2",
					actions: []string{"R2 test"},
					result:  checkFail,
				},
				{
					title:   "Rule 3",
					actions: []string{"R3 test"},
					result:  checkWarn,
				},
				{
					title:   "Rule 4",
					actions: []string{"R4 test"},
					result:  checkInfo,
				},
			},
		},
	}

	output := captureOutput(func() {
		err := printChecks(checks)
		require.NoError(t, err)
	})

	assert.Contains(t, output, "Test Check")
	assert.Contains(t, output, "Just a test")
	assert.Contains(t, output, "Rule 1")
	assert.Contains(t, output, "Rule 2")
	assert.Contains(t, output, "Rule 3")
	assert.Contains(t, output, "Rule 4")
	assert.Contains(t, output, "PASS")
	assert.Contains(t, output, "FAIL")
	assert.Contains(t, output, "WARN")
	assert.Contains(t, output, "INFO")
	assert.Contains(t, output, "Upgrade Summary")
	assert.Contains(t, output, "Total Checks: 1")
	assert.Contains(t, output, "Total Rules: 4")
	assert.Contains(t, output, "Total Results:")
}

func TestPrintChecks_EmptyRules(t *testing.T) {
	checks := []CheckResult{
		{
			title:       "Test Check",
			description: "Just a test",
			rules:       []Rule{},
		},
	}

	output := captureOutput(func() {
		err := printChecks(checks)
		require.NoError(t, err)
	})

	assert.Contains(t, output, "Test Check")
	assert.Contains(t, output, "Just a test")
	assert.Contains(t, output, "No rules found for this check")
}

func TestPrintChecks_EmptyList(t *testing.T) {
	var capturedErr error
	_ = captureOutput(func() {
		capturedErr = printChecks([]CheckResult{})
	})

	require.Error(t, capturedErr)
	assert.Equal(t, "no checks found for this upgrade", capturedErr.Error())
}

func TestPrintBanner(t *testing.T) {
	output := captureOutput(func() {
		printBanner("=", 5)
	})

	assert.Equal(t, "=====\n", output)
}
