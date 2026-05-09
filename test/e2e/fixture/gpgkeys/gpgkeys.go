package gpgkeys

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

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

	// When running against local goreman processes, write the key directly to the filesystem
	// that repo-server reads from. When running against a real cluster (k3s in CI or remote),
	// restart repo-server to force it to reload the GPG keyring from the updated ConfigMap.
	if fixture.IsLocal() && !fixture.IsK3s() {
		keyData, err := os.ReadFile(keyPath)
		require.NoError(t, err)
		err = os.WriteFile(fmt.Sprintf("%s/app/config/gpg/source/%s", fixture.TmpDir(), fixture.GpgGoodKeyID), keyData, 0o644)
		require.NoError(t, err)
	} else {
		fixture.RestartRepoServer(t)
	}
}

func DeleteGPGPublicKey(t *testing.T) {
	t.Helper()
	args := []string{"gpg", "rm", fixture.GpgGoodKeyID}
	errors.NewHandler(t).FailOnErr(fixture.RunCli(args...))
	if fixture.IsLocal() && !fixture.IsK3s() {
		require.NoError(t, os.Remove(fmt.Sprintf("%s/app/config/gpg/source/%s", fixture.TmpDir(), fixture.GpgGoodKeyID)))
	} else {
		fixture.RestartRepoServer(t)
	}
}
