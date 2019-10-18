package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_serverToSecretName(t *testing.T) {
	t.Run("URL", func(t *testing.T) {
		name, err := serverToSecretName("http://foo")
		assert.NoError(t, err)
		assert.Equal(t, "cluster-foo-752281925", name)
	})
	t.Run("NotURL", func(t *testing.T) {
		name, err := serverToSecretName("foo")
		assert.NoError(t, err)
		assert.Equal(t, "cluster-foo-636106838", name)
	})
}
