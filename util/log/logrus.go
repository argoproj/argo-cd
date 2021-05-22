package log

import (
	"fmt"
	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/util/errors"
	adapter "github.com/bombsimon/logrusr"
	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"
	"os"
	"strings"
)

func NewLogrusLogger(fieldLogger logrus.FieldLogger) logr.Logger {
	return adapter.NewLoggerWithFormatter(fieldLogger, func(val interface{}) string {
		return fmt.Sprintf("%v", val)
	})
}

// NewWithCurrentConfig create logrus logger by using current configuration
func NewWithCurrentConfig() *logrus.Logger {
	l := logrus.New()
	l.SetFormatter(CreateFormatter(os.Getenv(common.EnvLogFormat)))

	level, err := logrus.ParseLevel(os.Getenv(common.EnvLogLevel))
	errors.CheckError(err)
	l.SetLevel(level)

	return l
}

// CreateFormatter create logrus formatter by string
func CreateFormatter(logFormat string) logrus.Formatter {
	var formatType logrus.Formatter
	switch strings.ToLower(logFormat) {
	case "json":
		formatType = &logrus.JSONFormatter{}
	case "text":
		if os.Getenv("FORCE_LOG_COLORS") == "1" {
			formatType = &logrus.TextFormatter{ForceColors: true}
		} else {
			formatType = &logrus.TextFormatter{}
		}
	default:
		logrus.Fatalf("Unknown log format '%s'", logFormat)
		formatType = &logrus.TextFormatter{}
	}

	return formatType
}
