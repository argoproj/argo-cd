package gpgkeys

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/argoproj/argo-cd/test/e2e/fixture"
	"github.com/argoproj/argo-cd/util/errors"
)

// Add GPG public key via API and create appropriate file where the ConfigMap mount would de it as well
func AddGPGPublicKey() {
	keyPath, err := filepath.Abs(fmt.Sprintf("../fixture/gpg/%s", fixture.GpgGoodKeyID))
	errors.CheckError(err)
	args := []string{"gpg", "add", "--from", keyPath}
	errors.FailOnErr(fixture.RunCli(args...))

	keyData, err := ioutil.ReadFile(keyPath)
	errors.CheckError(err)
	err = ioutil.WriteFile(fmt.Sprintf("%s/app/config/gpg/source/%s", fixture.TmpDir, fixture.GpgGoodKeyID), keyData, 0644)
	errors.CheckError(err)
}

func DeleteGPGPublicKey() {
	args := []string{"gpg", "rm", fixture.GpgGoodKeyID}
	errors.FailOnErr(fixture.RunCli(args...))

	errors.CheckError(os.Remove(fmt.Sprintf("%s/app/config/gpg/source/%s", fixture.TmpDir, fixture.GpgGoodKeyID)))
}
