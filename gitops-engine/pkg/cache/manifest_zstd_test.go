package cache

import (
	"bytes"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/cache/manifestdict"
)

func TestManifestDictionary(t *testing.T) {
	jsonDict, msgpackDict, err := buildManifestDictionaries(manifestdict.Source)
	require.NoError(t, err)

	t.Run("fits without truncation", func(t *testing.T) {
		// The sample is trimmed from the front if it grows past the limit, which would silently
		// drop its least common kinds. Keep it within the limit instead.
		jsonSample, msgpackSample, err := serializeDictionarySample(manifestdict.Source)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(jsonSample), zstdMaxDictionarySize, "dictionary.jsonl is too large when serialized as JSON")
		assert.LessOrEqual(t, len(msgpackSample), zstdMaxDictionarySize, "dictionary.jsonl is too large when serialized as msgpack")
		assert.True(t, bytes.Equal(jsonSample, jsonDict), "JSON dictionary differs from the serialized sample")
		assert.True(t, bytes.Equal(msgpackSample, msgpackDict), "msgpack dictionary differs from the serialized sample")
	})

	t.Run("is deterministic", func(t *testing.T) {
		for range 5 {
			j, m, err := buildManifestDictionaries(manifestdict.Source)
			require.NoError(t, err)
			assert.True(t, bytes.Equal(jsonDict, j), "JSON dictionary changed between builds")
			assert.True(t, bytes.Equal(msgpackDict, m), "msgpack dictionary changed between builds")
		}
	})

	t.Run("rejects an empty sample", func(t *testing.T) {
		_, _, err := buildManifestDictionaries([]byte("\n"))
		require.ErrorContains(t, err, "empty")
	})
}

func TestZstdCompressor_FramesUseFormatDictionary(t *testing.T) {
	c := compressors[ManifestCompressionZstd].(*zstdCompressor)
	for storageType, wantID := range map[ManifestStorageType]uint32{
		ManifestStorageJSON:     zstdDictIDJSON,
		ManifestStorageJSONIter: zstdDictIDJSON,
		ManifestStorageMsgPack:  zstdDictIDMsgPack,
	} {
		t.Run(string(storageType), func(t *testing.T) {
			data, err := serializeManifestObject(typicalDeploymentManifest().Object, storageType)
			require.NoError(t, err)
			frame, err := c.compress(data, storageType)
			require.NoError(t, err)

			var header zstd.Header
			require.NoError(t, header.Decode(frame))
			assert.Equal(t, wantID, header.DictionaryID)

			got, err := c.decompress(frame, storageType)
			require.NoError(t, err)
			assert.Equal(t, data, got)
		})
	}
}

// TestZstdCompressor_DictionaryImprovesCompression guards against a dictionary that is empty,
// mismatched with its byte format, or otherwise not helping.
func TestZstdCompressor_DictionaryImprovesCompression(t *testing.T) {
	c := compressors[ManifestCompressionZstd].(*zstdCompressor)
	plain, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedFastest), zstd.WithEncoderCRC(false), zstd.WithWindowSize(zstdWindowSize))
	require.NoError(t, err)
	defer plain.Close()

	for _, storageType := range []ManifestStorageType{ManifestStorageJSON, ManifestStorageMsgPack} {
		t.Run(string(storageType), func(t *testing.T) {
			data, err := serializeManifestObject(typicalDeploymentManifest().Object, storageType)
			require.NoError(t, err)
			withDict, err := c.compress(data, storageType)
			require.NoError(t, err)
			withoutDict := plain.EncodeAll(data, nil)
			assert.Less(t, len(withDict), len(withoutDict)*9/10,
				"dictionary should save at least 10%%: with=%d without=%d", len(withDict), len(withoutDict))
		})
	}
}
