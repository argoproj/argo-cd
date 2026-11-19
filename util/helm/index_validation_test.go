package helm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func fullIndexFixtureString() string {
	return `apiVersion: v1
entries:
  demo:
    - version: 1.0.0
      name: demo
      urls: [demo-1.0.0.tgz]
generated: "2024-01-01T00:00:00Z"
`
}

// CR follow-up: an index followed by an EMPTY YAML document must stay
// acceptable — the pinned Helm loader also stops at the first document.
func TestParseIndexBodyAcceptsTrailingEmptyDocument(t *testing.T) {
	require.NoError(t, parseIndexBody([]byte(fullIndexFixtureString()+"---\n")))
	require.NoError(t, parseIndexBody([]byte(fullIndexFixtureString()+"---\n---\n")))
}

func TestParseIndexBodyRejectsTrailingDocument(t *testing.T) {
	require.Error(t, parseIndexBody([]byte(fullIndexFixtureString()+"---\nentries:\n  other: []\n")))
}
