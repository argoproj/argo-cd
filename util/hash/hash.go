package hash

import (
	"hash/fnv"
)

// FNVa computes a FNVa hash on a string
func FNVa(s string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return h.Sum32()
}
