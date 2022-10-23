package log

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/term"
)

const (
	JsonFormat = "json"
	TextFormat = "text"
)

const (
	// EnvLogFormat log format that is defined by `--logformat` option
	EnvLogFormat = "ARGOCD_LOG_FORMAT"
	// EnvLogLevel log level that is defined by `--loglevel` option
	EnvLogLevel = "ARGOCD_LOG_LEVEL"
	// EnvLogTap log tap that is defined by `--logtap` option
	EnvLogTap = "ARGOCD_LOG_TAP"
)

const (
	PanicLevel = zap.PanicLevel
	FatalLevel = zap.FatalLevel
	ErrorLevel = zap.ErrorLevel
	WarnLevel  = zap.WarnLevel
	InfoLevel  = zap.InfoLevel
	DebugLevel = zap.DebugLevel
	TraceLevel = zap.DebugLevel // Zap has no corresponding level.
)

// Fields defines log extension fields.
// This is just an alias to reduce code length.
type Fields map[string]interface{}

// Logger defines the system standard logger.
// This interface is used to mask the details of the underlying log driver, and any logger
// that implements this interface can be used directly in the system without any modification.
type Logger interface {
	Print(args ...interface{})
	Printf(format string, args ...interface{})

	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Warn(args ...interface{})
	Warnf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})

	// WithFields adds extended parameters to the log and returns a new Logger instance.
	WithFields(fields map[string]interface{}) Logger
	// WithField adds extended named parameter to the log and returns a new Logger instance.
	WithField(key string, value interface{}) Logger
	// WithError adds an error to the log and returns a new Logger instance.
	WithError(err error) Logger

	// AsLogrLogger converts the current logger to a logr logger.
	AsLogrLogger() logr.Logger

	// Driver returns the log driver instance.
	Driver() *zap.Logger
}

// New creates and returns a new logger instance with the current default configuration.
func New() Logger {
	return newZapLogger()
}

var (
	stdErr = newOutputWrapper(os.Stderr)
	stdOut = newOutputWrapper(os.Stdout)
)

// This is a log output wrapper that extends standard output and standard error.
type outputWrapper struct {
	io.Writer
	terminal bool
}

// Creates an output wrapper instance from the given file object.
func newOutputWrapper(f *os.File) *outputWrapper {
	return &outputWrapper{Writer: f, terminal: term.IsTerminal(int(f.Fd()))}
}

// IsTerminal determines whether the current output driver is a terminal.
func (w *outputWrapper) IsTerminal() bool {
	return w.terminal
}

// Creates and returns a wrapper instance of a new zap logger.
// The returned wrapper instance already implements the Logger interface.
func newZapLogger() *zapLogger {
	return newZapLoggerFrom(os.Getenv(EnvLogLevel), os.Getenv(EnvLogFormat), os.Getenv(EnvLogTap))
}

// Creates and returns a wrapper instance of a new zap logger from the given options.
func newZapLoggerFrom(level, format, tap string) *zapLogger {
	lvl := zap.NewAtomicLevelAt(parseLevel(level))
	w := &zapLogger{level: &lvl, optLevel: level, optFormat: format, optTap: tap}
	var cores []zapcore.Core
	if parseTap(tap) {
		high, low := parseTapEncoder(format)
		cores = []zapcore.Core{
			zapcore.NewCore(high, zapcore.AddSync(stdErr), zap.LevelEnablerFunc(w.isHighPriorityLevel)),
			zapcore.NewCore(low, zapcore.AddSync(stdOut), zap.LevelEnablerFunc(w.isLowPriorityLevel)),
		}
	} else {
		encoder := parseEncoder(format)
		cores = []zapcore.Core{
			zapcore.NewCore(encoder, zapcore.AddSync(stdErr), w.level),
		}
	}
	l := zap.New(zapcore.NewTee(cores...), zap.WithFatalHook(defaultFatalLogHook), zap.Hooks(w.doHook))
	w.log = l.Sugar()
	return w
}

// zapLogger defines a zap logger wrapper.
// We converted the zap logger to the system standard logger.
type zapLogger struct {
	log   *zap.SugaredLogger
	level *zap.AtomicLevel

	optLevel  string
	optFormat string
	optTap    string

	// Hooks are used to capture log message.
	// Just for testing.
	hook func(zapcore.Entry)
}

// Print records a debug level log.
func (o *zapLogger) Print(args ...interface{}) {
	o.log.Debug(args...)
}

// Printf formats and records a debug level log.
func (o *zapLogger) Printf(format string, args ...interface{}) {
	o.log.Debugf(format, args...)
}

// Debug records a debug level log.
func (o *zapLogger) Debug(args ...interface{}) {
	o.log.Debug(args...)
}

// Debugf formats and records a debug level log.
func (o *zapLogger) Debugf(format string, args ...interface{}) {
	o.log.Debugf(format, args...)
}

// Info records an info level log.
func (o *zapLogger) Info(args ...interface{}) {
	o.log.Info(args...)
}

// Infof formats and records an info level log.
func (o *zapLogger) Infof(format string, args ...interface{}) {
	o.log.Infof(format, args...)
}

// Warn records a warning level log.
func (o *zapLogger) Warn(args ...interface{}) {
	o.log.Warn(args...)
}

// Warnf formats and records a warning level log.
func (o *zapLogger) Warnf(format string, args ...interface{}) {
	o.log.Warnf(format, args...)
}

// Error records an error level log.
func (o *zapLogger) Error(args ...interface{}) {
	o.log.Error(args...)
}

// Errorf formats and records an error level log.
func (o *zapLogger) Errorf(format string, args ...interface{}) {
	o.log.Errorf(format, args...)
}

// Fatal records a fatal level log.
func (o *zapLogger) Fatal(args ...interface{}) {
	o.log.Error(args...)
}

// Fatalf formats and records a fatal level log.
func (o *zapLogger) Fatalf(format string, args ...interface{}) {
	o.log.Errorf(format, args...)
}

// WithFields returns a new logger with the given extension parameters added.
// The returned Logger will inherit all extended parameters of all parent loggers.
func (o *zapLogger) WithFields(fields map[string]interface{}) Logger {
	if len(fields) == 0 {
		return o
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	if len(keys) > 1 {
		// It is necessary to keep the extended parameters in order when outputting the
		// log in the console, so that the log output is predictable by humans.
		sort.Strings(keys)
	}
	items := make([]interface{}, 0, len(fields)*2)
	for i, j := 0, len(keys); i < j; i++ {
		items = append(items, keys[i], fields[keys[i]])
	}
	return &zapLogger{log: o.log.With(items...)}
}

// WithField adds extended named parameter to the log and returns a new Logger instance.
// The returned Logger will inherit all extended parameters of all parent loggers.
func (o *zapLogger) WithField(key string, value interface{}) Logger {
	return &zapLogger{log: o.log.With(key, value)}
}

// WithError adds an error to the log and returns a new Logger instance.
// The returned Logger will inherit all extended parameters of all parent loggers.
func (o *zapLogger) WithError(err error) Logger {
	return &zapLogger{log: o.log.With("error", err)}
}

// AsLogrLogger converts the current logger to a logr logger.
func (o *zapLogger) AsLogrLogger() logr.Logger {
	return logr.New(&bridgeLogSink{l: newLoggerWrapper(o)})
}

// Driver returns the log driver instance.
func (o *zapLogger) Driver() *zap.Logger {
	return o.log.Desugar()
}

// Determines whether the given level is a high priority level.
func (o *zapLogger) isHighPriorityLevel(level zapcore.Level) bool {
	return o.level.Enabled(level) && level >= zapcore.ErrorLevel
}

// Determines whether the given level is a low priority level.
func (o *zapLogger) isLowPriorityLevel(level zapcore.Level) bool {
	return o.level.Enabled(level) && level < zapcore.ErrorLevel
}

// Adds a logging hook that, if given nil, removes the hook.
func (o *zapLogger) addHook(hook func(zapcore.Entry)) {
	o.hook = hook
}

// Execute the bound hook, if one exists.
func (o *zapLogger) doHook(entry zapcore.Entry) error {
	if o.hook != nil {
		o.hook(entry)
	}
	return nil
}

// Parses and creates a new log encoder from the given format.
func parseEncoder(format string) zapcore.Encoder {
	switch strings.ToLower(format) {
	case JsonFormat:
		return newEncoder(zapcore.NewJSONEncoder, liveEncoder)
	case TextFormat:
		return newConsoleEncoder(os.Getenv("FORCE_LOG_COLORS") == "1")
	default:
		return newConsoleEncoder(false)
	}
}

// Creates a new console log encoder.
// The parameter colors controls whether to force the console color adapter to be enabled.
func newConsoleEncoder(colors bool) zapcore.Encoder {
	if colors || stdErr.IsTerminal() {
		return newEncoder(zapcore.NewConsoleEncoder, liveColorEncoder)
	}
	return newEncoder(zapcore.NewConsoleEncoder, liveEncoder)
}

// Parses and creates a new tap console log encoder from the given format.
func parseTapEncoder(format string) (high zapcore.Encoder, low zapcore.Encoder) {
	switch strings.ToLower(format) {
	case JsonFormat:
		high = newEncoder(zapcore.NewJSONEncoder, liveEncoder)
		low = high
	case TextFormat:
		high, low = newTapConsoleEncoder(os.Getenv("FORCE_LOG_COLORS") == "1")
	default:
		high, low = newTapConsoleEncoder(false)
	}
	return
}

// Creates a new tap console log encoder.
// The parameter colors controls whether to force the console color adapter to be enabled.
func newTapConsoleEncoder(colors bool) (high zapcore.Encoder, low zapcore.Encoder) {
	if colors {
		high = newEncoder(zapcore.NewConsoleEncoder, liveColorEncoder)
		low = high
		return
	}
	if stdErr.IsTerminal() {
		high = newEncoder(zapcore.NewConsoleEncoder, liveColorEncoder)
	} else {
		high = newEncoder(zapcore.NewConsoleEncoder, liveEncoder)
	}
	if stdOut.IsTerminal() {
		low = newEncoder(zapcore.NewConsoleEncoder, liveColorEncoder)
	} else {
		low = newEncoder(zapcore.NewConsoleEncoder, liveEncoder)
	}
	return
}

// Create a zap log encoder from the given encoder factory and log level encoder.
func newEncoder(factory func(cfg zapcore.EncoderConfig) zapcore.Encoder, enc zapcore.LevelEncoder) zapcore.Encoder {
	cfg := zap.NewProductionEncoderConfig()
	cfg.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02T15:04:05.000Z07:00")
	cfg.ConsoleSeparator = " "
	cfg.EncodeLevel = enc
	return factory(cfg)
}

// This is a string formatted collection of all supported log levels.
// We use these predefined strings to convert log levels to strings.
// We don't use zap's built-in format because zap's built-in format is not aesthetically
// pleasing for console output, and we want it to be clearer and more comfortable when
// viewing logs in the console.
var levelStrings = map[zapcore.Level][2]string{
	zapcore.DebugLevel:  {"DBG", "\u001B[36mDBG\u001B[0m"},
	zapcore.InfoLevel:   {"INF", "\u001B[96mINF\u001B[0m"},
	zapcore.WarnLevel:   {"WAN", "\u001B[92mWAN\u001B[0m"},
	zapcore.ErrorLevel:  {"ERR", "\u001B[93mERR\u001B[0m"},
	zapcore.DPanicLevel: {"PNC", "\u001B[95mPNC\u001B[0m"},
	zapcore.PanicLevel:  {"PNC", "\u001B[31mPNC\u001B[0m"},
	zapcore.FatalLevel:  {"FAT", "\u001B[91mFAT\u001B[0m"},
}

// Encodes the given log level as a regular string.
func liveEncoder(level zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(levelStrings[level][0])
}

// Encodes the given log level as a console-colored string.
func liveColorEncoder(level zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(levelStrings[level][1])
}

// Parse log level from environment variable.
func parseLevel(s string) zapcore.Level {
	var level zapcore.Level
	switch strings.ToLower(s) {
	case "panic":
		level = zap.PanicLevel
	case "fatal":
		level = zap.FatalLevel
	case "error":
		level = zap.ErrorLevel
	case "warn", "warning":
		level = zap.WarnLevel
	case "info":
		level = zap.InfoLevel
	case "debug":
		level = zap.DebugLevel
	case "trace": // Zap has no corresponding level.
		level = zap.DebugLevel
	default:
		level = zap.InfoLevel
	}
	return level
}

// Parse log tap from environment variable.
func parseTap(s string) bool {
	// Ignore error, we don't use the log output tap by default.
	tap, _ := strconv.ParseBool(s)
	return tap
}

// CheckLevel determines whether the given log level is valid.
func CheckLevel(s string) error {
	switch strings.ToLower(s) {
	case "panic", "fatal", "error", "warn", "warning", "info", "debug", "trace":
		return nil
	}
	return fmt.Errorf("unknown log level %s", s)
}

// CheckFormat determines whether the given log format is valid.
func CheckFormat(s string) error {
	switch strings.ToLower(s) {
	case "json", "text":
		return nil
	}
	return fmt.Errorf("unknown log format %s", s)
}

// CheckTap determines whether the given log tap is valid.
func CheckTap(s string) error {
	if _, err := strconv.ParseBool(s); err != nil {
		return fmt.Errorf("unknown log tap %s", s)
	}
	return nil
}
