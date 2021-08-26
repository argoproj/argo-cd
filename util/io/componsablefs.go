package io

import "io/fs"

type composableFS struct {
	innerFS []fs.FS
}

// NewComposableFS creates files system that attempts reading file from multiple wrapped file systems
func NewComposableFS(innerFS ...fs.FS) *composableFS {
	return &composableFS{innerFS: innerFS}
}

// Open attempts open file in wrapped file systems and returns first successful
func (c composableFS) Open(name string) (f fs.File, err error) {
	for i := range c.innerFS {
		f, err = c.innerFS[i].Open(name)
		if err == nil {
			break
		}
	}
	return
}
