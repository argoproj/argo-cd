package log

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	l := New()
	assert.NotNil(t, l, "New(): return nil logger.")
}

func doLoggerTestWith(tap bool, format string, fn ...func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer)) {
	// Capture log output.
	stdErrBuf, stdOutBuf := new(bytes.Buffer), new(bytes.Buffer)
	defStdErr, defStdOut := stdErr.Writer, stdOut.Writer
	stdErr.Writer, stdOut.Writer = stdErrBuf, stdOutBuf
	defer func() { stdErr.Writer, stdOut.Writer = defStdErr, defStdOut }()

	oldLevel := os.Getenv("ARGOCD_LOG_LEVEL")
	oldTap := os.Getenv("ARGOCD_LOG_TAP")
	oldFormat := os.Getenv("ARGOCD_LOG_FORMAT")
	defer func() {
		_ = os.Setenv("ARGOCD_LOG_LEVEL", oldLevel)
		_ = os.Setenv("ARGOCD_LOG_TAP", oldTap)
		_ = os.Setenv("ARGOCD_LOG_FORMAT", oldFormat)
	}()

	// Use the lowest log level to test all log output.
	_ = os.Setenv("ARGOCD_LOG_LEVEL", "debug")

	// Formatter: text; Tap: false
	_ = os.Setenv("ARGOCD_LOG_FORMAT", format)
	if tap {
		_ = os.Setenv("ARGOCD_LOG_TAP", "1")
	} else {
		_ = os.Setenv("ARGOCD_LOG_TAP", "0")
	}

	l := New()
	for i, j := 0, len(fn); i < j; i++ {
		stdErrBuf.Reset()
		stdOutBuf.Reset()
		fn[i](l, stdErrBuf, stdOutBuf)
	}
}

func TestLoggerWithTextFormat(t *testing.T) {
	assertLog := func(want, got string) {
		_, text, _ := strings.Cut(got, " ")
		assert.Equal(t, want, text, want, got)
	}

	// Formatter: text; Tap: false
	doLoggerTestWith(false, "text",
		// Debug
		func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Debug("test.debug")
			assertLog("DBG test.debug\n", stdErrBuf.String())
			assertLog("", stdOutBuf.String())
		}, func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Debugf("test.debug.%d", 1)
			assertLog("DBG test.debug.1\n", stdErrBuf.String())
			assertLog("", stdOutBuf.String())
		},
		// Info
		func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Info("test.info")
			assertLog("INF test.info\n", stdErrBuf.String())
			assertLog("", stdOutBuf.String())
		}, func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Infof("test.info.%d", 1)
			assertLog("INF test.info.1\n", stdErrBuf.String())
			assertLog("", stdOutBuf.String())
		},
		// Warn
		func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Warn("test.warn")
			assertLog("WAN test.warn\n", stdErrBuf.String())
			assertLog("", stdOutBuf.String())
		}, func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Warnf("test.warn.%d", 1)
			assertLog("WAN test.warn.1\n", stdErrBuf.String())
			assertLog("", stdOutBuf.String())
		},
		// Error
		func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Error("test.error")
			assertLog("ERR test.error\n", stdErrBuf.String())
			assertLog("", stdOutBuf.String())
		}, func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Errorf("test.error.%d", 1)
			assertLog("ERR test.error.1\n", stdErrBuf.String())
			assertLog("", stdOutBuf.String())
		},
		// Fields
		func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.WithFields(Fields{"key1": 1, "key2": 2}).Info("fields")
			assertLog("INF fields {\"key1\": 1, \"key2\": 2}\n", stdErrBuf.String())
			assertLog("", stdOutBuf.String())
		}, func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.WithField("key1", 1).Info("field")
			assertLog("INF field {\"key1\": 1}\n", stdErrBuf.String())
			assertLog("", stdOutBuf.String())
		}, func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.WithError(errors.New("err1")).Error("error")
			assertLog("ERR error {\"error\": \"err1\"}\n", stdErrBuf.String())
			assertLog("", stdOutBuf.String())
		},
	)
}

func TestLoggerWithJSONFormat(t *testing.T) {
	assertLog := func(want map[string]interface{}, got string) {
		if len(want) == 0 {
			assert.Empty(t, got, got)
			return
		}
		m := make(map[string]interface{})
		err := json.Unmarshal([]byte(got), &m)
		assert.NoError(t, err, want, got)
		delete(m, "ts")
		b1, _ := json.Marshal(m)
		b2, _ := json.Marshal(want)
		assert.Equal(t, b2, b1, string(b2), string(b1))
	}

	doLoggerTestWith(false, "json",
		// Debug
		func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Debug("test.debug")
			assertLog(map[string]interface{}{"level": "DBG", "msg": "test.debug"}, stdErrBuf.String())
			assertLog(nil, stdOutBuf.String())
		}, func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Debugf("test.debug.%d", 1)
			assertLog(map[string]interface{}{"level": "DBG", "msg": "test.debug.1"}, stdErrBuf.String())
			assertLog(nil, stdOutBuf.String())
		},
		// Info
		func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Info("test.info")
			assertLog(map[string]interface{}{"level": "INF", "msg": "test.info"}, stdErrBuf.String())
			assertLog(nil, stdOutBuf.String())
		}, func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Infof("test.info.%d", 1)
			assertLog(map[string]interface{}{"level": "INF", "msg": "test.info.1"}, stdErrBuf.String())
			assertLog(nil, stdOutBuf.String())
		},
		// Warn
		func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Warn("test.warn")
			assertLog(map[string]interface{}{"level": "WAN", "msg": "test.warn"}, stdErrBuf.String())
			assertLog(nil, stdOutBuf.String())
		}, func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Warnf("test.warn.%d", 1)
			assertLog(map[string]interface{}{"level": "WAN", "msg": "test.warn.1"}, stdErrBuf.String())
			assertLog(nil, stdOutBuf.String())
		},
		// Error
		func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Error("test.error")
			assertLog(map[string]interface{}{"level": "ERR", "msg": "test.error"}, stdErrBuf.String())
			assertLog(nil, stdOutBuf.String())
		}, func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Errorf("test.error.%d", 1)
			assertLog(map[string]interface{}{"level": "ERR", "msg": "test.error.1"}, stdErrBuf.String())
			assertLog(nil, stdOutBuf.String())
		},
		// Fields
		func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.WithFields(Fields{"key1": 1, "key2": 2}).Info("fields")
			assertLog(map[string]interface{}{"level": "INF", "msg": "fields", "key1": 1, "key2": 2}, stdErrBuf.String())
			assertLog(nil, stdOutBuf.String())
		}, func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.WithField("key1", 1).Info("field")
			assertLog(map[string]interface{}{"level": "INF", "msg": "field", "key1": 1}, stdErrBuf.String())
			assertLog(nil, stdOutBuf.String())
		}, func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.WithError(errors.New("err1")).Error("error")
			assertLog(map[string]interface{}{"level": "ERR", "msg": "error", "error": "err1"}, stdErrBuf.String())
			assertLog(nil, stdOutBuf.String())
		},
	)
}

func TestLoggerWithTextFormatAndTap(t *testing.T) {
	assertLog := func(want, got string) {
		_, text, _ := strings.Cut(got, " ")
		assert.Equal(t, want, text, want, got)
	}

	doLoggerTestWith(true, "text",
		// Debug
		func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Debug("test.debug")
			assertLog("DBG test.debug\n", stdOutBuf.String())
			assertLog("", stdErrBuf.String())
		}, func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Debugf("test.debug.%d", 1)
			assertLog("DBG test.debug.1\n", stdOutBuf.String())
			assertLog("", stdErrBuf.String())
		},
		// Info
		func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Info("test.info")
			assertLog("INF test.info\n", stdOutBuf.String())
			assertLog("", stdErrBuf.String())
		}, func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Infof("test.info.%d", 1)
			assertLog("INF test.info.1\n", stdOutBuf.String())
			assertLog("", stdErrBuf.String())
		},
		// Warn
		func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Warn("test.warn")
			assertLog("WAN test.warn\n", stdOutBuf.String())
			assertLog("", stdErrBuf.String())
		}, func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Warnf("test.warn.%d", 1)
			assertLog("WAN test.warn.1\n", stdOutBuf.String())
			assertLog("", stdErrBuf.String())
		},
		// Error
		func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Error("test.error")
			assertLog("ERR test.error\n", stdErrBuf.String())
			assertLog("", stdOutBuf.String())
		}, func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Errorf("test.error.%d", 1)
			assertLog("ERR test.error.1\n", stdErrBuf.String())
			assertLog("", stdOutBuf.String())
		},
		// Fields
		func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.WithFields(Fields{"key1": 1, "key2": 2}).Info("fields")
			assertLog("INF fields {\"key1\": 1, \"key2\": 2}\n", stdOutBuf.String())
			assertLog("", stdErrBuf.String())
		}, func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.WithField("key1", 1).Info("field")
			assertLog("INF field {\"key1\": 1}\n", stdOutBuf.String())
			assertLog("", stdErrBuf.String())
		}, func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.WithError(errors.New("err1")).Error("error")
			assertLog("ERR error {\"error\": \"err1\"}\n", stdErrBuf.String())
			assertLog("", stdOutBuf.String())
		},
	)
}

func TestLoggerWithJSONFormatAndTap(t *testing.T) {
	assertLog := func(want map[string]interface{}, got string) {
		if len(want) == 0 {
			assert.Empty(t, got, got)
			return
		}
		m := make(map[string]interface{})
		err := json.Unmarshal([]byte(got), &m)
		assert.NoError(t, err, want, got)
		delete(m, "ts")
		b1, _ := json.Marshal(m)
		b2, _ := json.Marshal(want)
		assert.Equal(t, b2, b1, string(b2), string(b1))
	}

	doLoggerTestWith(true, "json",
		// Debug
		func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Debug("test.debug")
			assertLog(map[string]interface{}{"level": "DBG", "msg": "test.debug"}, stdOutBuf.String())
			assertLog(nil, stdErrBuf.String())
		}, func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Debugf("test.debug.%d", 1)
			assertLog(map[string]interface{}{"level": "DBG", "msg": "test.debug.1"}, stdOutBuf.String())
			assertLog(nil, stdErrBuf.String())
		},
		// Info
		func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Info("test.info")
			assertLog(map[string]interface{}{"level": "INF", "msg": "test.info"}, stdOutBuf.String())
			assertLog(nil, stdErrBuf.String())
		}, func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Infof("test.info.%d", 1)
			assertLog(map[string]interface{}{"level": "INF", "msg": "test.info.1"}, stdOutBuf.String())
			assertLog(nil, stdErrBuf.String())
		},
		// Warn
		func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Warn("test.warn")
			assertLog(map[string]interface{}{"level": "WAN", "msg": "test.warn"}, stdOutBuf.String())
			assertLog(nil, stdErrBuf.String())
		}, func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Warnf("test.warn.%d", 1)
			assertLog(map[string]interface{}{"level": "WAN", "msg": "test.warn.1"}, stdOutBuf.String())
			assertLog(nil, stdErrBuf.String())
		},
		// Error
		func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Error("test.error")
			assertLog(map[string]interface{}{"level": "ERR", "msg": "test.error"}, stdErrBuf.String())
			assertLog(nil, stdOutBuf.String())
		}, func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Errorf("test.error.%d", 1)
			assertLog(map[string]interface{}{"level": "ERR", "msg": "test.error.1"}, stdErrBuf.String())
			assertLog(nil, stdOutBuf.String())
		},
		// Fields
		func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.WithFields(Fields{"key1": 1, "key2": 2}).Info("fields")
			assertLog(map[string]interface{}{"level": "INF", "msg": "fields", "key1": 1, "key2": 2}, stdOutBuf.String())
			assertLog(nil, stdErrBuf.String())
		}, func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.WithField("key1", 1).Info("field")
			assertLog(map[string]interface{}{"level": "INF", "msg": "field", "key1": 1}, stdOutBuf.String())
			assertLog(nil, stdErrBuf.String())
		}, func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.WithError(errors.New("err1")).Error("error")
			assertLog(map[string]interface{}{"level": "ERR", "msg": "error", "error": "err1"}, stdErrBuf.String())
			assertLog(nil, stdOutBuf.String())
		},
	)
}

func TestLoggerWithTextFormatAndColor(t *testing.T) {
	assertLog := func(want, got string) {
		_, text, _ := strings.Cut(got, " ")
		assert.Equal(t, want, text, want, got)
	}

	oldColor := os.Getenv("FORCE_LOG_COLORS")
	defer func() {
		_ = os.Setenv("FORCE_LOG_COLORS", oldColor)
	}()
	_ = os.Setenv("FORCE_LOG_COLORS", "1")

	// Formatter: text; Tap: false
	doLoggerTestWith(false, "text",
		// Debug
		func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Debug("test.debug")
			assertLog("\u001B[36mDBG\u001B[0m test.debug\n", stdErrBuf.String())
			assertLog("", stdOutBuf.String())
		},
		// Info
		func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Info("test.info")
			assertLog("\u001B[96mINF\u001B[0m test.info\n", stdErrBuf.String())
			assertLog("", stdOutBuf.String())
		},
		// Warn
		func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Warn("test.warn")
			assertLog("\u001B[92mWAN\u001B[0m test.warn\n", stdErrBuf.String())
			assertLog("", stdOutBuf.String())
		},
		// Error
		func(l Logger, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Error("test.error")
			assertLog("\u001B[93mERR\u001B[0m test.error\n", stdErrBuf.String())
			assertLog("", stdOutBuf.String())
		},
	)
}
