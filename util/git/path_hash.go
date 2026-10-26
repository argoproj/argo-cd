package git

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"sort"
	"strings"
)

// ComputePathHash generates a deterministic hash from a list of sparse-checkout
// paths. Inputs are normalized first (see normalizeSparsePaths) so that a
// user-supplied slice like ["app-a/"] hashes to the same value as git's
// canonical form ["app-a"] returned by `git sparse-checkout list`. Without this
// normalization, recovering an existing on-disk workdir at Init time would
// produce a different hash than the runtime lookup and orphan the directory.
// Returns the first 16 hex characters of the SHA256 hash.
func ComputePathHash(paths []string) string {
	normalized := normalizeSparsePaths(paths)
	if len(normalized) == 0 {
		return ""
	}

	combined := strings.Join(normalized, ",")
	hash := sha256.Sum256([]byte(combined))

	// 16 hex chars = 8 bytes = 2^64 distinct values, ample for this use.
	return hex.EncodeToString(hash[:])[:16]
}

// NormalizeSparsePaths validates cone-mode sparse paths and returns their
// canonical form. Entries git would reject (escaping the repo, globs, flags)
// make `git sparse-checkout set` fail, so they are rejected up front.
func NormalizeSparsePaths(paths []string) ([]string, error) {
	for _, p := range paths {
		cleaned := path.Clean(strings.TrimSpace(p))
		switch {
		case strings.HasPrefix(p, "-"):
			return nil, fmt.Errorf("invalid sparse path %q: must not start with '-'", p)
		case cleaned == ".." || strings.HasPrefix(cleaned, "../"):
			return nil, fmt.Errorf("invalid sparse path %q: must not reference a parent directory", p)
		case strings.ContainsAny(p, "*?[\\"):
			return nil, fmt.Errorf("invalid sparse path %q: must be a directory, not a pattern", p)
		}
	}
	normalized := normalizeSparsePaths(paths)
	if len(paths) > 0 && len(normalized) == 0 {
		return nil, fmt.Errorf("invalid sparse paths %q: no directory given", paths)
	}
	return normalized, nil
}

// normalizeSparsePaths returns a canonical, hash-stable form of the input:
//   - lexically cleans each entry (path.Clean), so "./charts", "charts/./sub",
//     and "charts//sub" hash identically to git's canonical "charts" /
//     "charts/sub" as printed by `git sparse-checkout list`
//   - strips leading and trailing '/' (git cone-mode strips trailing slashes
//     and rejects leading slashes; stripping both lets a user's "/charts" or
//     "charts/" hash identically to git's stored "charts")
//   - drops empty and "." entries (post-clean)
//   - removes exact duplicates
//   - sorts alphabetically
func normalizeSparsePaths(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(paths))
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		p = strings.Trim(path.Clean(p), "/")
		if p == "" || p == "." {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
