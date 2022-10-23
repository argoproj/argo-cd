package log

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
)

func TestNewLogSink(t *testing.T) {
	l := NewLogSink()
	assert.NotNil(t, l, "NewLogSink(): return nil.")
}

func TestNewLogrLogger(t *testing.T) {
	l := NewLogrLogger()
	assert.NotNil(t, l.GetSink(), "NewLogrLogger(): return invalid value.")
}

func TestDefaultLogSink(t *testing.T) {
	l := DefaultLogSink()
	assert.NotNil(t, l, "DefaultLogSink(): return nil.")
}

func TestDefaultLogrLogger(t *testing.T) {
	l := DefaultLogrLogger()
	assert.NotNil(t, l.GetSink(), "DefaultLogrLogger(): return invalid value.")
}

func doLogSinkTestWith(tap bool, fn ...func(l logr.LogSink, stdErrBuf, stdOutBuf *bytes.Buffer)) {
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
	_ = os.Setenv("ARGOCD_LOG_FORMAT", "text")
	if tap {
		_ = os.Setenv("ARGOCD_LOG_TAP", "1")
	} else {
		_ = os.Setenv("ARGOCD_LOG_TAP", "0")
	}

	l := NewLogSink()
	for i, j := 0, len(fn); i < j; i++ {
		stdErrBuf.Reset()
		stdOutBuf.Reset()
		fn[i](l, stdErrBuf, stdOutBuf)
	}
}

func TestLogSink(t *testing.T) {
	assertLog := func(want, got string) {
		_, text, _ := strings.Cut(got, " ")
		assert.Equal(t, want, text, want, got)
	}

	doLogSinkTestWith(false,
		func(l logr.LogSink, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Info(0, "test")
			assertLog("INF test {\"V\": 0}\n", stdErrBuf.String())
			assertLog("", stdOutBuf.String())
		}, func(l logr.LogSink, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Error(errors.New("err1"), "error")
			assertLog("ERR error {\"error\": \"err1\"}\n", stdErrBuf.String())
			assertLog("", stdOutBuf.String())
		}, func(l logr.LogSink, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.WithName("foo").Info(1, "test")
			assertLog("INF foo test {\"V\": 1}\n", stdErrBuf.String())
			assertLog("", stdOutBuf.String())
		}, func(l logr.LogSink, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.WithValues("key", 1).Info(2, "test")
			assertLog("INF test {\"key\": 1, \"V\": 2}\n", stdErrBuf.String())
			assertLog("", stdOutBuf.String())
		},
	)
}

func TestLogSinkWithTap(t *testing.T) {
	assertLog := func(want, got string) {
		_, text, _ := strings.Cut(got, " ")
		assert.Equal(t, want, text, want, got)
	}

	doLogSinkTestWith(true,
		func(l logr.LogSink, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Info(0, "test")
			assertLog("INF test {\"V\": 0}\n", stdOutBuf.String())
			assertLog("", stdErrBuf.String())
		}, func(l logr.LogSink, stdErrBuf, stdOutBuf *bytes.Buffer) {
			l.Error(errors.New("err1"), "error")
			assertLog("ERR error {\"error\": \"err1\"}\n", stdErrBuf.String())
			assertLog("", stdOutBuf.String())
		},
	)
}
