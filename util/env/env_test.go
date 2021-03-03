package env

import (
	"io"
	"os"
	"testing"
	"time"

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

func TestParseDurationFromEnv(t *testing.T) {
	testKey := "key"
	defaultVal := 2 * time.Second
	min := 1 * time.Second
	max := 3 * time.Second

	testCases := []struct {
		name     string
		env      string
		expected time.Duration
	}{{
		name:     "EnvNotSet",
		expected: defaultVal,
	}, {
		name:     "ValidValueSet",
		env:      "2s",
		expected: time.Second * 2,
	}, {
		name:     "MoreThanMaxSet",
		env:      "5s",
		expected: defaultVal,
	}, {
		name:     "LessThanMinSet",
		env:      "-1s",
		expected: defaultVal,
	}, {
		name:     "InvalidSet",
		env:      "hello",
		expected: defaultVal,
	}}

	for i, tc := range testCases {
		t.Run(testCases[i].name, func(t *testing.T) {
			tc = testCases[i]
			setEnv(t, testKey, tc.env)

			val := ParseDurationFromEnv(testKey, defaultVal, min, max)
			assert.Equal(t, tc.expected, val)
		})
	}
}
