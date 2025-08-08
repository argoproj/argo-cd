package git

import (
	"fmt"
	"os"

	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// List of all currently supported algorithms for SSH key exchange
// Unfortunately, crypto/ssh does not offer public constants or list for
// this.
var SupportedSSHKeyExchangeAlgorithms = []string{
	"curve25519-sha256",
	"curve25519-sha256@libssh.org",
	"ecdh-sha2-nistp256",
	"ecdh-sha2-nistp384",
	"ecdh-sha2-nistp521",
	"diffie-hellman-group-exchange-sha256",
	"diffie-hellman-group14-sha256",
	"diffie-hellman-group14-sha1",
}

var SupportedFIPSCompliantSSHKeyExchangeAlgorithms = []string{
	"ecdh-sha2-nistp256",
	"ecdh-sha2-nistp384",
	"ecdh-sha2-nistp521",
	"diffie-hellman-group-exchange-sha256",
	"diffie-hellman-group14-sha256",
	"diffie-hellman-group14-sha1",
}

// List of default key exchange algorithms to use. We use those that are
// available by default, we can become more opinionated later on (when
// we support configuration of algorithms to use).
var DefaultSSHKeyExchangeAlgorithms = SupportedSSHKeyExchangeAlgorithms

// PublicKeysWithOptions is an auth method for go-git's SSH client that
// inherits from PublicKeys, but provides the possibility to override
// some client options.
type PublicKeysWithOptions struct {
	KexAlgorithms []string
	gitssh.PublicKeys
}

// Name returns the name of the auth method
func (a *PublicKeysWithOptions) Name() string {
	return gitssh.PublicKeysName
}

// String returns the configured user and auth method name as string
func (a *PublicKeysWithOptions) String() string {
	return fmt.Sprintf("user: %s, name: %s", a.User, a.Name())
}

// ClientConfig returns a custom SSH client configuration
func (a *PublicKeysWithOptions) ClientConfig() (*ssh.ClientConfig, error) {
	// Algorithms used for kex can be configured
	var kexAlgos []string
	if len(a.KexAlgorithms) > 0 {
		kexAlgos = a.KexAlgorithms
	} else {
		kexAlgos = getDefaultSSHKeyExchangeAlgorithms()
	}
	config := ssh.Config{KeyExchanges: kexAlgos}
	opts := &ssh.ClientConfig{Config: config, User: a.User, Auth: []ssh.AuthMethod{ssh.PublicKeys(a.Signer)}}
	return a.SetHostKeyCallback(opts)
}

// getDefaultSSHKeyExchangeAlgorithms returns the default key exchange algorthim to be used
func getDefaultSSHKeyExchangeAlgorithms() []string {
	if isFipsMode, err := isHostRunningInFips(); isFipsMode {
		if err != nil {
			log.Warnf("error checking if host running in FIPS mode: %v", err)
		}
		return SupportedFIPSCompliantSSHKeyExchangeAlgorithms
	}
	return SupportedSSHKeyExchangeAlgorithms
}

// isHostRunningInFips returns true if the system in which the binary is running in
func isHostRunningInFips() (bool, error) {
	const procSysFipsEnabledPath = "/proc/sys/crypto/fips_enabled"
	b, err := os.ReadFile(procSysFipsEnabledPath)
	if err != nil {
		return false, err
	}
	return b[0] == '1', nil
}
