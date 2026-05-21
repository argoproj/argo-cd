package git

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// ComputePathHash generates a deterministic hash from a list of paths
// The paths are sorted before hashing to ensure consistency regardless of input order
// Returns the first 16 characters of the SHA256 hash
func ComputePathHash(paths []string) string {
	if len(paths) == 0 {
		return ""
	}

	// Sort paths to ensure deterministic hash
	sortedPaths := make([]string, len(paths))
	copy(sortedPaths, paths)
	sort.Strings(sortedPaths)

	combined := strings.Join(sortedPaths, ",")
	hash := sha256.Sum256([]byte(combined))

	// Return first 16 characters (8 bytes) of hex-encoded hash
	// This provides 2^64 possible values, which is more than sufficient
	return hex.EncodeToString(hash[:])[:16]
}
