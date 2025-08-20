package git

import (
	"crypto/fips140"
	"fmt"

	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"golang.org/x/crypto/ssh"
)

// SupportedSSHKeyExchangeAlgorithms is a list of all currently supported algorithms for SSH key exchange
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

// SupportedFIPSCompliantSSHKeyExchangeAlgorithms is a list of all currently supported algorithms for SSH key exchange
// that are FIPS compliant
var SupportedFIPSCompliantSSHKeyExchangeAlgorithms = []string{
	"ecdh-sha2-nistp256",
	"ecdh-sha2-nistp384",
	"ecdh-sha2-nistp521",
	"diffie-hellman-group-exchange-sha256",
	"diffie-hellman-group14-sha256",
}

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

// getDefaultSSHKeyExchangeAlgorithms returns the default key exchange algorithms to be used
func getDefaultSSHKeyExchangeAlgorithms() []string {
	if fips140.Enabled() {
		return SupportedFIPSCompliantSSHKeyExchangeAlgorithms
	}
	return SupportedSSHKeyExchangeAlgorithms
}
