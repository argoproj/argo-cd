package cert

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// defaultSystemCertDirs mirrors Go's unix certDirectories so SSL_CERT_DIR can keep system roots
// while also including a repository CA directory (Go replaces the default list when SSL_CERT_DIR is set).
var defaultSystemCertDirs = []string{
	"/etc/ssl/certs",
	"/etc/pki/tls/certs",
}

// BuildSSLCertDir returns an SSL_CERT_DIR value that includes customDir plus existing SSL_CERT_DIR
// or default system certificate directories.
func BuildSSLCertDir(customDir string) string {
	parts := []string{customDir}
	if existing := os.Getenv("SSL_CERT_DIR"); existing != "" {
		for part := range strings.SplitSeq(existing, ":") {
			if part == "" || part == customDir {
				continue
			}
			parts = append(parts, part)
		}
	} else {
		parts = append(parts, defaultSystemCertDirs...)
	}
	return strings.Join(parts, ":")
}

// PrepareSSLCertDirForRepositoryCA copies the repository CA PEM into destDir and returns the
// SSL_CERT_DIR value that includes destDir and system trust.
func PrepareSSLCertDirForRepositoryCA(caPath, destDir string) (string, error) {
	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return "", fmt.Errorf("failed to create repository CA directory: %w", err)
	}
	data, err := os.ReadFile(caPath)
	if err != nil {
		return "", fmt.Errorf("failed to read CA file %q: %w", caPath, err)
	}
	sum := sha256.Sum256([]byte(caPath))
	dest := filepath.Join(destDir, hex.EncodeToString(sum[:])+".crt")
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return "", fmt.Errorf("failed to write repository CA for SSL_CERT_DIR: %w", err)
	}
	return BuildSSLCertDir(destDir), nil
}

// UpsertEnvVars replaces or appends environment variables in KEY=VALUE form.
func UpsertEnvVars(envList []string, extras ...string) []string {
	if len(extras) == 0 {
		return envList
	}
	keys := map[string]string{}
	order := make([]string, 0, len(extras))
	for _, extra := range extras {
		key, _, ok := strings.Cut(extra, "=")
		if !ok || key == "" {
			continue
		}
		if _, seen := keys[key]; !seen {
			order = append(order, key)
		}
		keys[key] = extra
	}
	out := make([]string, 0, len(envList)+len(keys))
	for _, item := range envList {
		key, _, ok := strings.Cut(item, "=")
		if ok {
			if _, replace := keys[key]; replace {
				continue
			}
		}
		out = append(out, item)
	}
	for _, key := range order {
		out = append(out, keys[key])
	}
	return out
}
