package configbus

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteReferenceDoc(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, WriteReferenceDoc(&buf))
	out := buf.String()
	assert.Contains(t, out, NameReconciliationTimeout)
	assert.Contains(t, out, NameResourceCustomizations)
	assert.Contains(t, out, CMKeyTimeoutReconciliation)
	assert.True(t, strings.HasPrefix(out, "# Argo CD config registry"))
}
