package io

import (
	"os"
)

// TempDir is set to '/dev/shm' if exists, otherwise is "", which defaults to os.TempDir() when passed to os.CreateTemp()
var TempDir string

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
