package project

import (
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func TestGetRoleByName(t *testing.T) {
	t.Run("NotExists", func(t *testing.T) {
		role, i, err := GetRoleByName(&AppProject{}, "test-role")
		assert.Error(t, err)
		assert.Equal(t, -1, i)
		assert.Nil(t, role)
	})
	t.Run("NotExists", func(t *testing.T) {
		role, i, err := GetRoleByName(&AppProject{Spec: AppProjectSpec{Roles: []ProjectRole{{Name: "test-role"}}}}, "test-role")
		assert.NoError(t, err)
		assert.Equal(t, 0, i)
		assert.Equal(t, &ProjectRole{Name: "test-role"}, role)
	})
}

func TestAddGroupToRole(t *testing.T) {
	t.Run("NoRole", func(t *testing.T) {
		got, err := AddGroupToRole(&AppProject{}, "test-role", "test-group")
		assert.Error(t, err)
		assert.False(t, got)
	})
	t.Run("NoGroup", func(t *testing.T) {
		p := &AppProject{Spec: AppProjectSpec{Roles: []ProjectRole{{Name: "test-role", Groups: []string{}}}}}
		got, err := AddGroupToRole(p, "test-role", "test-group")
		assert.NoError(t, err)
		assert.True(t, got)
		assert.Len(t, p.Spec.Roles[0].Groups, 1)
	})
	t.Run("Exists", func(t *testing.T) {
		got, err := AddGroupToRole(&AppProject{Spec: AppProjectSpec{Roles: []ProjectRole{{Name: "test-role", Groups: []string{"test-group"}}}}}, "test-role", "test-group")
		assert.NoError(t, err)
		assert.False(t, got)
	})
}

func TestRemoveGroupFromRole(t *testing.T) {
	t.Run("NoRole", func(t *testing.T) {
		got, err := RemoveGroupFromRole(&AppProject{}, "test-role", "test-group")
		assert.Error(t, err)
		assert.False(t, got)
	})
	t.Run("NoGroup", func(t *testing.T) {
		p := &AppProject{Spec: AppProjectSpec{Roles: []ProjectRole{{Name: "test-role", Groups: []string{}}}}}
		got, err := RemoveGroupFromRole(p, "test-role", "test-group")
		assert.NoError(t, err)
		assert.False(t, got)
	})
	t.Run("Exists", func(t *testing.T) {
		p := &AppProject{Spec: AppProjectSpec{Roles: []ProjectRole{{Name: "test-role", Groups: []string{"test-group"}}}}}
		got, err := RemoveGroupFromRole(p, "test-role", "test-group")
		assert.NoError(t, err)
		assert.True(t, got)
		assert.Len(t, p.Spec.Roles[0].Groups, 0)
	})
}
