package commit

import (
	"fmt"
	"os"
	"path"

	"github.com/google/uuid"
)

// makeSecureTempDir creates a secure temporary directory and returns the path to the directory. The path is "secure" in
// the sense that its name is a UUID, which helps mitigate path traversal attacks. The function also returns a cleanup
// function that should be used to remove the directory when it is no longer needed.
func makeSecureTempDir() (string, func() error, error) {
	// The UUID is an important security mechanism to help mitigate path traversal attacks.
	dirName, err := uuid.NewRandom()
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate uuid: %w", err)
	}
	// Don't need SecureJoin here, both parts are safe.
	dirPath := path.Join("/tmp/_commit-service", dirName.String())
	err = os.MkdirAll(dirPath, os.ModePerm)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	cleanup := func() error {
		err := os.RemoveAll(dirPath)
		if err != nil {
			return fmt.Errorf("failed to remove temp dir: %w", err)
		}
		return nil
	}
	return dirPath, cleanup, nil
}
