package helm

// PR2 disk-slice helpers for running `helm dependency build` against a
// prewarmed private Helm home. The pieces here mirror the pinned Helm
// binary's cache layout:
//   - repositories.yaml at <helmHome>/config/helm/repositories.yaml
//     (helmpath.ConfigPath — no "repository" subdirectory; that one belongs
//     to the cache home),
//   - prewarmed indexes at <helmHome>/cache/helm/repository/<name>-index.yaml
//     (helmpath.CacheIndexFile),
//   - registry config at <helmHome>/config/helm/registry/config.json.
//
// Helm resolves a repository's cached index under EITHER of two names: the
// registered repository name (URL matched against repositories.yaml) or the
// dependency manager's generated name "helm-manager-<sha256(url)>" (URL
// referenced directly from Chart.yaml). installIndexes installs under both.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"time"

	"sigs.k8s.io/yaml"
)

// helmRepoEntry mirrors helm's repo.Entry JSON schema (pkg/repo/chartrepo.go):
// it is what helm writes to repositories.yaml on `helm repo add` and what the
// dependency downloader reads credentials from when pulling chart tarballs.
type helmRepoEntry struct {
	Name                  string `json:"name"`
	URL                   string `json:"url"`
	Username              string `json:"username,omitempty"`
	Password              string `json:"password,omitempty"`
	CertFile              string `json:"certFile,omitempty"`
	KeyFile               string `json:"keyFile,omitempty"`
	CAFile                string `json:"caFile,omitempty"`
	InsecureSkipTLSverify bool   `json:"insecure_skip_tls_verify,omitempty"`
	PassCredentialsAll    bool   `json:"pass_credentials_all,omitempty"`
}

// managerKeyPrefix mirrors helm's downloader.managerKeyPrefix: the generated
// repository cache name for unmanaged repositories.
const managerKeyPrefix = "helm-manager-"

// managerKey mirrors helm's downloader.key: a hex sha256 of the repository
// URL, used in the generated "helm-manager-<key>" cache name.
func managerKey(name string) string {
	sum := sha256.Sum256([]byte(name))
	return hex.EncodeToString(sum[:])
}

// prewarmIndexes installs a repository's cached index into the ephemeral helm
// cache home under BOTH names helm resolves indexes by: the registered
// repository name (used when the URL matches a repositories.yaml entry) and
// the dependency manager's generated name "helm-manager-<sha256(url)>" (used
// when a Chart.yaml dependency references the URL directly). Callers must
// register both names in repositories.yaml (see writeRepoEntries callers).
func (c *Cmd) prewarmIndexes(repoURL string, indexData []byte, repoName string) error {
	return c.installIndexes(repoURL, indexData, repoName, false)
}

func (c *Cmd) installIndexes(repoURL string, indexData []byte, repoName string, replace bool) error {
	destDir := path.Join(c.helmHome, "cache", "helm", "repository")
	if err := c.persistentIndexStore.copyTo(repoURL, indexData, destDir, repoName, replace); err != nil {
		return err
	}
	managerName := managerKeyPrefix + managerKey(repoURL)
	if repoName == managerName {
		return nil
	}
	return c.persistentIndexStore.copyTo(repoURL, indexData, destDir, managerName, replace)
}

// Repository client certificates and keys live for the entire subprocess
// lifetime and are removed with the private Helm home by Close.
func (c *Cmd) writeRepoCredential(data []byte) (string, error) {
	if len(data) == 0 {
		return "", nil
	}
	dir := path.Join(c.helmHome, "config")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(dir, "repo-credential-*")
	if err != nil {
		return "", err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	return file.Name(), nil
}

// repoFileEntries is the subset of helm's repositories.yaml schema needed to
// enumerate configured repositories (name + URL).
type repoFileEntries struct {
	APIVersion   string          `json:"apiVersion,omitempty"`
	Generated    string          `json:"generated,omitempty"`
	Repositories []helmRepoEntry `json:"repositories"`
}

// writeRepoEntries writes the given repository entries into the ephemeral
// helm config home's repositories.yaml, equivalent to a `helm repo add` per
// entry that skips the index download. The file lives at
// <XDG_CONFIG_HOME>/helm/repositories.yaml (helmpath.ConfigPath, no
// "repository" subdirectory — that one belongs to the cache home).
func (c *Cmd) writeRepoEntries(entries []helmRepoEntry) error {
	configRepoDir := path.Join(c.helmHome, "config", "helm")
	repoFile := path.Join(configRepoDir, "repositories.yaml")

	rf := repoFileEntries{
		APIVersion: "v1", // helm's repo.File default APIVersion
		Generated:  time.Now().UTC().Format(time.RFC3339),
	}
	if data, err := os.ReadFile(repoFile); err == nil {
		if err := yaml.Unmarshal(data, &rf); err != nil {
			return fmt.Errorf("failed to parse helm repositories.yaml: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read helm repositories.yaml: %w", err)
	}

	known := make(map[string]helmRepoEntry, len(rf.Repositories))
	for _, entry := range rf.Repositories {
		known[entry.Name] = entry
	}
	for _, entry := range entries {
		if existing, ok := known[entry.Name]; ok {
			if existing.URL != entry.URL {
				return fmt.Errorf("repository name %s already registered with a different URL", entry.Name)
			}
			if existing != entry {
				return fmt.Errorf("repository name %s has conflicting credentials or TLS settings", entry.Name)
			}
			continue // already registered
		}
		rf.Repositories = append(rf.Repositories, entry)
		known[entry.Name] = entry
	}

	if err := os.MkdirAll(configRepoDir, 0o700); err != nil {
		return fmt.Errorf("failed to create helm config dir: %w", err)
	}
	out, err := yaml.Marshal(rf)
	if err != nil {
		return fmt.Errorf("failed to serialize helm repositories.yaml: %w", err)
	}
	if err := os.WriteFile(repoFile, out, 0o600); err != nil {
		return fmt.Errorf("failed to write helm repositories.yaml: %w", err)
	}
	return nil
}
