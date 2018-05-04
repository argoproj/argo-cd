package git

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// EnsureSuffix idempotently ensures that a base string has a given suffix.
func ensureSuffix(s, suffix string) string {
	if !strings.HasSuffix(s, suffix) {
		s += suffix
	}
	return s
}

// NormalizeGitURL normalizes a git URL for lookup and storage
func NormalizeGitURL(repo string) string {
	repoURL, err := url.Parse(repo)
	if err != nil {
		return strings.ToLower(repo)
	}
	return repoURL.String()
}

// IsSshURL returns true is supplied URL is SSH URL
func IsSshURL(url string) bool {
	// TODO: should probably support ssh:// scheme too, since that's a valid SSH URL
	return strings.HasPrefix(url, "git@")
}

// GetGitCommandOptions returns URL and env options for git operation
func GetGitCommandEnvAndURL(repo, username, password string, sshPrivateKey string) (string, []string, error) {
	cmdURL := repo
	env := os.Environ()
	if IsSshURL(repo) {
		if sshPrivateKey != "" {
			sshFile, err := ioutil.TempFile("", "")
			if err != nil {
				return "", nil, err
			}
			_, err = sshFile.WriteString(sshPrivateKey)
			if err != nil {
				return "", nil, err
			}
			err = sshFile.Close()
			if err != nil {
				return "", nil, err
			}
			env = append(env, fmt.Sprintf("GIT_SSH_COMMAND=ssh -o StrictHostKeyChecking=no -i %s", sshFile.Name()))
		}
	} else {
		env = append(env, "GIT_ASKPASS=")
		repoURL, err := url.ParseRequestURI(repo)
		if err != nil {
			return "", nil, err
		}

		repoURL.User = url.UserPassword(username, password)
		cmdURL = repoURL.String()
	}
	return cmdURL, env, nil
}

// TestRepo tests if a repo exists and is accessible with the given credentials
func TestRepo(repo, username, password string, sshPrivateKey string) error {
	repo, env, err := GetGitCommandEnvAndURL(repo, username, password, sshPrivateKey)
	if err != nil {
		return err
	}
	cmd := exec.Command("git", "ls-remote", repo, "HEAD")

	cmd.Env = env
	_, err = cmd.Output()
	if err != nil {
		exErr := err.(*exec.ExitError)
		errOutput := strings.Split(string(exErr.Stderr), "\n")[0]
		errOutput = redactPassword(errOutput, password)
		return fmt.Errorf("%s: %s", repo, errOutput)
	}
	return nil
}

func redactPassword(msg string, password string) string {
	if password != "" {
		passwordRegexp := regexp.MustCompile("\\b" + regexp.QuoteMeta(password) + "\\b")
		msg = passwordRegexp.ReplaceAllString(msg, "*****")
	}
	return msg
}
