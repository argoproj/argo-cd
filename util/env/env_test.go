package env

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnviron_Environ(t *testing.T) {
	assert.Equal(t, []string{"FOO=bar"}, Vars{Var("FOO=bar")}.Environ())
}

func TestEnviron_Envsubst(t *testing.T) {
	assert.Equal(t, "", Vars{Var("FOO=bar")}.Envsubst()(""))
	assert.Equal(t, "bar", Vars{Var("FOO=bar")}.Envsubst()("$FOO"))
	assert.Equal(t, "bar", Vars{Var("FOO=bar")}.Envsubst()("${FOO}"))
}
