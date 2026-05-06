package hash

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"hash/fnv"

	"github.com/cespare/xxhash/v2"
)

// FNVa computes a FNVa hash on a string
func FNVa(s string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return h.Sum32()
}

func GobObjectHash(obj any) (uint64, error) {
	var buffer bytes.Buffer

	enc := gob.NewEncoder(&buffer)
	err := enc.Encode(obj)
	if err != nil {
		return 0, err
	}

	return xxhash.Sum64(buffer.Bytes()), nil
}

func JsonObjectHash(obj any) (uint64, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return 0, err
	}

	return xxhash.Sum64(data), nil
}
