package configbus

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSettingsManagerProviderRequiresMgr(t *testing.T) {
	assert.Panics(t, func() {
		NewSettingsManagerProvider(nil)
	})
}
