package log

// Wrapper package around logrus whose main purpose is to support having
// different output streams for error and non-error messages.
//
// Does not wrap every method of logrus package. If you need direct access,
// use log.Log() to get the actual logrus logger object.
//
// It might seem redundant, but we really want the different output streams.

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

// Internal Logger object
var logger *logrus.Logger

// LogContext contains a structured context for logging
type LogContext struct {
	fields    logrus.Fields
	normalOut io.Writer
	errorOut  io.Writer
	mutex     sync.RWMutex
}

// NewContext returns a LogContext with default settings
func NewContext() *LogContext {
	var logctx LogContext
	logctx.fields = make(logrus.Fields)
	logctx.normalOut = os.Stdout
	logctx.errorOut = os.Stderr
	return &logctx
}

// SetLogLevel sets the log level to use for the logger
func SetLogLevel(logLevel string) error {
	switch strings.ToLower(logLevel) {
	case "trace":
		logger.SetLevel(logrus.TraceLevel)
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		return fmt.Errorf("invalid loglevel: %s", logLevel)
	}
	return nil
}

// WithContext is an alias for NewContext
func WithContext() *LogContext {
	return NewContext()
}

// AddField adds a structured field to logctx
func (logctx *LogContext) AddField(key string, value interface{}) *LogContext {
	logctx.mutex.Lock()
	logctx.fields[key] = value
	logctx.mutex.Unlock()
	return logctx
}

// Logger retrieves the native logger interface. Use with care.
func Log() *logrus.Logger {
	return logger
}

// Tracef logs a debug message for logctx to stdout
func (logctx *LogContext) Tracef(format string, args ...interface{}) {
	logger.SetOutput(logctx.normalOut)
	if logctx.fields != nil && len(logctx.fields) > 0 {
		logger.WithFields(logctx.fields).Tracef(format, args...)
	} else {
		logger.Tracef(format, args...)
	}
}

// Debugf logs a debug message for logctx to stdout
func (logctx *LogContext) Debugf(format string, args ...interface{}) {
	logger.SetOutput(logctx.normalOut)
	if logctx.fields != nil && len(logctx.fields) > 0 {
		logger.WithFields(logctx.fields).Debugf(format, args...)
	} else {
		logger.Debugf(format, args...)
	}
}

// Infof logs an informational message for logctx to stdout
func (logctx *LogContext) Infof(format string, args ...interface{}) {
	logger.SetOutput(logctx.normalOut)
	if logctx.fields != nil && len(logctx.fields) > 0 {
		logger.WithFields(logctx.fields).Infof(format, args...)
	} else {
		logger.Infof(format, args...)
	}
}

// Warnf logs a warning message for logctx to stdout
func (logctx *LogContext) Warnf(format string, args ...interface{}) {
	logger.SetOutput(logctx.normalOut)
	if logctx.fields != nil && len(logctx.fields) > 0 {
		logger.WithFields(logctx.fields).Warnf(format, args...)
	} else {
		logger.Warnf(format, args...)
	}
}

// Errorf logs a non-fatal error message for logctx to stdout
func (logctx *LogContext) Errorf(format string, args ...interface{}) {
	logger.SetOutput(logctx.errorOut)
	if logctx.fields != nil && len(logctx.fields) > 0 {
		logger.WithFields(logctx.fields).Errorf(format, args...)
	} else {
		logger.Errorf(format, args...)
	}
}

// Fatalf logs a fatal error message for logctx to stdout
func (logctx *LogContext) Fatalf(format string, args ...interface{}) {
	logger.SetOutput(logctx.errorOut)
	if logctx.fields != nil && len(logctx.fields) > 0 {
		logger.WithFields(logctx.fields).Fatalf(format, args...)
	} else {
		logger.Fatalf(format, args...)
	}
}

// Debugf logs a warning message without context to stdout
func Tracef(format string, args ...interface{}) {
	logCtx := NewContext()
	logCtx.Tracef(format, args...)
}

// Debugf logs a warning message without context to stdout
func Debugf(format string, args ...interface{}) {
	logCtx := NewContext()
	logCtx.Debugf(format, args...)
}

// Infof logs a warning message without context to stdout
func Infof(format string, args ...interface{}) {
	logCtx := NewContext()
	logCtx.Infof(format, args...)
}

// Warnf logs a warning message without context to stdout
func Warnf(format string, args ...interface{}) {
	logCtx := NewContext()
	logCtx.Warnf(format, args...)
}

// Errorf logs an error message without context to stderr
func Errorf(format string, args ...interface{}) {
	logCtx := NewContext()
	logCtx.Errorf(format, args...)
}

// Fatalf logs a non-recoverable error message without context to stderr
func Fatalf(format string, args ...interface{}) {
	logCtx := NewContext()
	logCtx.Fatalf(format, args...)
}

func disableLogColors() bool {
	return strings.ToLower(os.Getenv("ENABLE_LOG_COLORS")) == "false"
}

// Initializes the logging subsystem with default values
func init() {
	logger = logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{DisableColors: disableLogColors()})
	logger.SetLevel(logrus.DebugLevel)
}
