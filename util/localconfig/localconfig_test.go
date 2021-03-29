package localconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetUsername(t *testing.T) {
	assert.Equal(t, "admin", GetUsername("admin:login"))
	assert.Equal(t, "admin", GetUsername("admin"))
	assert.Equal(t, "", GetUsername(""))
}
