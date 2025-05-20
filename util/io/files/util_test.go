package files_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/v2/util/io/files"
)

func TestRelativePath(t *testing.T) {
	type testcase struct {
		name        string
		fullpath    string
		basepath    string
		expected    string
		expectedErr error
	}
	cases := []testcase{
		{
			name:        "will return relative path from file path",
			fullpath:    "/home/test/app/readme.md",
			basepath:    "/home/test",
			expected:    "app/readme.md",
			expectedErr: nil,
		},
		{
			name:        "will return relative path from dir path",
			fullpath:    "/home/test/app/",
			basepath:    "/home/test",
			expected:    "app",
			expectedErr: nil,
		},
		{
			name:        "will return . if fullpath and basepath are the same",
			fullpath:    "/home/test/app/readme.md",
			basepath:    "/home/test/app/readme.md",
			expected:    ".",
			expectedErr: nil,
		},
		{
			name:        "will return error if basepath does not match",
			fullpath:    "/home/test/app/readme.md",
			basepath:    "/somewhere/else",
			expected:    "",
			expectedErr: files.RelativeOutOfBoundErr,
		},
		{
			name:        "will return relative path from dir path",
			fullpath:    "/home/test//app/",
			basepath:    "/home/test",
			expected:    "app",
			expectedErr: nil,
		},
		{
			name:        "will handle relative fullpath",
			fullpath:    "./app/",
			basepath:    "/home/test",
			expected:    "",
			expectedErr: files.RelativeOutOfBoundErr,
		},
		{
			name:        "will handle relative basepath",
			fullpath:    "/home/test/app/",
			basepath:    "./test",
			expected:    "",
			expectedErr: files.RelativeOutOfBoundErr,
		},
		{
			name:        "will handle relative paths",
			fullpath:    "./test/app",
			basepath:    "./test/app",
			expected:    ".",
			expectedErr: nil,
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			// given
			t.Parallel()

			// when
			relativePath, err := files.RelativePath(c.fullpath, c.basepath)

			// then
			assert.Equal(t, c.expectedErr, err)
			assert.Equal(t, c.expected, relativePath)
		})
	}
}

func TestInbound(t *testing.T) {
	type testcase struct {
		name      string
		candidate string
		basedir   string
		expected  bool
	}
	cases := []testcase{
		{
			name:      "will return true if candidate is inbound",
			candidate: "/home/test/app/readme.md",
			basedir:   "/home/test",
			expected:  true,
		},
		{
			name:      "will return false if candidate is not inbound",
			candidate: "/home/test/../readme.md",
			basedir:   "/home/test",
			expected:  false,
		},
		{
			name:      "will return true if candidate is relative inbound",
			candidate: "./readme.md",
			basedir:   "/home/test",
			expected:  true,
		},
		{
			name:      "will return false if candidate is relative outbound",
			candidate: "../readme.md",
			basedir:   "/home/test",
			expected:  false,
		},
		{
			name:      "will return false if basedir is relative",
			candidate: "/home/test/app/readme.md",
			basedir:   "./test",
			expected:  false,
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			// given
			t.Parallel()

			// when
			inbound := files.Inbound(c.candidate, c.basedir)

			// then
			assert.Equal(t, c.expected, inbound)
		})
	}
}
