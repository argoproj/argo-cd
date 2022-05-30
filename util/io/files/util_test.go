package files_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/v2/util/io/files"
)

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
