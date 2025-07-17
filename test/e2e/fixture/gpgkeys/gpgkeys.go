package gpgkeys

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	"github.com/argoproj/argo-cd/v2/util/errors"
)

// Add GPG public key via API and create appropriate file where the ConfigMap mount would de it as well
func AddGPGPublicKey() {
	keyPath, err := filepath.Abs(fmt.Sprintf("../fixture/gpg/%s", fixture.GpgGoodKeyID))
	errors.CheckError(err)
	args := []string{"gpg", "add", "--from", keyPath}
	errors.FailOnErr(fixture.RunCli(args...))

	if fixture.IsLocal() {
		keyData, err := os.ReadFile(keyPath)
		errors.CheckError(err)
		err = os.WriteFile(fmt.Sprintf("%s/app/config/gpg/source/%s", fixture.TmpDir, fixture.GpgGoodKeyID), keyData, 0o644)
		errors.CheckError(err)
	} else {
		fixture.RestartRepoServer()
	}
}

func DeleteGPGPublicKey() {
	args := []string{"gpg", "rm", fixture.GpgGoodKeyID}
	errors.FailOnErr(fixture.RunCli(args...))
	if fixture.IsLocal() {
		errors.CheckError(os.Remove(fmt.Sprintf("%s/app/config/gpg/source/%s", fixture.TmpDir, fixture.GpgGoodKeyID)))
	} else {
		fixture.RestartRepoServer()
	}
}
