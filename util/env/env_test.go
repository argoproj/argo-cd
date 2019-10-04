package env

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewEnv(t *testing.T) {
	assert.Equal(t, Var("="), NewEnv("", ""))
	assert.Equal(t, Var("FOO=bar"), NewEnv("FOO", "bar"))
}

func TestEnviron_Environ(t *testing.T) {
	assert.Equal(t, []string{"FOO=bar"}, Vars{NewEnv("FOO", "bar")}.Environ())
}

func TestEnviron_Envsubst(t *testing.T) {
	assert.Equal(t, "", Vars{NewEnv("FOO", "bar")}.Envsubst()(""))
	assert.Equal(t, "bar", Vars{NewEnv("FOO", "bar")}.Envsubst()("$FOO"))
	assert.Equal(t, "bar", Vars{NewEnv("FOO", "bar")}.Envsubst()("${FOO}"))
}