package io

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"testing"
)

func TestByteReadSeeker_Read(t *testing.T) {
	inString := "hello world"
	reader := NewByteReadSeeker([]byte(inString))
	var bytes = make([]byte, 11)
	n, err := reader.Read(bytes)
	require.NoError(t, err)
	assert.Equal(t, len(inString), n)
	assert.Equal(t, inString, string(bytes))
	_, err = reader.Read(bytes)
	assert.ErrorIs(t, err, io.EOF)
}

func TestByteReadSeeker_Seek_Start(t *testing.T) {
	inString := "hello world"
	reader := NewByteReadSeeker([]byte(inString))
	offset, err := reader.Seek(6, io.SeekStart)
	require.NoError(t, err)
	assert.Equal(t, int64(6), offset)
	var bytes = make([]byte, 5)
	n, err := reader.Read(bytes)
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, "world", string(bytes))
}

func TestByteReadSeeker_Seek_Current(t *testing.T) {
	inString := "hello world"
	reader := NewByteReadSeeker([]byte(inString))
	offset, err := reader.Seek(3, io.SeekCurrent)
	require.NoError(t, err)
	assert.Equal(t, int64(3), offset)
	offset, err = reader.Seek(3, io.SeekCurrent)
	require.NoError(t, err)
	assert.Equal(t, int64(6), offset)
	var bytes = make([]byte, 5)
	n, err := reader.Read(bytes)
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, "world", string(bytes))
}

func TestByteReadSeeker_Seek_End(t *testing.T) {
	inString := "hello world"
	reader := NewByteReadSeeker([]byte(inString))
	offset, err := reader.Seek(-5, io.SeekEnd)
	require.NoError(t, err)
	assert.Equal(t, int64(6), offset)
	var bytes = make([]byte, 5)
	n, err := reader.Read(bytes)
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, "world", string(bytes))
}

func TestByteReadSeeker_Seek_OutOfBounds(t *testing.T) {
	inString := "hello world"
	reader := NewByteReadSeeker([]byte(inString))
	_, err := reader.Seek(12, io.SeekStart)
	assert.Error(t, err)
	_, err = reader.Seek(-1, io.SeekStart)
	assert.Error(t, err)
}
