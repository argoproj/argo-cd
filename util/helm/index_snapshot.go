package helm

// Dependency-only index snapshots. When
// --helm-index-cache-filter-enabled is set, each private build snapshot of a
// cached index keeps only the entries for the charts the chart actually
// depends on, bounded by a process-wide projection cache. The on-disk and
// shared-cache records always keep the full original: filtering never removes
// information that revalidation or another build may need.

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"strings"
	"sync"

	"github.com/golang/groupcache/lru"
	"golang.org/x/sync/singleflight"
	"sigs.k8s.io/yaml"
)

// dependencyIndexes caches parsed JSON projections of full index bodies, not
// YAML trees. One index generation can serve different dependency sets
// without reparsing the complete repository index. Failures are cached too:
// a body that cannot be projected is not re-read for every build.
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
	mu       sync.Mutex
	entries  *lru.Cache
	pending  singleflight.Group
	limit    int
	used     int
	failures map[[sha256.Size]byte]error
}

func newIndexSnapshotCache(limit int) *indexSnapshotCache {
	c := &indexSnapshotCache{entries: lru.New(128), limit: limit, failures: map[[sha256.Size]byte]error{}}
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
		if err := validateIndexBody(data); err != nil {
			return nil, fmt.Errorf("JSON projection changes Helm field types: %w", err)
		}
	}
	index := &indexProjection{}
	if err := json.Unmarshal(data, &index.metadata); err != nil {
		return nil, err
	}
	for key := range index.metadata {
		for _, field := range []string{"apiVersion", "entries", "generated", "publicKeys", "annotations", "serverInfo"} {
			if strings.EqualFold(key, field) && key != field {
				return nil, errors.New("index field casing cannot be projected safely")
			}
		}
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
	c.mu.Lock()
	err, failed := c.failures[digest]
	c.mu.Unlock()
	if failed {
		return nil, err
	}
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
			if entry.err != nil {
				c.failures[digest] = entry.err
			} else if entry.size <= c.limit {
				for c.used+entry.size > c.limit {
					c.entries.RemoveOldest()
				}
				c.entries.Add(digest, entry)
				c.used += entry.size
			}
			c.mu.Unlock()
			return entry, nil
		})
		result = value.(*indexProjectionResult)
	}
	return result.index, result.err
}

// snapshot returns a filtered index body keeping only the named charts (with
// all their versions). When every chart is selected the ORIGINAL body is
// returned unchanged — writing a filtered file that drops nothing would only
// risk a decoder divergence. The second result reports whether a NEW body was
// produced (false = reuse the input as-is).
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
			// A dependency the cached index does not know at all (added
			// upstream after the fetch) is not a snapshot bug: callers fall
			// back to the full index and revalidation picks the chart up on
			// the next pass.
			return nil, false, &indexDependencyAbsentError{name: name}
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

// dependencyNamesByRepository groups the ORIGINAL chart names (never aliases)
// of a chart's http(s) dependencies by repository identity, so every direct
// dependency that uses a repository is retained, with all versions. Aliases
// are resolved through the configured repositories; http(s)-only entries are
// collected because only they can come from an index store.
func dependencyNamesByRepository(ctx context.Context, dir string, repos []HelmRepository) (map[string]map[string]struct{}, error) {
	type dependency struct {
		Name       string
		Repository string
	}
	var chart struct {
		APIVersion   string       `yaml:"apiVersion"`
		Dependencies []dependency `yaml:"dependencies"`
	}
	data, err := ReadChartMetadata(dir, "Chart.yaml")
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(data, &chart); err != nil {
		return nil, err
	}
	data, err = ReadChartMetadata(dir, "requirements.yaml")
	if err == nil {
		if err := yaml.Unmarshal(data, &chart); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	aliases := make(map[string]string)
	for _, repo := range repos {
		aliases[repo.Name] = repo.Repo
	}
	result := make(map[string]map[string]struct{})
	for _, dep := range chart.Dependencies {
		repository := dep.Repository
		if strings.HasPrefix(repository, "@") || strings.HasPrefix(repository, "alias:") {
			name := helmRepositoryAliasName(repository)
			resolved, ok := aliases[name]
			if !ok {
				return nil, fmt.Errorf("unresolved repository alias %q", name)
			}
			repository = resolved
		}
		if !strings.HasPrefix(repository, "http://") && !strings.HasPrefix(repository, "https://") {
			continue
		}
		if dep.Name == "" {
			return nil, errors.New("dependency has no chart name")
		}
		key, err := scopedIndexIdentityContext(ctx, repository, "", "", "")
		if err != nil {
			return nil, err
		}
		if result[key] == nil {
			result[key] = make(map[string]struct{})
		}
		result[key][dep.Name] = struct{}{}
	}
	return result, nil
}

// isIndexSnapshotError classifies a dependency-build failure as an index
// lookup miss that a filtered snapshot may have caused (retryable with the
// FULL original index) rather than an origin validation, authentication or
// network failure. oci:// and file:// sources never hit the index store.
func isIndexSnapshotError(err error) bool {
	message := err.Error()
	if strings.Contains(message, "oci://") || strings.Contains(message, "file://") {
		return false
	}
	if strings.Contains(message, "error loading ") && strings.Contains(message, "-index.yaml:") {
		return true
	}
	if strings.HasSuffix(strings.TrimSpace(message), "Error: could not find : entry not found") || strings.HasSuffix(strings.TrimSpace(message), "Error: could not find : no matching version") {
		return true
	}
	for _, text := range []string{"no cached repository", "no cached repo found", "chart not found in repo", "can't get a valid version for", "no chart name found", "no chart version found"} {
		if strings.Contains(message, text) {
			return true
		}
	}
	return strings.Contains(message, "matching") && strings.Contains(message, "not found in") && strings.Contains(message, "index")
}

// indexDependencyAbsentError reports that a required dependency does not
// exist in the cached index at all (typically a chart published upstream
// after the index was fetched). Callers must fall back to the full index
// instead of failing the build.
type indexDependencyAbsentError struct {
	name string
}

func (e *indexDependencyAbsentError) Error() string {
	return fmt.Sprintf("dependency %q is absent from the index", e.name)
}

// isIndexDependencyAbsentError reports whether the error came from a
// dependency missing from the cached index (as opposed to a malformed
// projection).
func isIndexDependencyAbsentError(err error) bool {
	var absent *indexDependencyAbsentError
	return errors.As(err, &absent)
}
