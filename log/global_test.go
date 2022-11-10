package log

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
)

func TestSetLevel(t *testing.T) {
	before := std.level.Level()
	defer func() { std.level.SetLevel(before) }()

	SetLevel("error")
	after := std.level.Level()

	assert.Equal(t, zapcore.ErrorLevel, after, after)
	assert.NotEqual(t, before, after, before, after)
}

func TestSetFormatter(t *testing.T) {
	// Capture log output.
	stdErrBuf, stdOutBuf := new(bytes.Buffer), new(bytes.Buffer)
	defStdErr, defStdOut := stdErr.Writer, stdOut.Writer
	stdErr.Writer, stdOut.Writer = stdErrBuf, stdOutBuf
	defer func() { stdErr.Writer, stdOut.Writer = defStdErr, defStdOut }()

	assertTextLog := func(want, got string) {
		_, text, _ := strings.Cut(got, " ")
		assert.Equal(t, want, text, want, got)
	}
	assertJSONLog := func(want map[string]interface{}, got string) {
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

	Info("test.SetFormatter")
	assertTextLog("INF test.SetFormatter\n", stdErrBuf.String())
	stdErrBuf.Reset()

	SetFormatter("json")

	Info("test.SetFormatter")
	assertJSONLog(map[string]interface{}{"level": "INF", "msg": "test.SetFormatter"}, stdErrBuf.String())
	stdErrBuf.Reset()

	SetFormatter("unknown")
	Info("test.SetFormatter")
	assertTextLog("INF test.SetFormatter\n", stdErrBuf.String())
	stdErrBuf.Reset()
}

func TestCapture(t *testing.T) {
	r := Capture(func() { Info("test") })
	assert.Len(t, r, 1, "capture")
	if len(r) == 1 {
		assert.Equal(t, r[0].Level, InfoLevel, "capture")
		assert.Equal(t, r[0].Message, "test", "capture")
	}
}
