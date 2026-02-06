package path

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type AddBinDirToPath struct {
	originalPath string
}

func (h AddBinDirToPath) Close() {
	_ = os.Setenv("PATH", h.originalPath)
}

// add the hack path which has the argocd binary
func NewBinDirToPath(t *testing.T) AddBinDirToPath {
	t.Helper()
	originalPath := os.Getenv("PATH")
	binDir, err := filepath.Abs("../../dist")
	require.NoError(t, err)
	t.Setenv("PATH", fmt.Sprintf("%s:%s", originalPath, binDir))
	return AddBinDirToPath{originalPath}
}
