package io

import (
	"io/fs"
	"path/filepath"
)

type subDirFs struct {
	dir string
	fs  fs.FS
}

func (s subDirFs) Open(name string) (fs.File, error) {
	return s.fs.Open(filepath.Join(s.dir, name))
}

// NewSubDirFS returns file system that represents sub-directory in a wrapped file system
func NewSubDirFS(dir string, fs fs.FS) *subDirFs {
	return &subDirFs{dir: dir, fs: fs}
}
