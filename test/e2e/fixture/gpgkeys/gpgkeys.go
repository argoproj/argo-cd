package gpgkeys

import (
	stderrors "errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	"github.com/argoproj/argo-cd/v3/util/errors"
)

// AddGPGPublicKey adds public key via API and creates the appropriate file where the ConfigMap mount would do it as well
func AddGPGPublicKey(t *testing.T) {
	t.Helper()
	keyPath, err := filepath.Abs("../fixture/gpg/" + fixture.GpgGoodKeyID)
	require.NoError(t, err)
	args := []string{"gpg", "add", "--from", keyPath}
	errors.NewHandler(t).FailOnErr(fixture.RunCli(args...))

	if fixture.IsLocal() {
		keyData, err := os.ReadFile(keyPath)
		require.NoError(t, err)
		err = os.WriteFile(fmt.Sprintf("%s/app/config/gpg/source/%s", fixture.TmpDir(), fixture.GpgGoodKeyID), keyData, 0o644)
		require.NoError(t, err)
		waitForKeyring(t, true)
	} else {
		fixture.RestartRepoServer(t)
	}
}

func DeleteGPGPublicKey(t *testing.T) {
	t.Helper()
	args := []string{"gpg", "rm", fixture.GpgGoodKeyID}
	errors.NewHandler(t).FailOnErr(fixture.RunCli(args...))
	if fixture.IsLocal() {
		require.NoError(t, os.Remove(fmt.Sprintf("%s/app/config/gpg/source/%s", fixture.TmpDir(), fixture.GpgGoodKeyID)))
		waitForKeyring(t, false)
	} else {
		fixture.RestartRepoServer(t)
	}
}

// waitForKeyring waits until the repo-server has synced its keyring with the
// key source directory. The repo-server watches that directory and imports or
// removes keys asynchronously, so a sync started right after the file changes
// can still verify against the old keyring. The gpg call takes no locks, skips the
// trustdb check and starts no agent, so it cannot block the repo-server's own
// import or delete.
func waitForKeyring(t *testing.T, present bool) {
	t.Helper()
	home := fixture.TmpDir() + "/app/config/gpg/keys"
	require.Eventually(t, func() bool {
		// EnsureCleanState leaves this directory empty, and gpg creates a keyring in an
		// empty directory. Wait for the repo-server to create it, so this check never
		// writes into the repo-server's keyring.
		if _, err := os.Stat(filepath.Join(home, "pubring.kbx")); err != nil {
			// No keyring means no key. Any other error proves nothing.
			return stderrors.Is(err, os.ErrNotExist) && !present
		}
		cmd := exec.CommandContext(t.Context(), "gpg", "--homedir", home, "--batch", "--lock-never", "--no-auto-check-trustdb", "--no-autostart", "--list-keys", fixture.GpgGoodKeyID)
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		out, err := cmd.CombinedOutput()
		if err == nil {
			return present
		}
		// Any other error, such as a read during the repo-server's rewrite, proves nothing.
		return !present && strings.Contains(string(out), "No public key")
	}, 30*time.Second, 100*time.Millisecond, "repo-server keyring did not sync key %s (want present=%v)", fixture.GpgGoodKeyID, present)
}
