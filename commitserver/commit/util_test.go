package commit

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMakeSecureTempDir(t *testing.T) {
	tempDir, cleanup, err := makeSecureTempDir()	

	require.NoError(t, err)
	defer cleanup()

	// Verify that the temporary directory exists
	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		t.Errorf("Temporary directory does not exist: %s", tempDir)
	}

	// Verify that the temporary directory is a directory
	fileInfo, err := os.Stat(tempDir)
	if err != nil {
		t.Fatalf("Failed to get file info for temporary directory: %v", err)
	}
	if !fileInfo.IsDir() {
		t.Errorf("Temporary directory is not a directory: %s", tempDir)
	}

	// Verify that the temporary directory is empty
	files, err := ioutil.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("Temporary directory is not empty: %s", tempDir)
	}
}