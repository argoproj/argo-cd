package path

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// CopyDir copies the contents of a directory from 'src' to 'dest'
func CopyDir(src string, dest string) error {
	mode, err := os.Stat(src)
	if err != nil {
		return err
	}

	if mode.IsDir() {
		dirContents, err := os.ReadDir(src)
		if err != nil {
			return err
		}

		for _, f := range dirContents {
			srcFileaPath := filepath.Join(src, f.Name())
			destFilePath := filepath.Join(dest, f.Name())
			if err := CopyDir(srcFileaPath, destFilePath); err != nil {
				return err
			}
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dest), os.ModePerm); err != nil {
		return err
	}

	return copySingleFile(src, dest, mode)
}

func copySingleFile(src string, dest string, mode os.FileInfo) error {
	if src == dest {
		return nil
	}

	// Ensure the file is not a directory
	file, err := os.Stat(src)
	if err != nil {
		return err
	}
	if file.IsDir() {
		return fmt.Errorf("unable to copy directories: %s", src)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()

	err = os.Chmod(destFile.Name(), mode.Mode())
	if err != nil {
		return err
	}

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return err
	}

	return nil
}

// CreateSymlink creates a symlink with name linkName to file destName in
// workingDir
func CreateSymlink(t *testing.T, workingDir, destName, linkName string) error {
	t.Helper()
	oldWorkingDir, err := os.Getwd()
	if err != nil {
		return err
	}
	if workingDir != "" {
		err = os.Chdir(workingDir)
		if err != nil {
			return err
		}
		defer func() {
			if err := os.Chdir(oldWorkingDir); err != nil {
				t.Fatal(err.Error())
			}
		}()
	}
	err = os.Symlink(destName, linkName)
	if err != nil {
		return err
	}
	return nil
}
