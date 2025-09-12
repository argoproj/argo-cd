package exec

import (
	"os/exec"
	"regexp"
	"syscall"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_timeout(t *testing.T) {
	t.Run("Default", func(t *testing.T) {
		initTimeout()
		assert.Equal(t, 90*time.Second, timeout)
		assert.Equal(t, 10*time.Second, fatalTimeout)
	})
	t.Run("Default", func(t *testing.T) {
		t.Setenv("ARGOCD_EXEC_TIMEOUT", "1s")
		t.Setenv("ARGOCD_EXEC_FATAL_TIMEOUT", "2s")
		initTimeout()
		assert.Equal(t, 1*time.Second, timeout)
		assert.Equal(t, 2*time.Second, fatalTimeout)
	})
}

func TestRun(t *testing.T) {
	out, err := Run(exec.Command("ls"))
	require.NoError(t, err)
	assert.NotEmpty(t, out)
}

func TestHideUsernamePassword(t *testing.T) {
	_, err := RunWithRedactor(exec.Command("helm registry login https://charts.bitnami.com/bitnami", "--username", "foo", "--password", "bar"), nil)
	require.Error(t, err)

	redactor := func(text string) string {
		return regexp.MustCompile("(--username|--password) [^ ]*").ReplaceAllString(text, "$1 ******")
	}
	_, err = RunWithRedactor(exec.Command("helm registry login https://charts.bitnami.com/bitnami", "--username", "foo", "--password", "bar"), redactor)
	require.Error(t, err)
}

// This tests a cmd that properly handles a SIGTERM signal
func TestRunWithExecRunOpts(t *testing.T) {
	t.Setenv("ARGOCD_EXEC_TIMEOUT", "200ms")
	initTimeout()

	opts := ExecRunOpts{
		TimeoutBehavior: TimeoutBehavior{
			Signal:     syscall.SIGTERM,
			ShouldWait: true,
		},
	}
	_, err := RunWithExecRunOpts(exec.Command("sh", "-c", "trap 'trap - 15 && echo captured && exit' 15 && sleep 2"), opts)
	assert.ErrorContains(t, err, "failed timeout after 200ms")
}

// This tests a mis-behaved cmd that stalls on SIGTERM and requires a SIGKILL
func TestRunWithExecRunOptsFatal(t *testing.T) {
	t.Setenv("ARGOCD_EXEC_TIMEOUT", "200ms")
	t.Setenv("ARGOCD_EXEC_FATAL_TIMEOUT", "100ms")

	initTimeout()

	opts := ExecRunOpts{
		TimeoutBehavior: TimeoutBehavior{
			Signal:     syscall.SIGTERM,
			ShouldWait: true,
		},
	}
	// The returned error string in this case should contain a "fatal" in this case
	_, err := RunWithExecRunOpts(exec.Command("sh", "-c", "trap 'trap - 15 && echo captured && sleep 10000' 15 && sleep 2"), opts)
	// The expected timeout is ARGOCD_EXEC_TIMEOUT + ARGOCD_EXEC_FATAL_TIMEOUT = 200ms + 100ms = 300ms
	assert.ErrorContains(t, err, "failed fatal timeout after 300ms")
}

func Test_getCommandArgsToLog(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "no spaces",
			args:     []string{"sh", "-c", "cat"},
			expected: "sh -c cat",
		},
		{
			name:     "spaces",
			args:     []string{"sh", "-c", `echo "hello world"`},
			expected: `sh -c "echo \"hello world\""`,
		},
		{
			name:     "empty string arg",
			args:     []string{"sh", "-c", ""},
			expected: `sh -c ""`,
		},
	}

	for _, tc := range testCases {
		tcc := tc
		t.Run(tcc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tcc.expected, GetCommandArgsToLog(exec.Command(tcc.args[0], tcc.args[1:]...)))
		})
	}
}

func TestRunCommand(t *testing.T) {
	hook := test.NewGlobal()
	log.SetLevel(log.DebugLevel)
	defer log.SetLevel(log.InfoLevel)

	message, err := RunCommand("echo", CmdOpts{Redactor: Redact([]string{"world"})}, "hello world")
	require.NoError(t, err)
	assert.Equal(t, "hello world", message)

	assert.Len(t, hook.Entries, 2)

	entry := hook.Entries[0]
	assert.Equal(t, log.InfoLevel, entry.Level)
	assert.Equal(t, "echo hello ******", entry.Message)
	assert.Contains(t, entry.Data, "dir")
	assert.Contains(t, entry.Data, "execID")

	entry = hook.Entries[1]
	assert.Equal(t, log.DebugLevel, entry.Level)
	assert.Equal(t, "hello ******\n", entry.Message)
	assert.Contains(t, entry.Data, "duration")
	assert.Contains(t, entry.Data, "execID")
}

func TestRunCommandSignal(t *testing.T) {
	hook := test.NewGlobal()
	log.SetLevel(log.DebugLevel)
	defer log.SetLevel(log.InfoLevel)

	timeoutBehavior := TimeoutBehavior{Signal: syscall.SIGTERM, ShouldWait: true}
	output, err := RunCommand("sh", CmdOpts{Timeout: 200 * time.Millisecond, TimeoutBehavior: timeoutBehavior}, "-c", "trap 'trap - 15 && echo captured && exit' 15 && sleep 2")
	assert.Equal(t, "captured", output)
	require.EqualError(t, err, "`sh -c trap 'trap - 15 && echo captured && exit' 15 && sleep 2` failed timeout after 200ms")

	assert.Len(t, hook.Entries, 3)
}

func TestTrimmedOutput(t *testing.T) {
	message, err := RunCommand("printf", CmdOpts{}, "hello world")
	require.NoError(t, err)
	assert.Equal(t, "hello world", message)
}

func TestRunCommandExitErr(t *testing.T) {
	hook := test.NewGlobal()
	log.SetLevel(log.DebugLevel)
	defer log.SetLevel(log.InfoLevel)

	output, err := RunCommand("sh", CmdOpts{Redactor: Redact([]string{"world"})}, "-c", "echo hello world && echo my-error >&2 && exit 1")
	assert.Equal(t, "hello world", output)
	require.EqualError(t, err, "`sh -c echo hello ****** && echo my-error >&2 && exit 1` failed exit status 1: my-error")

	assert.Len(t, hook.Entries, 3)

	entry := hook.Entries[0]
	assert.Equal(t, log.InfoLevel, entry.Level)
	assert.Equal(t, "sh -c echo hello ****** && echo my-error >&2 && exit 1", entry.Message)
	assert.Contains(t, entry.Data, "dir")
	assert.Contains(t, entry.Data, "execID")

	entry = hook.Entries[1]
	assert.Equal(t, log.DebugLevel, entry.Level)
	assert.Equal(t, "hello ******\n", entry.Message)
	assert.Contains(t, entry.Data, "duration")
	assert.Contains(t, entry.Data, "execID")

	entry = hook.Entries[2]
	assert.Equal(t, log.ErrorLevel, entry.Level)
	assert.Equal(t, "`sh -c echo hello ****** && echo my-error >&2 && exit 1` failed exit status 1: my-error", entry.Message)
	assert.Contains(t, entry.Data, "execID")
}

func TestRunCommandErr(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	defer log.SetLevel(log.InfoLevel)

	output, err := RunCommand("sh", CmdOpts{Redactor: Redact([]string{"world"})}, "-c", ">&2 echo 'failure'; false")
	assert.Empty(t, output)
	assert.EqualError(t, err, "`sh -c >&2 echo 'failure'; false` failed exit status 1: failure")
}

func TestRunInDir(t *testing.T) {
	cmd := exec.Command("pwd")
	cmd.Dir = "/"
	message, err := RunCommandExt(cmd, CmdOpts{})
	require.NoError(t, err)
	assert.Equal(t, "/", message)
}

func TestRedact(t *testing.T) {
	assert.Empty(t, Redact(nil)(""))
	assert.Empty(t, Redact([]string{})(""))
	assert.Empty(t, Redact([]string{"foo"})(""))
	assert.Equal(t, "foo", Redact([]string{})("foo"))
	assert.Equal(t, "******", Redact([]string{"foo"})("foo"))
	assert.Equal(t, "****** ******", Redact([]string{"foo", "bar"})("foo bar"))
	assert.Equal(t, "****** ******", Redact([]string{"foo"})("foo foo"))
}

func TestRunCaptureStderr(t *testing.T) {
	output, err := RunCommand("sh", CmdOpts{CaptureStderr: true}, "-c", "echo hello world && echo my-error >&2 && exit 0")
	assert.Equal(t, "hello world\nmy-error", output)
	assert.NoError(t, err)
}
