package adminsettings

import (
	"fmt"
	"syscall"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/util/password"
	"github.com/argoproj/argo-cd/util/session"
	"github.com/argoproj/argo-cd/util/settings"
	tlsutil "github.com/argoproj/argo-cd/util/tls"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
)

func readAndConfirmPassword() string {
	for {
		fmt.Print("*** Enter an admin password: ")
		password, err := terminal.ReadPassword(syscall.Stdin)
		errors.CheckError(err)
		fmt.Print("\n")
		fmt.Print("*** Confirm the admin password: ")
		confirmPassword, err := terminal.ReadPassword(syscall.Stdin)
		errors.CheckError(err)
		fmt.Print("\n")
		if string(password) == string(confirmPassword) {
			return string(password)
		}
		logrus.Error("Passwords do not match")
	}
}

func UpdateSettings(defaultPassword string, cdSettings *settings.ArgoCDSettings, UpdateSignature bool, UpdateSuperuser bool, Namespace string) *settings.ArgoCDSettings {

	if cdSettings.ServerSignature == nil || UpdateSignature {
		// set JWT signature
		signature, err := session.MakeSignature(32)
		errors.CheckError(err)
		cdSettings.ServerSignature = signature
	}

	if cdSettings.LocalUsers == nil {
		cdSettings.LocalUsers = make(map[string]string)
	}
	if _, ok := cdSettings.LocalUsers[common.ArgoCDAdminUsername]; !ok || UpdateSuperuser {
		passwordRaw := defaultPassword
		if passwordRaw == "" {
			passwordRaw = readAndConfirmPassword()
		}
		log.Infof("password set to %s", passwordRaw)
		hashedPassword, err := password.HashPassword(passwordRaw)
		errors.CheckError(err)
		cdSettings.LocalUsers = map[string]string{
			common.ArgoCDAdminUsername: hashedPassword,
		}
	}

	if cdSettings.Certificate == nil {
		// generate TLS cert
		hosts := []string{
			"localhost",
			"argocd-server",
			fmt.Sprintf("argocd-server.%s", Namespace),
			fmt.Sprintf("argocd-server.%s.svc", Namespace),
			fmt.Sprintf("argocd-server.%s.svc.cluster.local", Namespace),
		}
		certOpts := tlsutil.CertOptions{
			Hosts:        hosts,
			Organization: "Argo CD",
			IsCA:         true,
		}
		cert, err := tlsutil.GenerateX509KeyPair(certOpts)
		errors.CheckError(err)
		cdSettings.Certificate = cert
	}

	return cdSettings
}
