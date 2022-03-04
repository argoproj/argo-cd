package files_test

import (
	"testing"

	"github.com/argoproj/argo-cd/v2/util/files"
	"github.com/stretchr/testify/assert"
)

func TestRelativePath(t *testing.T) {
	type testcase struct {
		name     string
		fullpath string
		basepath string
		expected string
	}
	cases := []testcase{
		{
			name:     "will return relative path from file path",
			fullpath: "/home/test/app/readme.md",
			basepath: "/home/test",
			expected: "app/readme.md",
		},
		{
			name:     "will return relative path from dir path",
			fullpath: "/home/test/app/",
			basepath: "/home/test",
			expected: "app",
		},
		{
			name:     "will return . if fullpath and basepath are the same",
			fullpath: "/home/test/app/readme.md",
			basepath: "/home/test/app/readme.md",
			expected: ".",
		},
		{
			name:     "will return full path if basepath does not match",
			fullpath: "/home/test/app/readme.md",
			basepath: "/somewhere/else",
			expected: "/home/test/app/readme.md",
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			// given
			t.Parallel()

			// when
			relativePath := files.RelativePath(c.fullpath, c.basepath)

			// then
			assert.Equal(t, c.expected, relativePath)
		})
	}
}
