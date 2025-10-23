package commands

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appsv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// splitColumns splits a line produced by tabwriter using runs of 2 or more spaces
// as delimiters to obtain logical columns regardless of alignment padding.
func splitColumns(line string) []string {
	re := regexp.MustCompile(`\s{2,}`)
	return re.Split(strings.TrimSpace(line), -1)
}

func Test_printKeyTable_Empty(t *testing.T) {
	out, err := captureOutput(func() error {
		printKeyTable([]appsv1.GnuPGPublicKey{})
		return nil
	})
	require.NoError(t, err)

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	require.Len(t, lines, 1)

	headerCols := splitColumns(lines[0])
	assert.Equal(t, []string{"KEYID", "TYPE", "IDENTITY"}, headerCols)
}

func Test_printKeyTable_Single(t *testing.T) {
	keys := []appsv1.GnuPGPublicKey{
		{
			KeyID:   "ABCDEF1234567890",
			SubType: "rsa4096",
			Owner:   "Alice <alice@example.com>",
		},
	}

	out, err := captureOutput(func() error {
		printKeyTable(keys)
		return nil
	})
	require.NoError(t, err)

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	require.Len(t, lines, 2)

	// Header
	assert.Equal(t, []string{"KEYID", "TYPE", "IDENTITY"}, splitColumns(lines[0]))

	// Row
	row := splitColumns(lines[1])
	require.Len(t, row, 3)
	assert.Equal(t, "ABCDEF1234567890", row[0])
	assert.Equal(t, "RSA4096", row[1]) // subtype upper-cased
	assert.Equal(t, "Alice <alice@example.com>", row[2])
}

func Test_printKeyTable_Multiple(t *testing.T) {
	keys := []appsv1.GnuPGPublicKey{
		{
			KeyID:   "ABCD",
			SubType: "ed25519",
			Owner:   "User One <one@example.com>",
		},
		{
			KeyID:   "0123456789ABCDEF",
			SubType: "rsa2048",
			Owner:   "Second User <second@example.com>",
		},
	}

	out, err := captureOutput(func() error {
		printKeyTable(keys)
		return nil
	})
	require.NoError(t, err)

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	require.Len(t, lines, 3)

	// Header
	assert.Equal(t, []string{"KEYID", "TYPE", "IDENTITY"}, splitColumns(lines[0]))

	// First row
	row1 := splitColumns(lines[1])
	require.Len(t, row1, 3)
	assert.Equal(t, "ABCD", row1[0])
	assert.Equal(t, "ED25519", row1[1])
	assert.Equal(t, "User One <one@example.com>", row1[2])

	// Second row
	row2 := splitColumns(lines[2])
	require.Len(t, row2, 3)
	assert.Equal(t, "0123456789ABCDEF", row2[0])
	assert.Equal(t, "RSA2048", row2[1])
	assert.Equal(t, "Second User <second@example.com>", row2[2])
}
