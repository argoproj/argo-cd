package cache

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	jsoniter "github.com/json-iterator/go"
	"github.com/klauspost/compress/s2"
	"github.com/vmihailenco/msgpack/v5"
)

// ManifestCompressionType defines the compression algorithm for cached manifests.
type ManifestCompressionType string

// ManifestStorageType defines the serialization format for cached manifests.
type ManifestStorageType string

const (
	ManifestCompressionGZip           ManifestCompressionType = "gzip"
	ManifestCompressionGZipDefault    ManifestCompressionType = "gzip-default"
	ManifestCompressionGZipBestSpeed  ManifestCompressionType = "gzip-bestspeed"
	ManifestCompressionS2Encode       ManifestCompressionType = "s2-encode"
	ManifestCompressionS2EncodeBetter ManifestCompressionType = "s2-encodebetter"
	ManifestCompressionZLib           ManifestCompressionType = "zlib"
	ManifestCompressionNone           ManifestCompressionType = "none"

	ManifestStorageJSON     ManifestStorageType = "json"
	ManifestStorageJSONIter ManifestStorageType = "jsoniter"
	ManifestStorageMsgPack  ManifestStorageType = "msgpack"
)

// manifestSerializer handles serialization and deserialization of manifest objects.
type manifestSerializer interface {
	marshal(obj map[string]any) ([]byte, error)
	unmarshal(data []byte) (map[string]any, error)
}

// manifestCompressor handles compression and decompression of manifest bytes.
type manifestCompressor interface {
	compress(data []byte) ([]byte, error)
	decompress(data []byte) ([]byte, error)
}

// serializers maps each ManifestStorageType to its codec implementation.
var serializers = map[ManifestStorageType]manifestSerializer{
	ManifestStorageJSON:     jsonSerializer{},
	ManifestStorageJSONIter: jsoniterSerializer{},
	ManifestStorageMsgPack:  msgpackSerializer{},
}

// compressors maps each ManifestCompressionType to its codec implementation.
var compressors = map[ManifestCompressionType]manifestCompressor{
	ManifestCompressionGZipDefault:    &gzipCompressor{level: gzip.DefaultCompression},
	ManifestCompressionGZipBestSpeed:  &gzipCompressor{level: gzip.BestSpeed},
	ManifestCompressionS2Encode:       s2Compressor{better: false},
	ManifestCompressionS2EncodeBetter: s2Compressor{better: true},
	ManifestCompressionZLib:           zlibCompressor{},
	ManifestCompressionNone:           noneCompressor{},
}

// normalizeManifestCompressionType normalizes a compression type to a known value.
// "gzip" is an alias for "gzip-bestspeed"; unknown values default to "gzip-bestspeed".
func normalizeManifestCompressionType(ct ManifestCompressionType) ManifestCompressionType {
	switch ct {
	case ManifestCompressionGZip:
		return ManifestCompressionGZipBestSpeed
	case ManifestCompressionGZipDefault, ManifestCompressionGZipBestSpeed,
		ManifestCompressionS2Encode, ManifestCompressionS2EncodeBetter,
		ManifestCompressionZLib, ManifestCompressionNone:
		return ct
	default:
		return ManifestCompressionGZipBestSpeed
	}
}

// normalizeManifestStorageType normalizes a storage type to a known value.
// Unknown values default to "json".
func normalizeManifestStorageType(st ManifestStorageType) ManifestStorageType {
	switch st {
	case ManifestStorageJSON, ManifestStorageJSONIter, ManifestStorageMsgPack:
		return st
	default:
		return ManifestStorageJSON
	}
}

// serializeManifestObject serializes a manifest object using the registered serializer.
func serializeManifestObject(obj map[string]any, storageType ManifestStorageType) ([]byte, error) {
	s, ok := serializers[storageType]
	if !ok {
		return nil, fmt.Errorf("unknown storage type: %s", storageType)
	}
	return s.marshal(obj)
}

// deserializeManifestObject deserializes manifest bytes using the registered serializer.
func deserializeManifestObject(data []byte, storageType ManifestStorageType) (map[string]any, error) {
	s, ok := serializers[storageType]
	if !ok {
		return nil, fmt.Errorf("unknown storage type: %s", storageType)
	}
	return s.unmarshal(data)
}

// compressManifestData compresses manifest bytes using the registered compressor.
func compressManifestData(data []byte, compressionType ManifestCompressionType) ([]byte, error) {
	c, ok := compressors[compressionType]
	if !ok {
		return nil, fmt.Errorf("unknown compression type: %s", compressionType)
	}
	return c.compress(data)
}

// decompressManifestData decompresses manifest bytes using the registered compressor.
func decompressManifestData(data []byte, compressionType ManifestCompressionType) ([]byte, error) {
	c, ok := compressors[compressionType]
	if !ok {
		return nil, fmt.Errorf("unknown compression type: %s", compressionType)
	}
	return c.decompress(data)
}

type jsonSerializer struct{}

func (jsonSerializer) marshal(obj map[string]any) ([]byte, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("json marshal failed: %w", err)
	}
	return data, nil
}

func (jsonSerializer) unmarshal(data []byte) (map[string]any, error) {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("json unmarshal failed: %w", err)
	}
	return obj, nil
}

type jsoniterSerializer struct{}

var jsoniterAPI = jsoniter.ConfigCompatibleWithStandardLibrary

func (jsoniterSerializer) marshal(obj map[string]any) ([]byte, error) {
	data, err := jsoniterAPI.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("jsoniter marshal failed: %w", err)
	}
	return data, nil
}

func (jsoniterSerializer) unmarshal(data []byte) (map[string]any, error) {
	var obj map[string]any
	if err := jsoniterAPI.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("jsoniter unmarshal failed: %w", err)
	}
	return obj, nil
}

type msgpackSerializer struct{}

func (msgpackSerializer) marshal(obj map[string]any) ([]byte, error) {
	data, err := msgpack.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("msgpack marshal failed: %w", err)
	}
	return data, nil
}

func (msgpackSerializer) unmarshal(data []byte) (map[string]any, error) {
	var obj map[string]any
	if err := msgpack.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("msgpack unmarshal failed: %w", err)
	}
	normalizeManifestValue(obj)
	return obj, nil
}

type gzipCompressor struct {
	level int
	pool  sync.Pool
}

func (c *gzipCompressor) compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer

	var gz *gzip.Writer
	if v := c.pool.Get(); v != nil {
		gz = v.(*gzip.Writer)
		gz.Reset(&buf)
	} else {
		var err error
		gz, err = gzip.NewWriterLevel(&buf, c.level)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip writer: %w", err)
		}
	}

	if _, err := gz.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write gzip data: %w", err)
	}
	if err := gz.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}
	c.pool.Put(gz)
	return buf.Bytes(), nil
}

func (*gzipCompressor) decompress(data []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gz.Close()
	result, err := io.ReadAll(gz)
	if err != nil {
		return nil, fmt.Errorf("failed to read gzip data: %w", err)
	}
	return result, nil
}

type s2Compressor struct{ better bool }

func (c s2Compressor) compress(data []byte) ([]byte, error) {
	if c.better {
		return s2.EncodeBetter(nil, data), nil
	}
	return s2.Encode(nil, data), nil
}

func (s2Compressor) decompress(data []byte) ([]byte, error) {
	result, err := s2.Decode(nil, data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode s2 data: %w", err)
	}
	return result, nil
}

type zlibCompressor struct{}

func (zlibCompressor) compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write zlib data: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to close zlib writer: %w", err)
	}
	return buf.Bytes(), nil
}

func (zlibCompressor) decompress(data []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create zlib reader: %w", err)
	}
	defer r.Close()
	result, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read zlib data: %w", err)
	}
	return result, nil
}

type noneCompressor struct{}

func (noneCompressor) compress(data []byte) ([]byte, error)   { return data, nil }
func (noneCompressor) decompress(data []byte) ([]byte, error) { return data, nil }

// normalizeManifestValue normalizes msgpack-decoded values to match
// unstructured.Unstructured expectations: integer types become float64,
// and map[any]any becomes map[string]any.
func normalizeManifestValue(obj map[string]any) {
	for k, v := range obj {
		obj[k] = normalizeValue(v)
	}
}

func normalizeValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		normalizeManifestValue(val)
		return val
	case map[any]any:
		converted := make(map[string]any, len(val))
		for mk, mv := range val {
			converted[fmt.Sprintf("%v", mk)] = normalizeValue(mv)
		}
		return converted
	case []any:
		for i, item := range val {
			val[i] = normalizeValue(item)
		}
		return val
	case uint64:
		return float64(val)
	case int64:
		return float64(val)
	case uint32:
		return float64(val)
	case int32:
		return float64(val)
	case int:
		return float64(val)
	case uint:
		return float64(val)
	default:
		return v
	}
}
