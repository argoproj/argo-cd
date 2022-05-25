package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewClient_invalidSSHURL(t *testing.T) {
	client, err := NewClient("ssh://bitbucket.org:org/repo", NopCreds{}, false, false, "")
	assert.Nil(t, client)
	assert.ErrorIs(t, err, ErrInvalidRepoURL)
}
