package io

import (
	"os"
)

var (
	TempDir string
)

func init() {
	fileInfo, err := os.Stat("/dev/shm")
	if err == nil && fileInfo.IsDir() {
		TempDir = "/dev/shm"
	}
}

// DeleteFile is best effort deletion of a file
func DeleteFile(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return
	}
	_ = os.Remove(path)
}
