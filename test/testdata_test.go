package test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnnotate(t *testing.T) {
	obj := Annotate(NewPod(), "foo", "bar")
	assert.Equal(t, "bar", obj.GetAnnotations()["foo"])

	obj = Annotate(Annotate(NewPod(), "foo", "bar"), "baz", "qux")
	assert.Equal(t, "bar", obj.GetAnnotations()["foo"])
	assert.Equal(t, "qux", obj.GetAnnotations()["baz"])
}

func TestNewCRD(t *testing.T) {
	assert.Equal(t, "CustomResourceDefinition", NewCRD().GetKind())
}
