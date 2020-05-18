package git

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	argoio "github.com/argoproj/gitops-engine/pkg/utils/io"
	log "github.com/sirupsen/logrus"

	certutil "github.com/argoproj/argo-cd/util/cert"
)

type Creds interface {
	Environ() (io.Closer, []string, error)
}

// nop implementation
type NopCloser struct {
}

func (c NopCloser) Close() error {
	return nil
}

type NopCreds struct {
}

func (c NopCreds) Environ() (io.Closer, []string, error) {
	return NopCloser{}, nil, nil
}

// HTTPS creds implementation
type HTTPSCreds struct {
	// Username for authentication
	username string
	// Password for authentication
	password string
	// Whether to ignore invalid server certificates
	insecure bool
	// Client certificate to use
	clientCertData string
	// Client certificate key to use
	clientCertKey string
}

func NewHTTPSCreds(username string, password string, clientCertData string, clientCertKey string, insecure bool) HTTPSCreds {
	return HTTPSCreds{
		username,
		password,
		insecure,
		clientCertData,
		clientCertKey,
	}
}

// Get additional required environment variables for executing git client to
// access specific repository via HTTPS.
func (c HTTPSCreds) Environ() (io.Closer, []string, error) {
	env := []string{fmt.Sprintf("GIT_ASKPASS=%s", "git-ask-pass.sh"), fmt.Sprintf("GIT_USERNAME=%s", c.username), fmt.Sprintf("GIT_PASSWORD=%s", c.password)}
	httpCloser := authFilePaths(make([]string, 0))

	// GIT_SSL_NO_VERIFY is used to tell git not to validate the server's cert at
	// all.
	if c.insecure {
		env = append(env, "GIT_SSL_NO_VERIFY=true")
	}

	// In case the repo is configured for using a TLS client cert, we need to make
	// sure git client will use it. The certificate's key must not be password
	// protected.
	if c.clientCertData != "" && c.clientCertKey != "" {
		var certFile, keyFile *os.File

		// We need to actually create two temp files, one for storing cert data and
		// another for storing the key. If we fail to create second fail, the first
		// must be removed.
		certFile, err := ioutil.TempFile(argoio.TempDir, "")
		if err == nil {
			defer certFile.Close()
			keyFile, err = ioutil.TempFile(argoio.TempDir, "")
			if err != nil {
				removeErr := os.Remove(certFile.Name())
				if removeErr != nil {
					log.Errorf("Could not remove previously created tempfile %s: %v", certFile.Name(), removeErr)
				}
				return NopCloser{}, nil, err
			}
			defer keyFile.Close()
		} else {
			return NopCloser{}, nil, err
		}

		// We should have both temp files by now
		httpCloser = authFilePaths([]string{certFile.Name(), keyFile.Name()})

		_, err = certFile.WriteString(c.clientCertData)
		if err != nil {
			httpCloser.Close()
			return NopCloser{}, nil, err
		}
		// GIT_SSL_CERT is the full path to a client certificate to be used
		env = append(env, fmt.Sprintf("GIT_SSL_CERT=%s", certFile.Name()))

		_, err = keyFile.WriteString(c.clientCertKey)
		if err != nil {
			httpCloser.Close()
			return NopCloser{}, nil, err
		}
		// GIT_SSL_KEY is the full path to a client certificate's key to be used
		env = append(env, fmt.Sprintf("GIT_SSL_KEY=%s", keyFile.Name()))

	}
	return httpCloser, env, nil
}

// SSH implementation
type SSHCreds struct {
	sshPrivateKey string
	caPath        string
	insecure      bool
}

func NewSSHCreds(sshPrivateKey string, caPath string, insecureIgnoreHostKey bool) SSHCreds {
	return SSHCreds{sshPrivateKey, caPath, insecureIgnoreHostKey}
}

type sshPrivateKeyFile string

type authFilePaths []string

func (f sshPrivateKeyFile) Close() error {
	return os.Remove(string(f))
}

// Remove a list of files that have been created as temp files while creating
// HTTPCreds object above.
func (f authFilePaths) Close() error {
	var retErr error = nil
	for _, path := range f {
		err := os.Remove(path)
		if err != nil {
			log.Errorf("HTTPSCreds.Close(): Could not remove temp file %s: %v", path, err)
			retErr = err
		}
	}
	return retErr
}

func (c SSHCreds) Environ() (io.Closer, []string, error) {
	// use the SHM temp dir from util, more secure
	file, err := ioutil.TempFile(argoio.TempDir, "")
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	_, err = file.WriteString(c.sshPrivateKey + "\n")
	if err != nil {
		return nil, nil, err
	}

	args := []string{"ssh", "-i", file.Name()}
	var env []string
	if c.caPath != "" {
		env = append(env, fmt.Sprintf("GIT_SSL_CAINFO=%s", c.caPath))
	}
	if c.insecure {
		log.Warn("temporarily disabling strict host key checking (i.e. '-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null'), please don't use in production")
		// StrictHostKeyChecking will add the host to the knownhosts file,  we don't want that - a security issue really,
		// UserKnownHostsFile=/dev/null is therefore used so we write the new insecure host to /dev/null
		args = append(args, "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null")
	} else {
		knownHostsFile := certutil.GetSSHKnownHostsDataPath()
		args = append(args, "-o", "StrictHostKeyChecking=yes", "-o", fmt.Sprintf("UserKnownHostsFile=%s", knownHostsFile))
	}
	env = append(env, []string{fmt.Sprintf("GIT_SSH_COMMAND=%s", strings.Join(args, " "))}...)
	return sshPrivateKeyFile(file.Name()), env, nil
}
