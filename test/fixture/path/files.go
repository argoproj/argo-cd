package path

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

// CopyDir copies the contents of a directory from 'src' to 'dest'
func CopyDir(src string, dest string) error {

	mode, err := os.Stat(src)
	if err != nil {
		return err
	}

	if mode.IsDir() {
		dirContents, err := ioutil.ReadDir(src)
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
