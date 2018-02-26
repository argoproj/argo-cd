package git

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// NormalizeGitURL normalizes a git URL for lookup and storage
func NormalizeGitURL(repo string) string {
	repoURL, _ := url.Parse(repo)
	return repoURL.String()
}

// TestRepo tests if a repo exists and is accessible with the given credentials
func TestRepo(repo, username, password string) error {
	repoURL, err := url.ParseRequestURI(repo)
	if err != nil {
		return err
	}
	repoURL.User = url.UserPassword(username, password)
	cmd := exec.Command("git", "ls-remote", repoURL.String(), "HEAD")
	env := os.Environ()
	env = append(env, "GIT_ASKPASS=")
	cmd.Env = env
	_, err = cmd.Output()
	if err != nil {
		exErr := err.(*exec.ExitError)
		errOutput := strings.Split(string(exErr.Stderr), "\n")[0]
		errOutput = redactPassword(errOutput, password)
		return fmt.Errorf("failed to test %s: %s", repo, errOutput)
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
