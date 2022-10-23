package log

import (
	"io"
	"os"

	"go.uber.org/zap/zapcore"
	"golang.org/x/term"
)

// This is the global default logger.
var std = newLoggerWrapper(newZapLogger())

// This is the global default post-fatal logging hook.
var defaultFatalLogHook = new(fatalLogHook)

// RegisterExitHandler registers the given fatal logging hook.
func RegisterExitHandler(f func()) {
	defaultFatalLogHook.Register(f)
}

// InitDefaultLogger (re)initializes the global default logger.
func InitDefaultLogger(level, format, tap string) {
	std.zapLogger = newZapLoggerFrom(level, format, tap)
}

// Default returns the global default logger instance.
func Default() Logger {
	return std
}

// Debug records a debug level log.
func Debug(args ...interface{}) {
	std.Debug(args...)
}

// Debugf formats and records a debug level log.
func Debugf(format string, args ...interface{}) {
	std.Debugf(format, args...)
}

// Info records an info level log.
func Info(args ...interface{}) {
	std.Info(args...)
}

// Infof formats and records an info level log.
func Infof(format string, args ...interface{}) {
	std.Infof(format, args...)
}

// Warn records a warning level log.
func Warn(args ...interface{}) {
	std.Warn(args...)
}

// Warnf formats and records a warning level log.
func Warnf(format string, args ...interface{}) {
	std.Warnf(format, args...)
}

// Error records an error level log.
func Error(args ...interface{}) {
	std.Error(args...)
}

// Errorf formats and records an error level log.
func Errorf(format string, args ...interface{}) {
	std.Errorf(format, args...)
}

// Fatal records a fatal level log.
func Fatal(args ...interface{}) {
	std.Fatal(args...)
}

// Fatalf formats and records a fatal level log.
func Fatalf(format string, args ...interface{}) {
	std.Fatalf(format, args...)
}

// WithFields adds extended parameters to the log and returns a new Logger instance.
func WithFields(fields map[string]interface{}) Logger {
	return std.WithFields(fields)
}

// WithField adds extended named parameter to the log and returns a new Logger instance.
func WithField(key string, value interface{}) Logger {
	return std.WithField(key, value)
}

// WithError adds an error to the log and returns a new Logger instance.
func WithError(err error) Logger {
	return std.WithError(err)
}

// SetLevel sets the log level of the global default logger.
func SetLevel(s string) {
	std.level.SetLevel(parseLevel(s))
}

// GetLevel gets the log level of the global default logger.
func GetLevel() zapcore.Level {
	return std.level.Level()
}

// SetFormatter sets the log formatter of the global default logger.
// Make sure to call this method before using the global logger.
func SetFormatter(s string) {
	l := newZapLoggerFrom(std.optLevel, s, std.optTap)
	l.level.SetLevel(std.level.Level()) // Reset to the latest level.
	std.zapLogger = l
}

// SetOutput sets the log output of the global default logger.
func SetOutput(w io.Writer) {
	var terminal bool
	if f, ok := w.(*os.File); ok {
		terminal = term.IsTerminal(int(f.Fd()))
	}
	stdErr.Writer, stdOut.Writer = w, w
	// Match console color output.
	if stdErr.IsTerminal() != terminal || stdOut.IsTerminal() != terminal {
		stdErr.terminal, stdOut.terminal = terminal, terminal
		l := newZapLoggerFrom(std.optLevel, std.optFormat, std.optTap)
		l.level.SetLevel(std.level.Level()) // Reset to the latest level.
		std.zapLogger = l
	}
}

// This is the hook after fatal logging.
type fatalLogHook struct {
	handlers []func()
}

// Register registers the given fatal logging hook.
func (h *fatalLogHook) Register(f func()) {
	h.handlers = append(h.handlers, f)
}

// OnWrite is invoked with the CheckedEntry that was written and a list
// of fields added with that entry.
// The list of fields DOES NOT include fields that were already added
// to the logger with the With method.
// This method is an implementation of the zapcore.CheckWriteHook interface.
func (h *fatalLogHook) OnWrite(_ *zapcore.CheckedEntry, _ []zapcore.Field) {
	for _, handler := range h.handlers {
		handler()
	}
}

// Internal wrapper for the logger.
// This wrapper is used to ensure that the top-level reference to the global default logger does not change.
type loggerWrapper struct {
	*zapLogger
}

// Creates a wrapper instance from the given logger.
func newLoggerWrapper(l *zapLogger) *loggerWrapper {
	return &loggerWrapper{l}
}

// SimpleLogRecord defines the log records captured by the log catcher.
// This struct serves the test case.
type SimpleLogRecord struct {
	Level   zapcore.Level
	Message string
}

// Capture captures log output triggered by the given function.
// This method serves the test case.
func Capture(f func()) []SimpleLogRecord {
	var r []SimpleLogRecord
	std.addHook(func(e zapcore.Entry) {
		r = append(r, SimpleLogRecord{e.Level, e.Message})
	})
	defer std.addHook(nil)

	f()
	return r
}
