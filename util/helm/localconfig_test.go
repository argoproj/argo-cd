package helm

import (
	"io"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHelmConfiguration(t *testing.T) {
	tmp, err := ioutil.TempDir("", "argocd")
	assert.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmp) }()
	err = os.Setenv("HOME", tmp)
	assert.NoError(t, err)

	config, err := DefaultLocalRepositoryConfigPath()
	assert.NoError(t, err)
	repos, err := ReadLocalRepositoryConfig(config)
	if assert.NoError(t, err) {
		assert.Empty(t, repos)
	}

	err = os.MkdirAll(path.Join(tmp, ".helm", "repository"), 0755)
	assert.NoError(t, err)
	src, err := os.Open(path.Join("testdata", "repositories.yaml"))
	if assert.NoError(t, err) {
		defer src.Close()
	}
	dst, err := os.Create(path.Join(tmp, ".helm", "repository", "repositories.yaml"))
	if assert.NoError(t, err) {
		defer dst.Close()
	}
	_, err = io.Copy(dst, src)
	assert.NoError(t, err)

	repos, err = ReadLocalRepositoryConfig(config)
	assert.NoError(t, err)
	assert.Len(t, repos, 1)
}
