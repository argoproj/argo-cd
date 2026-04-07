package common

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetVersion(t *testing.T) {
	tests := []struct {
		name           string
		inputGitCommit string
		inputGitTag    string
		inputTreeState string
		inputVersion   string
		expected       string
	}{
		{
			name:           "Official release with tag and clean state",
			inputGitCommit: "abcdef123456",
			inputGitTag:    "v1.2.3",
			inputTreeState: "clean",
			inputVersion:   "1.2.3",
			expected:       "v1.2.3",
		},
		{
			name:           "Dirty state with commit",
			inputGitCommit: "deadbeefcafebabe",
			inputGitTag:    "",
			inputTreeState: "dirty",
			inputVersion:   "2.0.1",
			expected:       "v2.0.1+deadbee.dirty",
		},
		{
			name:           "Clean state with commit, no tag",
			inputGitCommit: "cafebabedeadbeef",
			inputGitTag:    "",
			inputTreeState: "clean",
			inputVersion:   "2.1.0",
			expected:       "v2.1.0+cafebab",
		},
		{
			name:           "Missing commit and tag",
			inputGitCommit: "",
			inputGitTag:    "",
			inputTreeState: "clean",
			inputVersion:   "3.1.0",
			expected:       "v3.1.0+unknown",
		},
		{
			name:           "Short commit",
			inputGitCommit: "abc",
			inputGitTag:    "",
			inputTreeState: "clean",
			inputVersion:   "4.0.0",
			expected:       "v4.0.0+unknown",
		},
	}
	for _, tt := range tests {
		gitCommit = tt.inputGitCommit
		gitTag = tt.inputGitTag
		gitTreeState = tt.inputTreeState
		version = tt.inputVersion

		buildDate = "2025-06-26"
		kubectlVersion = "v1.30.0"
		extraBuildInfo = "test-build"

		got := GetVersion()
		assert.Equal(t, tt.expected, got.Version)
		assert.Equal(t, buildDate, got.BuildDate)
		assert.Equal(t, tt.inputGitCommit, got.GitCommit)
		assert.Equal(t, tt.inputGitTag, got.GitTag)
		assert.Equal(t, tt.inputTreeState, got.GitTreeState)
		assert.Equal(t, runtime.Version(), got.GoVersion)
		assert.Equal(t, runtime.Compiler, got.Compiler)
		assert.Equal(t, runtime.GOOS+"/"+runtime.GOARCH, got.Platform)
		assert.Equal(t, kubectlVersion, got.KubectlVersion)
		assert.Equal(t, extraBuildInfo, got.ExtraBuildInfo)
	}
}
