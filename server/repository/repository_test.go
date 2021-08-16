package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_createRBACObject(t *testing.T) {
	object := createRBACObject("test-prj", "test-repo")
	assert.Equal(t, "test-prj/test-repo", object)
	objectWithoutPrj := createRBACObject("", "test-repo")
	assert.Equal(t, "test-repo", objectWithoutPrj)
}
