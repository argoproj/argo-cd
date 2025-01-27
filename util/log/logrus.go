package log

import (
	"fmt"
	"os"
	"strings"

	adapter "github.com/bombsimon/logrusr/v2"
	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v2/common"
)

const (
	JsonFormat = "json"
	TextFormat = "text"
)

func NewLogrusLogger(fieldLogger logrus.FieldLogger) logr.Logger {
	return adapter.New(fieldLogger, adapter.WithFormatter(func(val interface{}) string {
		return fmt.Sprintf("%v", val)
	}))
}

// NewWithCurrentConfig create logrus logger by using current configuration
func NewWithCurrentConfig() *logrus.Logger {
	l := logrus.New()
	l.SetFormatter(CreateFormatter(os.Getenv(common.EnvLogFormat)))
	l.SetLevel(createLogLevel())
	return l
}

// CreateFormatter create logrus formatter by string
func CreateFormatter(logFormat string) logrus.Formatter {
	var formatType logrus.Formatter
	switch strings.ToLower(logFormat) {
	case JsonFormat:
		formatType = &logrus.JSONFormatter{}
	case TextFormat:
		formatType = &logrus.TextFormatter{
			ForceColors:   checkForceLogColors(),
			FullTimestamp: checkEnableFullTimestamp(),
		}
	default:
		formatType = &logrus.TextFormatter{
			FullTimestamp: checkEnableFullTimestamp(),
		}
	}

	return formatType
}

func createLogLevel() logrus.Level {
	level, err := logrus.ParseLevel(os.Getenv(common.EnvLogLevel))
	if err != nil {
		level = logrus.InfoLevel
	}
	return level
}

func checkForceLogColors() bool {
	return strings.ToLower(os.Getenv("FORCE_LOG_COLORS")) == "1"
}

func checkEnableFullTimestamp() bool {
	return strings.ToLower(os.Getenv(common.EnvLogFormatEnableFullTimestamp)) == "1"
}
