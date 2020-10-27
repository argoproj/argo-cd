package git

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	argoio "github.com/argoproj/gitops-engine/pkg/utils/io"
	log "github.com/sirupsen/logrus"

	certutil "github.com/argoproj/argo-cd/util/cert"
	"github.com/argoproj/argo-cd/util/github"
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

// GitHubAppCreds to authenticate as GitHub application
type GitHubAppCreds struct {
	appID       string
	privateKey  string
	baseURL     string
	accessToken string
	repoURL     string
}

// NewGitHubAppCreds provide github app credentials
func NewGitHubAppCreds(appID string, privateKey string, baseURL string, repoURL string) GitHubAppCreds {
	return GitHubAppCreds{appID: appID, privateKey: privateKey, baseURL: baseURL, repoURL: repoURL}
}

func (g GitHubAppCreds) Environ() (io.Closer, []string, error) {
	// NOTE: this function is untested; it is sort-of pseudo code but compiles

	// if this custom token logic doesn't work, we could try to pull in something like go-github
	// however, it would be neat to avoid that as it should be possible to create a sane process without
	// pulling in thousands of lines of code tht this project does not need at all
	// Furthermore, the project already has a dependency that allows for working with tokens
	// github.com/dgrijalva/jwt-go

	// TODO allow the UI to create github app creds
	baseURL := github.BaseURL(g.baseURL)

	owner, repo := github.OwnerAndRepoName(g.repoURL)
	if owner == "" || repo == "" {
		return nil, nil, errors.new("Cannot extract owner and/or repo from repository URL")
	}

	ownerRepo := fmt.Sprintf("%s/%s", owner, repo)

	// TODO cache installation id, access token under ownerRepo key
	// Potentially we need to cache installation response etag
	// to see if the installation id has not changed.

	bearer, err := github.Bearer(g.appID, g.privateKey)
	if err != nil {
		return nil, nil, err
	}

	authorization := fmt.Sprintf("Bearer %s", bearer)
	accept := "application/vnd.github.v3+json"

	url := fmt.Sprintf("%s/repos/%s/%s/installation", baseURL, owner, repo)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Add("Authorization", authorization)
	req.Header.Add("Accept", accept)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}

	defer resp.Body.Close()

	// TODO handle response code

	var installation github.Installation
	json.NewDecoder(resp.body).decode(&installation)

	// TODO check installation permissions for to contents and metadata READ

	// TODO Cleanup http request code and decoding JSON

	url := fmt.Sprintf("%s/app/installations/%d/access_tokens", baseURL, installation.ID)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Add("Authorization", authorization)
	req.Header.Add("Accept", accept)
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}

	defer resp.Body.Close()

	// TODO handle response code

	var access github.InstallationAccessToken
	json.NewDecoder(resp.body).decode(&access)

	g.accessToken = access.Token
	// TODO cache the token and implement refresh.
	// There is no refresh option for an installation access token.
	// You simply request another token.

	// TODO potentially implement similar things to HTTPCreds environ

	return nil, nil, nil
}
