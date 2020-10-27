package env

import (
	"io"
	"os"
	"testing"

	util "github.com/argoproj/argo-cd/util/io"

	"github.com/stretchr/testify/assert"
)

// nolint:unparam
func setEnv(t *testing.T, env string, val string) io.Closer {
	assert.NoError(t, os.Setenv(env, val))
	return util.NewCloser(func() error {
		assert.NoError(t, os.Setenv(env, ""))
		return nil
	})
}

func TestParseNumFromEnv_NoEnvVariable(t *testing.T) {
	num := ParseNumFromEnv("test", 10, 0, 100)

	assert.Equal(t, 10, num)
}

func TestParseNumFromEnv_CorrectValueSet(t *testing.T) {
	closer := setEnv(t, "test", "15")
	defer util.Close(closer)

	num := ParseNumFromEnv("test", 10, 0, 100)

	assert.Equal(t, 15, num)
}

func TestParseNumFromEnv_NonIntValueSet(t *testing.T) {
	closer := setEnv(t, "test", "wrong")
	defer util.Close(closer)

	num := ParseNumFromEnv("test", 10, 0, 100)

	assert.Equal(t, 10, num)
}

func TestParseNumFromEnv_NegativeValueSet(t *testing.T) {
	closer := setEnv(t, "test", "-1")
	defer util.Close(closer)

	num := ParseNumFromEnv("test", 10, 0, 100)

	assert.Equal(t, 10, num)
}

func TestParseNumFromEnv_OutOfRangeValueSet(t *testing.T) {
	closer := setEnv(t, "test", "1000")
	defer util.Close(closer)

	num := ParseNumFromEnv("test", 10, 0, 100)

	assert.Equal(t, 10, num)
}
