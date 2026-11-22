package helm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const parseIndexFixture = "apiVersion: v1\nentries:\n  child:\n    - name: child\n      version: 1.0.0\n"

func TestParseIndexBodyAcceptsTrailingEmptyDocument(t *testing.T) {
	require.NoError(t, parseIndexBody([]byte(parseIndexFixture+"---\n")))
	require.NoError(t, parseIndexBody([]byte(parseIndexFixture+"---\n---\n")))
}

func TestParseIndexBodyRejectsTrailingDocument(t *testing.T) {
	require.Error(t, parseIndexBody([]byte(parseIndexFixture+"---\nentries:\n  other: []\n")))
}
