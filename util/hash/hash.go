package hash

import (
	"bytes"
	"encoding/gob"
	"hash/fnv"

	"github.com/cespare/xxhash/v2"
)

// FNVa computes a FNVa hash on a string
func FNVa(s string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return h.Sum32()
}

func ObjectHash(obj any) (uint64, error) {
	var buffer bytes.Buffer

	enc := gob.NewEncoder(&buffer)
	err := enc.Encode(obj)

	if err != nil {
		return 0, err
	}

	return xxhash.Sum64(buffer.Bytes()), nil
}