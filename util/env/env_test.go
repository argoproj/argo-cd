package env

import (
	"fmt"
	"io"
	"math"
	"os"
	"testing"
	"time"

	util "github.com/argoproj/argo-cd/v2/util/io"

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

func TestParseFloatFromEnv(t *testing.T) {
	t.Run("Env not set", func(t *testing.T) {
		closer := setEnv(t, "test", "")
		defer util.Close(closer)
		f := ParseFloatFromEnv("test", 1, 0, math.MaxFloat32)
		assert.Equal(t, float32(1.0), f)
	})
	t.Run("Parse valid float", func(t *testing.T) {
		closer := setEnv(t, "test", "2.5")
		defer util.Close(closer)
		f := ParseFloatFromEnv("test", 1, 0, math.MaxFloat32)
		assert.Equal(t, float32(2.5), f)
	})
	t.Run("Parse valid integer as float", func(t *testing.T) {
		closer := setEnv(t, "test", "2")
		defer util.Close(closer)
		f := ParseFloatFromEnv("test", 1, 0, math.MaxFloat32)
		assert.Equal(t, float32(2.0), f)
	})
	t.Run("Parse invalid value", func(t *testing.T) {
		closer := setEnv(t, "test", "foo")
		defer util.Close(closer)
		f := ParseFloatFromEnv("test", 1, 0, math.MaxFloat32)
		assert.Equal(t, float32(1.0), f)
	})
	t.Run("Float lesser than allowed", func(t *testing.T) {
		closer := setEnv(t, "test", "-2.0")
		defer util.Close(closer)
		f := ParseFloatFromEnv("test", 1, 0, math.MaxFloat32)
		assert.Equal(t, float32(1.0), f)
	})
	t.Run("Float greater than allowed", func(t *testing.T) {
		closer := setEnv(t, "test", "5.0")
		defer util.Close(closer)
		f := ParseFloatFromEnv("test", 1, 0, 4)
		assert.Equal(t, float32(1.0), f)
	})
	t.Run("Check float overflow returning default value", func(t *testing.T) {
		closer := setEnv(t, "test", fmt.Sprintf("%f", math.MaxFloat32*2))
		defer util.Close(closer)
		f := ParseFloatFromEnv("test", 1, 0, math.MaxFloat32)
		assert.Equal(t, float32(1.0), f)
	})
}

func TestParseInt64FromEnv(t *testing.T) {
	t.Run("Env not set", func(t *testing.T) {
		closer := setEnv(t, "test", "")
		defer util.Close(closer)
		i := ParseInt64FromEnv("test", 1, 0, math.MaxInt64)
		assert.Equal(t, int64(1), i)
	})
	t.Run("Parse valid int64", func(t *testing.T) {
		closer := setEnv(t, "test", "3")
		defer util.Close(closer)
		i := ParseInt64FromEnv("test", 1, 0, math.MaxInt64)
		assert.Equal(t, int64(3), i)
	})
	t.Run("Parse invalid value", func(t *testing.T) {
		closer := setEnv(t, "test", "foo")
		defer util.Close(closer)
		i := ParseInt64FromEnv("test", 1, 0, math.MaxInt64)
		assert.Equal(t, int64(1), i)
	})
	t.Run("Int64 lesser than allowed", func(t *testing.T) {
		closer := setEnv(t, "test", "-2")
		defer util.Close(closer)
		i := ParseInt64FromEnv("test", 1, 0, math.MaxInt64)
		assert.Equal(t, int64(1), i)
	})
	t.Run("Int64 greater than allowed", func(t *testing.T) {
		closer := setEnv(t, "test", "5")
		defer util.Close(closer)
		i := ParseInt64FromEnv("test", 1, 0, 4)
		assert.Equal(t, int64(1), i)
	})
}

func TestParseDurationFromEnv(t *testing.T) {
	testKey := "key"
	defaultVal := 2 * time.Second

	testCases := []struct {
		name     string
		env      string
		min      time.Duration
		max      time.Duration
		expected time.Duration
	}{{
		name:     "EnvNotSet",
		expected: defaultVal,
	}, {
		name:     "InvalidSet",
		env:      "hello",
		expected: defaultVal,
	}, {
		name:     "ValidTimeValueSet",
		env:      "10s",
		max:      math.MaxInt64,
		expected: time.Second * 10,
	}, {
		name:     "ValidSignedValueSet",
		env:      "+10s",
		max:      math.MaxInt64,
		expected: time.Second * 10,
	}, {
		name:     "ValidTimeFractionValueSet",
		env:      "1.5m",
		max:      math.MaxInt64,
		expected: time.Second * 90,
	}, {
		name:     "ValidDayValueSet",
		env:      "1d",
		max:      math.MaxInt64,
		expected: time.Hour * 24,
	}, {
		name:     "ValidDayFractionValueSet",
		env:      "1.5d",
		max:      math.MaxInt64,
		expected: time.Hour * 36,
	}, {
		name:     "ValidComplexDurationSet",
		env:      "1d2h",
		max:      math.MaxInt64,
		expected: time.Hour * 26,
	}, {
		name:     "MoreThanMaxSet",
		env:      "5s",
		min:      2 * time.Second,
		max:      3 * time.Second,
		expected: defaultVal,
	}, {
		name:     "LessThanMinSet",
		env:      "-1s",
		min:      2 * time.Second,
		max:      3 * time.Second,
		expected: defaultVal,
	}}

	for i, tc := range testCases {
		t.Run(testCases[i].name, func(t *testing.T) {
			tc = testCases[i]
			setEnv(t, testKey, tc.env)

			val := ParseDurationFromEnv(testKey, defaultVal, tc.min, tc.max)
			assert.Equal(t, tc.expected, val)
		})
	}
}

func Test_ParseBoolFromEnv(t *testing.T) {
	t.Run("Get 'true' value from existing env var", func(t *testing.T) {
		_ = os.Setenv("TEST_BOOL_VAL", "true")
		defer os.Setenv("TEST_BOOL_VAL", "")
		assert.True(t, ParseBoolFromEnv("TEST_BOOL_VAL", false))
	})
	t.Run("Get 'false' value from existing env var", func(t *testing.T) {
		_ = os.Setenv("TEST_BOOL_VAL", "false")
		defer os.Setenv("TEST_BOOL_VAL", "")
		assert.False(t, ParseBoolFromEnv("TEST_BOOL_VAL", true))
	})
	t.Run("Get default value from non-existing env var", func(t *testing.T) {
		_ = os.Setenv("TEST_BOOL_VAL", "")
		assert.True(t, ParseBoolFromEnv("TEST_BOOL_VAL", true))
	})
}
