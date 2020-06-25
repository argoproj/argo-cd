package gpgkeys

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/argoproj/gitops-engine/pkg/utils/errors"

	"github.com/argoproj/argo-cd/test/e2e/fixture"
)

// Add GPG public key via API and create appropriate file where the ConfigMap mount would de it as well
func AddGPGPublicKey() {
	keyPath, err := filepath.Abs(fmt.Sprintf("../fixture/gpg/%s", fixture.GpgGoodKeyID))
	errors.CheckErrorWithCode(err, errors.ErrorCommandSpecific)
	args := []string{"gpg", "add", "--from", keyPath}
	errors.FailOnErr(fixture.RunCli(args...))

	keyData, err := ioutil.ReadFile(keyPath)
	errors.CheckErrorWithCode(err, errors.ErrorCommandSpecific)
	err = ioutil.WriteFile(fmt.Sprintf("%s/app/config/gpg/source/%s", fixture.TmpDir, fixture.GpgGoodKeyID), keyData, 0644)
	errors.CheckErrorWithCode(err, errors.ErrorCommandSpecific)
}

func DeleteGPGPublicKey() {
	args := []string{"gpg", "rm", fixture.GpgGoodKeyID}
	errors.FailOnErr(fixture.RunCli(args...))
	errors.CheckErrorWithCode(os.Remove(fmt.Sprintf("%s/app/config/gpg/source/%s", fixture.TmpDir, fixture.GpgGoodKeyID)), errors.ErrorCommandSpecific)
	os.Exit(
}
