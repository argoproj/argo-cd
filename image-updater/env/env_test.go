package env

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GetBoolVal(t *testing.T) {
	t.Run("Get 'true' value from existing env var", func(t *testing.T) {
		_ = os.Setenv("TEST_BOOL_VAL", "true")
		defer os.Setenv("TEST_BOOL_VAL", "")
		assert.True(t, GetBoolVal("TEST_BOOL_VAL", false))
	})
	t.Run("Get 'false' value from existing env var", func(t *testing.T) {
		_ = os.Setenv("TEST_BOOL_VAL", "false")
		defer os.Setenv("TEST_BOOL_VAL", "")
		assert.False(t, GetBoolVal("TEST_BOOL_VAL", true))
	})
	t.Run("Get default value from non-existing env var", func(t *testing.T) {
		_ = os.Setenv("TEST_BOOL_VAL", "")
		assert.True(t, GetBoolVal("TEST_BOOL_VAL", true))
	})
}

func Test_GetStringVal(t *testing.T) {
	t.Run("Get string value from existing env var", func(t *testing.T) {
		_ = os.Setenv("TEST_STRING_VAL", "test")
		defer os.Setenv("TEST_STRING_VAL", "")
		assert.Equal(t, "test", GetStringVal("TEST_STRING_VAL", "invalid"))
	})
	t.Run("Get default value from non-existing env var", func(t *testing.T) {
		_ = os.Setenv("TEST_STRING_VAL", "")
		defer os.Setenv("TEST_STRING_VAL", "")
		assert.Equal(t, "invalid", GetStringVal("TEST_STRING_VAL", "invalid"))
	})
}
