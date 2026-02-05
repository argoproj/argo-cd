package test

import (
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
	test.CIOnly(t)
	out, err := argoexec.RunCommand("kustomize", argoexec.CmdOpts{}, "version")
	require.NoError(t, err)
	assert.Contains(t, out, "v5.", "kustomize should be version 5")
}

// TestBuildManifests makes sure we are consistent in naming, and all kustomization.yamls are buildable
func TestBuildManifests(t *testing.T) {
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
