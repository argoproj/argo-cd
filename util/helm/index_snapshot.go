package helm

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/golang/groupcache/lru"
	"golang.org/x/sync/singleflight"
	"sigs.k8s.io/yaml"
)

// Cache compact JSON fragments, not YAML trees. One index generation can serve
// different dependency sets without reparsing the complete repository index.
var dependencyIndexes = newIndexSnapshotCache(64 << 20)

type indexProjection struct {
	metadata map[string]json.RawMessage
	entries  map[string]json.RawMessage
	size     int
}

type indexProjectionResult struct {
	index *indexProjection
	err   error
	size  int
}

type indexSnapshotCache struct {
	mu      sync.Mutex
	entries *lru.Cache
	pending singleflight.Group
	limit   int
	used    int
}

func newIndexSnapshotCache(limit int) *indexSnapshotCache {
	c := &indexSnapshotCache{entries: lru.New(128), limit: limit}
	c.entries.OnEvicted = func(_ lru.Key, value any) { c.used -= value.(*indexProjectionResult).size }
	return c
}

func (c *indexSnapshotCache) get(digest [sha256.Size]byte) *indexProjectionResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	if entry, ok := c.entries.Get(digest); ok {
		return entry.(*indexProjectionResult)
	}
	return nil
}

func parseIndexProjection(data []byte) (*indexProjection, error) {
	if !json.Valid(data) {
		var err error
		data, err = yaml.YAMLToJSONStrict(data)
		if err != nil {
			return nil, err
		}
	}
	index := &indexProjection{}
	if err := json.Unmarshal(data, &index.metadata); err != nil {
		return nil, err
	}
	if string(index.metadata["apiVersion"]) != `"v1"` {
		return nil, errors.New("unsupported index projection version")
	}
	if err := json.Unmarshal(index.metadata["entries"], &index.entries); err != nil {
		return nil, err
	}
	delete(index.metadata, "entries")
	for key, value := range index.metadata {
		index.size += len(key) + len(value) + 64
	}
	for key, value := range index.entries {
		index.size += len(key) + len(value) + 64
	}
	return index, nil
}

func (c *indexSnapshotCache) projection(data []byte) (*indexProjection, error) {
	if len(data) > c.limit {
		return nil, errors.New("index exceeds snapshot cache budget")
	}
	digest := sha256.Sum256(data)
	result := c.get(digest)
	if result == nil {
		value, _, _ := c.pending.Do(string(digest[:]), func() (any, error) {
			if cached := c.get(digest); cached != nil {
				return cached, nil
			}
			index, err := parseIndexProjection(data)
			entry := &indexProjectionResult{index: index, err: err, size: 128}
			if err == nil {
				entry.size += index.size
			}
			if entry.size > c.limit {
				entry = &indexProjectionResult{err: errors.New("projected index exceeds snapshot cache budget"), size: 128}
			}
			c.mu.Lock()
			defer c.mu.Unlock()
			if entry.size <= c.limit {
				for c.used+entry.size > c.limit {
					c.entries.RemoveOldest()
				}
				c.entries.Add(digest, entry)
				c.used += entry.size
			}
			return entry, nil
		})
		result = value.(*indexProjectionResult)
	}
	return result.index, result.err
}

func (c *indexSnapshotCache) snapshot(data []byte, names map[string]struct{}) ([]byte, bool, error) {
	if len(names) == 0 {
		return data, false, nil
	}
	index, err := c.projection(data)
	if err != nil {
		return nil, false, err
	}
	selected := make(map[string]json.RawMessage, len(names))
	for name := range names {
		entry, ok := index.entries[name]
		if !ok {
			return nil, false, fmt.Errorf("dependency %q is absent from the index", name)
		}
		selected[name] = entry
	}
	if len(selected) == len(index.entries) {
		return data, false, nil
	}
	entries, err := json.Marshal(selected)
	if err != nil {
		return nil, false, err
	}
	metadata := make(map[string]json.RawMessage, len(index.metadata)+1)
	maps.Copy(metadata, index.metadata)
	metadata["entries"] = entries
	snapshot, err := json.Marshal(metadata)
	return snapshot, err == nil, err
}

// dependencyNamesByRepository uses original chart names, never chart aliases.
// Repository aliases and URL variants are grouped before building snapshots so
// every direct dependency using a repository is retained, with all versions.
func dependencyNamesByRepository(dir string, repos []HelmRepository) (map[string]map[string]struct{}, error) {
	type dependency struct{ Name, Repository string }
	var chart struct {
		APIVersion   string
		Dependencies []dependency
	}
	data, err := os.ReadFile(filepath.Join(dir, "Chart.yaml"))
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(data, &chart); err != nil {
		return nil, err
	}
	if chart.APIVersion == "v1" {
		data, err := os.ReadFile(filepath.Join(dir, "requirements.yaml"))
		if err == nil {
			chart.Dependencies = nil
			if err := yaml.Unmarshal(data, &chart); err != nil {
				return nil, err
			}
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	aliases := make(map[string]string)
	for _, repo := range repos {
		aliases[repo.Name] = repo.Repo
	}
	result := make(map[string]map[string]struct{})
	for _, dep := range chart.Dependencies {
		repository := dep.Repository
		if strings.HasPrefix(repository, "@") || strings.HasPrefix(repository, "alias:") {
			name := strings.TrimPrefix(strings.TrimPrefix(repository, "@"), "alias:")
			var ok bool
			repository, ok = aliases[name]
			if !ok {
				return nil, fmt.Errorf("unresolved repository alias %q", name)
			}
		}
		if !strings.HasPrefix(repository, "http://") && !strings.HasPrefix(repository, "https://") {
			continue
		}
		if dep.Name == "" {
			return nil, errors.New("dependency has no chart name")
		}
		key := scopedIndexIdentity(repository, "", "", "")
		if result[key] == nil {
			result[key] = make(map[string]struct{})
		}
		result[key][dep.Name] = struct{}{}
	}
	return result, nil
}

func isIndexSnapshotError(err error) bool {
	message := err.Error()
	if strings.Contains(message, "oci://") || strings.Contains(message, "file://") {
		return false
	}
	for _, text := range []string{"no cached repository", "no cached repo found", "chart not found in repo", "can't get a valid version for", "no chart name found", "no chart version found"} {
		if strings.Contains(message, text) {
			return true
		}
	}
	return strings.Contains(message, "matching") && strings.Contains(message, "not found in") && strings.Contains(message, "index")
}
