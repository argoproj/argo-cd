package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindPreviousTagRules(t *testing.T) {
	t.Parallel()
	tags := []string{"v1.2.3", "v1.2.2", "v1.2.1", "v1.2.0", "v1.1.9", "v1.1.8", "v1.1.7-rc1", "v1.1.6"}

	tests := []struct {
		name, proposedTag, expected string
		expectError                 bool
	}{
		// Rule 1: If we're releasing a .0 patch release, get the most recent tag on the previous minor release series.
		{"Rule 1: .0 patch release", "v1.2.0", "v1.1.9", false},
		// Rule 2: If we're releasing a non-0 patch release, get the most recent tag within the same minor release series.
		{"Rule 2: non-0 patch release", "v1.2.3", "v1.2.2", false},
		// Rule 3: If we're releasing a 1 release candidate, get the most recent tag on the previous minor release series.
		{"Rule 3: 1 release candidate", "v1.2.0-rc1", "v1.1.9", false},
		// Rule 4: If we're releasing a non-1 release candidate, get the most recent rc tag on the current minor release series.
		{"Rule 4: non-1 release candidate", "v1.2.0-rc2", "v1.2.0-rc1", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result, err := findPreviousTag(test.proposedTag, tags)
			if test.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equalf(t, test.expected, test.expected, "for proposed tag %s expected %s but got %s", test.proposedTag, test.expected, result)
			}
		})
	}
}
