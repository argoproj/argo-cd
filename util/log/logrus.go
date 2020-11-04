package log

import (
	"fmt"

	adapter "github.com/bombsimon/logrusr"
	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"
)

func NewLogrusLogger(fieldLogger logrus.FieldLogger) logr.Logger {
	return adapter.NewLoggerWithFormatter(fieldLogger, func(val interface{}) string {
		return fmt.Sprintf("%v", val)
	})
}
