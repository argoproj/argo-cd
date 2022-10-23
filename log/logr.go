package log

import (
	"github.com/go-logr/logr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	minZapLevel = int(zapcore.DebugLevel)
	maxZapLevel = int(zapcore.FatalLevel)
)

// NewLogSink creates and returns a LogSink instance with the current default configuration.
func NewLogSink() logr.LogSink {
	return &bridgeLogSink{l: newLoggerWrapper(newZapLogger())}
}

// NewLogrLogger creates a new logr.Logger instance from the current default configuration.
func NewLogrLogger() logr.Logger {
	return logr.New(NewLogSink())
}

// DefaultLogSink returns the global default LogSink instance.
func DefaultLogSink() logr.LogSink {
	return &bridgeLogSink{l: std}
}

// DefaultLogrLogger returns the global default logr.Logger instance.
func DefaultLogrLogger() logr.Logger {
	return logr.New(DefaultLogSink())
}

// This is a bridge from the zap logger wrapper to the logr.LogSink interface.
type bridgeLogSink struct {
	l *loggerWrapper
}

// Init receives optional information about the logr library for LogSink
// implementations that need it.
// This method is an implementation of the logr.LogSink interface.
func (b *bridgeLogSink) Init(info logr.RuntimeInfo) {
	b.l.log = b.l.log.WithOptions(zap.AddCallerSkip(info.CallDepth))
}

// Enabled tests whether this LogSink is enabled at the specified V-level.
// For example, commandline flags might be used to set the logging
// verbosity and disable some info logs.
// This method is an implementation of the logr.LogSink interface.
func (b *bridgeLogSink) Enabled(level int) bool {
	if level < minZapLevel {
		return false
	}
	return level > maxZapLevel || b.l.level.Enabled(zapcore.Level(level))
}

// Info logs a non-error message with the given key/value pairs as context.
// The level argument is provided for optional logging.  This method will
// only be called when Enabled(level) is true. See Logger.Info for more
// details.
// This method is an implementation of the logr.LogSink interface.
func (b *bridgeLogSink) Info(level int, msg string, kvs ...interface{}) {
	if len(kvs) > 0 {
		b.l.log.With(kvs...).With("V", level).Info(msg)
	} else {
		b.l.log.With("V", level).Info(msg)
	}
}

// Error logs an error, with the given message and key/value pairs as
// context.  See Logger.Error for more details.
// This method is an implementation of the logr.LogSink interface.
func (b *bridgeLogSink) Error(err error, msg string, kvs ...interface{}) {
	if len(kvs) > 0 {
		b.l.log.With(kvs...).With("error", err).Error(msg)
	} else {
		b.l.log.With("error", err).Error(msg)
	}
}

// WithValues returns a new LogSink with additional key/value pairs.  See
// Logger.WithValues for more details.
// This method is an implementation of the logr.LogSink interface.
func (b *bridgeLogSink) WithValues(kvs ...interface{}) logr.LogSink {
	if len(kvs) == 0 {
		return b
	}
	return &bridgeLogSink{l: newLoggerWrapper(&zapLogger{log: b.l.log.With(kvs...), level: b.l.level})}
}

// WithName returns a new LogSink with the specified name appended.  See
// Logger.WithName for more details.
// This method is an implementation of the logr.LogSink interface.
func (b *bridgeLogSink) WithName(name string) logr.LogSink {
	return &bridgeLogSink{l: newLoggerWrapper(&zapLogger{log: b.l.log.Named(name), level: b.l.level})}
}
