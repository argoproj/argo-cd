package io

import (
	"io"
	"io/fs"
)

func NewByteReadSeeker(data []byte) *byteReadSeeker {
	return &byteReadSeeker{data: data}
}

type byteReadSeeker struct {
	data   []byte
	offset int64
}

func (f *byteReadSeeker) Read(b []byte) (int, error) {
	if f.offset >= int64(len(f.data)) {
		return 0, io.EOF
	}
	n := copy(b, f.data[f.offset:])
	f.offset += int64(n)
	return n, nil
}

func (f *byteReadSeeker) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case 1:
		offset += f.offset
	case 2:
		offset += int64(len(f.data))
	}
	if offset < 0 || offset > int64(len(f.data)) {
		return 0, &fs.PathError{Op: "seek", Err: fs.ErrInvalid}
	}
	f.offset = offset
	return offset, nil
}
