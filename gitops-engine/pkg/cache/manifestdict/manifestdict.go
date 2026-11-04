// Package manifestdict holds the sample manifests used to build the zstd dictionaries for
// compressed cluster-cache manifests. See README.md for how the sample is built and maintained.
package manifestdict

import _ "embed"

// Source is the dictionary sample: one JSON object per line. Each line is serialized with the
// cache's own serializers to produce the dictionary for that byte format, so its key order and
// punctuation match the manifests the cache stores.
//
//go:embed dictionary.jsonl
var Source []byte
