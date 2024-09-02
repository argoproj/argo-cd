package log

import (
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/v2/common"
)

func TestCreateFormatter(t *testing.T) {
	t.Run("log format is json", func(t *testing.T) {
		result := CreateFormatter("json")
		assert.Equal(t, &logrus.JSONFormatter{}, result)
	})
	t.Run("log format is text", func(t *testing.T) {
		t.Run("FORCE_LOG_COLORS == 1", func(t *testing.T) {
			t.Setenv("FORCE_LOG_COLORS", "1")
			result := CreateFormatter("text")
			assert.Equal(t, &logrus.TextFormatter{ForceColors: true}, result)
		})
		t.Run("FORCE_LOG_COLORS != 1", func(t *testing.T) {
			t.Setenv("FORCE_LOG_COLORS", "0")
			result := CreateFormatter("text")
			assert.Equal(t, &logrus.TextFormatter{}, result)
		})
		t.Run(fmt.Sprintf("%s == 1", common.EnvLogFormatEnableFullTimestamp), func(t *testing.T) {
			t.Setenv(common.EnvLogFormatEnableFullTimestamp, "1")
			result := CreateFormatter("text")
			assert.Equal(t, &logrus.TextFormatter{FullTimestamp: true}, result)
		})
		t.Run(fmt.Sprintf("%s != 1", common.EnvLogFormatEnableFullTimestamp), func(t *testing.T) {
			t.Setenv(common.EnvLogFormatEnableFullTimestamp, "0")
			result := CreateFormatter("text")
			assert.Equal(t, &logrus.TextFormatter{}, result)
		})
	})
	t.Run("log format is not json or text", func(t *testing.T) {
		result := CreateFormatter("xml")
		assert.Equal(t, &logrus.TextFormatter{}, result)
	})
}
