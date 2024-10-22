package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindPreviousTagRules(t *testing.T) {
	t.Parallel()
	// Sample pulled from git tag --sort=-v:refname output.
	tags := []string{
		"v2.13.0-rc3",
		"v2.13.0-rc2",
		"v2.13.0-rc1",
		"v2.12.5",
		"v2.12.4",
		"v2.12.3",
		"v2.12.2",
		"v2.12.1",
		"v2.12.0-rc5",
		"v2.12.0-rc4",
		"v2.12.0-rc3",
		"v2.12.0-rc2",
		"v2.12.0-rc1",
		"v2.12.0",
		"v2.11.11",
		"v2.11.10",
		"v2.11.9",
		"v2.11.8",
		"v2.11.7",
		"v2.11.6",
		"v2.11.5",
		"v2.11.4",
		"v2.11.3",
		"v2.11.2",
		"v2.11.1",
		"v2.11.0-rc3",
		"v2.11.0-rc2",
		"v2.11.0-rc1",
		"v2.11.0",
		"v2.10.17",
		"v2.10.16",
		"v2.10.15",
		"v2.10.14",
		"v2.10.13",
		"v2.10.12",
		"v2.10.11",
		"v2.10.10",
		"v2.10.9",
		"v2.10.8",
		"v2.10.7",
		"v2.10.6",
		"v2.10.5",
		"v2.10.4",
		"v2.10.3",
		"v2.10.2",
		"v2.10.1",
		"v2.10.0-rc5",
		"v2.10.0-rc4",
		"v2.10.0-rc3",
		"v2.10.0-rc2",
		"v2.10.0-rc1",
		"v2.10.0",
	}

	tests := []struct {
		name, proposedTag, expected string
		expectError                 bool
	}{
		// Rule 1: If we're releasing a .0 patch release, get the most recent tag on the previous minor release series.
		{"Rule 1: .0 patch release", "v2.13.0", "v2.12.5", false},
		// Rule 2: If we're releasing a non-0 patch release, get the most recent tag within the same minor release series.
		{"Rule 2: non-0 patch release", "v2.12.6", "v2.12.5", false},
		// Rule 3: If we're releasing a 1 release candidate, get the most recent tag on the previous minor release series.
		{"Rule 3: 1 release candidate", "v2.14.0-rc1", "v2.13.0-rc3", false},
		// Rule 4: If we're releasing a non-1 release candidate, get the most recent rc tag on the current minor release series.
		{"Rule 4: non-1 release candidate", "v2.13.0-rc4", "v2.13.0-rc3", false},
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
