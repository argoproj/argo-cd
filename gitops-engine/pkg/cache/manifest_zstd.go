package cache

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"sync"

	"github.com/klauspost/compress/zstd"
	"github.com/vmihailenco/msgpack/v5"
	k8sjson "k8s.io/apimachinery/pkg/util/json"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/cache/manifestdict"
)

const (
	// zstdMaxDictionarySize is the most dictionary content used per byte format. zstd can reference
	// further back, but this bounds memory and keeps the sample focused.
	zstdMaxDictionarySize = 64 << 10
	// zstdWindowSize bounds the encoder history buffer. zstd's default (8 MiB) costs that much
	// memory per encoder with no measurable gain on Kubernetes manifests.
	zstdWindowSize = 1 << 20
	// zstdConcurrency bounds how many encoders and decoders are kept. EncodeAll and DecodeAll wait
	// for a free one, so this also caps parallelism; at tens of microseconds per manifest, four
	// handle on the order of 100,000 manifests per second.
	zstdConcurrency = 4

	// Raw dictionaries still need an ID in the frame header. These are in the range zstd leaves to
	// applications (>= 32768). Frames never leave the process, so they only need to be distinct.
	zstdDictIDJSON    = 32768
	zstdDictIDMsgPack = 32769
)

// zstdCompressor compresses manifests with zstd, using a raw dictionary built from
// manifestdict.Source for the manifest's byte format. The encoders and decoder are created on
// first use, so the cost is only paid when the zstd compression type is configured.
type zstdCompressor struct {
	once       sync.Once
	err        error
	encJSON    *zstd.Encoder
	encMsgPack *zstd.Encoder
	dec        *zstd.Decoder
}

func (c *zstdCompressor) init() error {
	c.once.Do(func() {
		c.err = c.build(manifestdict.Source)
	})
	return c.err
}

func (c *zstdCompressor) build(source []byte) error {
	jsonDict, msgpackDict, err := buildManifestDictionaries(source)
	if err != nil {
		return err
	}
	encoderOptions := func(id uint32, dict []byte) []zstd.EOption {
		return []zstd.EOption{
			zstd.WithEncoderLevel(zstd.SpeedFastest),
			zstd.WithEncoderConcurrency(zstdConcurrency),
			zstd.WithWindowSize(zstdWindowSize),
			// Manifests are only held in memory, so a checksum per frame adds bytes without value.
			zstd.WithEncoderCRC(false),
			zstd.WithLowerEncoderMem(true),
			zstd.WithEncoderDictRaw(id, dict),
		}
	}
	if c.encJSON, err = zstd.NewWriter(nil, encoderOptions(zstdDictIDJSON, jsonDict)...); err != nil {
		return fmt.Errorf("failed to create zstd encoder: %w", err)
	}
	if c.encMsgPack, err = zstd.NewWriter(nil, encoderOptions(zstdDictIDMsgPack, msgpackDict)...); err != nil {
		return fmt.Errorf("failed to create zstd encoder: %w", err)
	}
	c.dec, err = zstd.NewReader(nil,
		zstd.WithDecoderConcurrency(zstdConcurrency),
		zstd.WithDecoderLowmem(true),
		zstd.WithDecoderDictRaw(zstdDictIDJSON, jsonDict),
		zstd.WithDecoderDictRaw(zstdDictIDMsgPack, msgpackDict),
	)
	if err != nil {
		return fmt.Errorf("failed to create zstd decoder: %w", err)
	}
	return nil
}

func (c *zstdCompressor) encoder(storageType ManifestStorageType) *zstd.Encoder {
	if storageType == ManifestStorageMsgPack {
		return c.encMsgPack
	}
	// json and jsoniter produce the same bytes.
	return c.encJSON
}

func (c *zstdCompressor) compress(data []byte, storageType ManifestStorageType) ([]byte, error) {
	if err := c.init(); err != nil {
		return nil, err
	}
	return c.encoder(storageType).EncodeAll(data, nil), nil
}

func (c *zstdCompressor) decompress(data []byte, _ ManifestStorageType) ([]byte, error) {
	if err := c.init(); err != nil {
		return nil, err
	}
	// Each frame names its dictionary, and the decoder holds both, so the storage type isn't needed.
	result, err := c.dec.DecodeAll(data, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decode zstd data: %w", err)
	}
	return result, nil
}

// buildManifestDictionaries returns the JSON and msgpack dictionaries for source, each trimmed to
// its last zstdMaxDictionarySize bytes. zstd references the end of a dictionary most cheaply,
// which is why the sample lists the most common kinds last.
func buildManifestDictionaries(source []byte) (jsonDict, msgpackDict []byte, err error) {
	jsonSample, msgpackSample, err := serializeDictionarySample(source)
	if err != nil {
		return nil, nil, err
	}
	return lastBytes(jsonSample, zstdMaxDictionarySize), lastBytes(msgpackSample, zstdMaxDictionarySize), nil
}

// serializeDictionarySample serializes each sample object in source (one JSON object per line)
// as JSON and as msgpack, and returns the concatenation for each format.
func serializeDictionarySample(source []byte) (jsonSample, msgpackSample []byte, err error) {
	var jsonBuf, msgpackBuf bytes.Buffer
	enc := msgpack.NewEncoder(&msgpackBuf)
	// Sorted keys make the dictionary bytes the same on every run. Stored manifests are
	// msgpack-encoded without sorting, but they still share every key and value string.
	enc.SetSortMapKeys(true)

	scanner := bufio.NewScanner(bytes.NewReader(source))
	scanner.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var obj map[string]any
		// Decode the way unstructured objects are decoded, so integers are int64 rather than
		// float64 and msgpack encodes them as the cache does.
		if err := k8sjson.Unmarshal(line, &obj); err != nil {
			return nil, nil, fmt.Errorf("invalid manifest dictionary sample: %w", err)
		}
		data, err := serializeManifestObject(obj, ManifestStorageJSON)
		if err != nil {
			return nil, nil, err
		}
		jsonBuf.Write(data)
		if err := enc.Encode(obj); err != nil {
			return nil, nil, fmt.Errorf("msgpack marshal failed: %w", err)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("failed to read manifest dictionary sample: %w", err)
	}
	if jsonBuf.Len() == 0 {
		return nil, nil, errors.New("manifest dictionary sample is empty")
	}
	return jsonBuf.Bytes(), msgpackBuf.Bytes(), nil
}

func lastBytes(b []byte, n int) []byte {
	if len(b) > n {
		b = b[len(b)-n:]
	}
	return bytes.Clone(b)
}
