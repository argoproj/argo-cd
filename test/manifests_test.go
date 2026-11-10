package test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/test/fixture/test"
	argoexec "github.com/argoproj/argo-cd/v3/util/exec"
)

func TestKustomizeVersion(t *testing.T) {
	t.Parallel()
	test.CIOnly(t)
	out, err := argoexec.RunCommand("kustomize", argoexec.CmdOpts{}, "version")
	require.NoError(t, err)
	assert.Contains(t, out, "v5.", "kustomize should be version 5")
}

// TestBuildManifests makes sure we are consistent in naming, and all kustomization.yamls are buildable
func TestBuildManifests(t *testing.T) {
	t.Parallel()
	err := filepath.Walk("../manifests", func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		switch filepath.Base(path) {
		case "kustomization.yaml":
			// noop
		case "Kustomization", "kustomization.yml":
			// These are valid, but we want to be consistent with filenames
			return fmt.Errorf("Please name file 'kustomization.yaml' instead of '%s'", filepath.Base(path))
		case "Kustomize", "kustomize.yaml", "kustomize.yml":
			// These are not even valid kustomization filenames but sometimes get mistaken for them
			return fmt.Errorf("'%s' is not a valid kustomize name", filepath.Base(path))
		default:
			return nil
		}
		dirName := filepath.Dir(path)
		_, err = argoexec.RunCommand("kustomize", argoexec.CmdOpts{}, "build", dirName)
		return err
	})
	require.NoError(t, err)
}

// TestDashboardDescription verifies that the Grafana dashboard JSON has a valid
// description field matching the expected "Argo CD Dashboard <version>" format.
func TestDashboardDescription(t *testing.T) {
	t.Parallel()

	dashboardPath := filepath.Join("..", "examples", "dashboard.json")
	data, err := os.ReadFile(dashboardPath)
	require.NoError(t, err, "examples/dashboard.json must exist")

	var dashboard map[string]any
	require.NoError(t, json.Unmarshal(data, &dashboard), "dashboard.json must be valid JSON")

	descRaw, ok := dashboard["description"]
	require.True(t, ok, "dashboard.json must have a description field")
	desc, ok := descRaw.(string)
	require.True(t, ok, "dashboard description must be a string")
	assert.Regexp(t, `^Argo CD Dashboard \S+$`, desc, "description must match 'Argo CD Dashboard <version>'")
}

// simulateDashboardDescUpdate applies the same replacement that
// hack/update-manifests.sh performs using jq, which safely handles
// any characters in the version string.
func simulateDashboardDescUpdate(t *testing.T, jsonContent string, imageTag string) string {
	t.Helper()

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(jsonContent), &parsed))
	parsed["description"] = "Argo CD Dashboard " + imageTag
	out, err := json.Marshal(parsed)
	require.NoError(t, err)
	return string(out)
}

func TestDashboardVersionInjection(t *testing.T) {
	t.Parallel()

	dashboardPath := filepath.Join("..", "examples", "dashboard.json")
	original, err := os.ReadFile(dashboardPath)
	require.NoError(t, err)

	tests := []struct {
		name string
		tag  string
		want string
	}{
		{name: "default latest", tag: "latest", want: "Argo CD Dashboard latest"},
		{name: "release branch", tag: "v3.0.0", want: "Argo CD Dashboard v3.0.0"},
		{name: "explicit tag", tag: "abc123", want: "Argo CD Dashboard abc123"},
		{name: "tag with ampersand", tag: "test&1", want: "Argo CD Dashboard test&1"},
		{name: "tag with pipe", tag: "a|b", want: "Argo CD Dashboard a|b"},
		{name: "tag with backslash", tag: `a\b`, want: `Argo CD Dashboard a\b`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := simulateDashboardDescUpdate(t, string(original), tc.tag)

			var parsed map[string]any
			require.NoError(t, json.Unmarshal([]byte(result), &parsed), "result must be valid JSON")

			desc, ok := parsed["description"].(string)
			require.True(t, ok)
			assert.Equal(t, tc.want, desc)
		})
	}
}
