package oci

import "strings"

const Prefix = "oci://"

func HasOCIPrefix(repoURL string) bool {
	return strings.HasPrefix(repoURL, Prefix)
}
