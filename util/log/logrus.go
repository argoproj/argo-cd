package log

import (
	adapter "github.com/bombsimon/logrusr"
	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"
)

func NewLogrusLogger(fieldLogger logrus.FieldLogger) logr.Logger {
	return adapter.NewLogger(fieldLogger)
}
