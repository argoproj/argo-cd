package rand

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRandString(t *testing.T) {
	ss, err := StringFromCharset(10, "A")
	require.NoError(t, err)
	assert.Equal(t, "AAAAAAAAAA", ss)

	ss, err = StringFromCharset(5, "ABC123")
	require.NoError(t, err)
	assert.Len(t, ss, 5)
}
