package versions

import (
	digest "github.com/opencontainers/go-digest"
)

// IsDigest checks if the provided revision string is a valid SHA256 digest.
// It returns true if the revision is a valid digest with SHA256 algorithm,
// and false otherwise.
//
// In OCI (Open Container Initiative) repositories, content is often referenced by
// digest rather than by tag to ensure immutability. A valid digest has the format:
// "sha256:abcdef1234567890..." where the part after the colon is a hexadecimal string.
//
// This function performs two validations:
// 1. Checks if the string can be parsed as a digest (correct format)
// 2. Verifies that the algorithm is specifically SHA256 (not other hash algorithms)
func IsDigest(revision string) bool {
	d, err := digest.Parse(revision)
	if err != nil {
		return false
	}

	return d.Validate() == nil && d.Algorithm() == digest.SHA256
}
