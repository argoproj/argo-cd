package cli

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v2/common"
	testutil "github.com/argoproj/argo-cd/v2/test"
)

func Test_GetExecTimeoutEnvVarValue(t *testing.T) {
	newEnvVar := "SOME_NEW_ENV_VAR"

	// The log messages are escaped to match the logger's escape format.
	expectedDeprecationNotice := strconv.Quote(fmt.Sprintf("The %q environment variable is deprecated in Argo CD 2.5. Use %q instead.", common.EnvExecTimeout, newEnvVar))
	expectedOverrideNotice := strconv.Quote(fmt.Sprintf("Both %q and %q are set. Using %q.", newEnvVar, common.EnvExecTimeout, newEnvVar))

	t.Run("fall back to old env var", func(t *testing.T) {
		logs := testutil.CaptureLogEntries(func() {
			expected := "120s"
			t.Setenv(common.EnvExecTimeout, expected)
			envValue := GetExecTimeoutEnvVarValue(newEnvVar)
			expectedDuration, err := time.ParseDuration(expected)
			require.NoError(t, err)
			assert.Equal(t, expectedDuration, envValue)
		})
		assert.Contains(t, logs, expectedDeprecationNotice)
		assert.NotContains(t, logs, expectedOverrideNotice)
	})
	t.Run("use new env var if both are set", func(t *testing.T) {
		logs := testutil.CaptureLogEntries(func() {
			expected := "60s"
			t.Setenv(newEnvVar, expected)
			t.Setenv(common.EnvExecTimeout, "120s")
			envValue := GetExecTimeoutEnvVarValue(newEnvVar)
			expectedDuration, err := time.ParseDuration(expected)
			require.NoError(t, err)
			assert.Equal(t, expectedDuration, envValue)
		})
		assert.Contains(t, logs, expectedDeprecationNotice)
		assert.Contains(t, logs, expectedOverrideNotice)
	})
	t.Run("use new env var if only it is set", func(t *testing.T) {
		logs := testutil.CaptureLogEntries(func() {
			expected := "60s"
			t.Setenv(newEnvVar, "60s")
			envValue := GetExecTimeoutEnvVarValue(newEnvVar)
			expectedDuration, err := time.ParseDuration(expected)
			require.NoError(t, err)
			assert.Equal(t, expectedDuration, envValue)
		})
		assert.NotContains(t, logs, expectedDeprecationNotice)
		assert.NotContains(t, logs, expectedOverrideNotice)
	})
}
