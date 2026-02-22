package settings

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrivyExclusions(t *testing.T) {
	settings := &ResourcesFilter{}
	assert.True(t, settings.IsExcludedResource("aquasecurity.github.io", "VulnerabilityReport", ""))
	assert.True(t, settings.IsExcludedResource("aquasecurity.github.io", "ClusterVulnerabilityReport", ""))
	// "UnknownReport" is not in the list, so it should NOT be excluded (False)
	assert.False(t, settings.IsExcludedResource("aquasecurity.github.io", "UnknownReport", ""))
}
