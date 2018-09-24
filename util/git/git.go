package git

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// EnsurePrefix idempotently ensures that a base string has a given prefix.
func ensurePrefix(s, prefix string) string {
	if !strings.HasPrefix(s, prefix) {
		s = prefix + s
	}
	return s
}

// EnsureSuffix idempotently ensures that a base string has a given suffix.
func ensureSuffix(s, suffix string) string {
	if !strings.HasSuffix(s, suffix) {
		s += suffix
	}
	return s
}

var commitSHARegex = regexp.MustCompile("^[0-9A-Fa-f]{40}$")

// IsCommitSHA returns whether or not a string is a 40 character SHA-1
func IsCommitSHA(sha string) bool {
	return commitSHARegex.MatchString(sha)
}

var truncatedCommitSHARegex = regexp.MustCompile("^[0-9A-Fa-f]{7,}$")

// IsTruncatedCommitSHA returns whether or not a string is a truncated  SHA-1
func IsTruncatedCommitSHA(sha string) bool {
	return truncatedCommitSHARegex.MatchString(sha)
}

// NormalizeGitURL normalizes a git URL for lookup and storage
func NormalizeGitURL(repo string) string {
	// preprocess
	repo = strings.TrimSpace(repo)
	repo = ensureSuffix(repo, ".git")
	if IsSSHURL(repo) {
		repo = ensurePrefix(repo, "ssh://")
	}

	// process
	repoURL, err := url.Parse(repo)
	if err != nil {
		return ""
	}

	// postprocess
	repoURL.Host = strings.ToLower(repoURL.Host)
	normalized := repoURL.String()
	return strings.TrimPrefix(normalized, "ssh://")
}

// IsSSHURL returns true if supplied URL is SSH URL
func IsSSHURL(url string) bool {
	return strings.HasPrefix(url, "git@") || strings.HasPrefix(url, "ssh://")
}

const gitSSHCommand = "ssh -q -F /dev/null -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=20"

//TODO: Make sure every public method works with '*' repo

// GetGitCommandEnvAndURL returns URL and env options for git operation
func GetGitCommandEnvAndURL(repo, username, password string, sshPrivateKey string) (string, []string, error) {
	cmdURL := repo
	env := os.Environ()
	if IsSSHURL(repo) {
		sshCmd := gitSSHCommand
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
			sshCmd += " -i " + sshFile.Name()
		}
		env = append(env, fmt.Sprintf("GIT_SSH_COMMAND=%s", sshCmd))
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
		if exErr, ok := err.(*exec.ExitError); ok {
			errOutput := strings.Split(string(exErr.Stderr), "\n")[0]
			errOutput = fmt.Sprintf("%s: %s", repo, errOutput)
			return errors.New(redactPassword(errOutput, password))
		}
		return err
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
