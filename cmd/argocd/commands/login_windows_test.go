//go:build windows

package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// A batch file wrapper would run through cmd.exe, which runs whatever follows a "&" in the URL.
func Test_startBrowserCommand_refusesCmdExe(t *testing.T) {
	const url = "http://test-sso-browser-flow.com/auth?a=1&b=2"
	wrapper := filepath.Join(t.TempDir(), "browser.cmd")
	require.NoError(t, os.WriteFile(wrapper, []byte("@echo off\r\n"), 0o600))
	for _, browser := range []string{"cmd /c start %s", "'" + wrapper + "' %s"} {
		require.ErrorContains(t, startBrowserCommand(browser, url), "cmd.exe would interpret the URL", browser)
	}
}
