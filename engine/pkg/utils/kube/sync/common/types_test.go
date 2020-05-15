package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewHookType(t *testing.T) {
	t.Run("Garbage", func(t *testing.T) {
		_, ok := NewHookType("Garbage")
		assert.False(t, ok)
	})
	t.Run("PreSync", func(t *testing.T) {
		hookType, ok := NewHookType("PreSync")
		assert.True(t, ok)
		assert.Equal(t, HookTypePreSync, hookType)
	})
	t.Run("Sync", func(t *testing.T) {
		hookType, ok := NewHookType("Sync")
		assert.True(t, ok)
		assert.Equal(t, HookTypeSync, hookType)
	})
	t.Run("PostSync", func(t *testing.T) {
		hookType, ok := NewHookType("PostSync")
		assert.True(t, ok)
		assert.Equal(t, HookTypePostSync, hookType)
	})
}

func TestNewHookDeletePolicy(t *testing.T) {
	t.Run("Garbage", func(t *testing.T) {
		_, ok := NewHookDeletePolicy("Garbage")
		assert.False(t, ok)
	})
	t.Run("HookSucceeded", func(t *testing.T) {
		p, ok := NewHookDeletePolicy("HookSucceeded")
		assert.True(t, ok)
		assert.Equal(t, HookDeletePolicyHookSucceeded, p)
	})
	t.Run("HookFailed", func(t *testing.T) {
		p, ok := NewHookDeletePolicy("HookFailed")
		assert.True(t, ok)
		assert.Equal(t, HookDeletePolicyHookFailed, p)
	})
	t.Run("BeforeHookCreation", func(t *testing.T) {
		p, ok := NewHookDeletePolicy("BeforeHookCreation")
		assert.True(t, ok)
		assert.Equal(t, HookDeletePolicyBeforeHookCreation, p)
	})
}
