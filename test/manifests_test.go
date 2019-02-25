package test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	argoexec "github.com/argoproj/pkg/exec"
	"github.com/stretchr/testify/assert"
)

// TestBuildManifests makes sure we are consistent in naming, and all kustomization.yamls are buildable
func TestBuildManifests(t *testing.T) {
	out, err := argoexec.RunCommand("kustomize", "version")
	assert.NoError(t, err)
	if !assert.Contains(t, out, "KustomizeVersion:2", "kustomize should be version 2") {
		t.FailNow()
	}

	err = filepath.Walk("../manifests", func(path string, f os.FileInfo, err error) error {
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
		_, err = argoexec.RunCommand("kustomize", "build", dirName)
		return err
	})
	assert.NoError(t, err)
}
