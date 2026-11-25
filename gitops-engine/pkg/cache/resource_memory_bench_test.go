package cache

import (
	"runtime"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// BenchmarkManifestRetainedMemory measures how much heap each cached manifest keeps alive
// after SetManifestWithCodec, for every supported codec. It reports:
//   - stored-B/manifest:   len() of the stored byte slice (the useful payload)
//   - capacity-B/manifest: cap() of the stored byte slice (what the cache actually keeps allocated)
//   - retained-B/manifest: live heap growth per manifest, measured via runtime.MemStats; this also
//     includes encoder pools and other process-wide buffers, so it is noisier than capacity-B
//
// The gap between stored-B and capacity-B is memory wasted by spare slice capacity.
func BenchmarkManifestRetainedMemory(b *testing.B) {
	storageTypes := []ManifestStorageType{ManifestStorageJSON, ManifestStorageJSONIter, ManifestStorageMsgPack}
	compressionTypes := []ManifestCompressionType{
		ManifestCompressionGZipBestSpeed,
		ManifestCompressionGZipDefault,
		ManifestCompressionS2Encode,
		ManifestCompressionS2EncodeBetter,
		ManifestCompressionZLib,
		ManifestCompressionNone,
	}

	for _, testCase := range []struct {
		name           string
		un             *unstructured.Unstructured
		manifestsPerOp int
	}{
		// Enough copies per iteration for the heap delta to dominate GC noise, without making
		// each iteration of the ~1 MB case take seconds.
		{name: "typical-deployment", un: typicalDeploymentManifest(), manifestsPerOp: 200},
		{name: "large-configmap", un: largeConfigMapManifest(), manifestsPerOp: 32},
	} {
		for _, storageType := range storageTypes {
			for _, compressionType := range compressionTypes {
				b.Run(testCase.name+"/"+string(storageType)+"/"+string(compressionType), func(b *testing.B) {
					manifestsPerOp := testCase.manifestsPerOp
					resources := make([]Resource, manifestsPerOp)
					var retained, stored, capacity uint64
					var ms runtime.MemStats
					b.ReportAllocs()
					for b.Loop() {
						for i := range resources {
							resources[i] = Resource{}
						}
						b.StopTimer()
						runtime.GC()
						runtime.ReadMemStats(&ms)
						before := ms.HeapAlloc
						b.StartTimer()

						for i := range resources {
							if err := resources[i].SetManifestWithCodec(testCase.un, storageType, compressionType); err != nil {
								b.Fatal(err)
							}
						}

						b.StopTimer()
						runtime.GC()
						runtime.ReadMemStats(&ms)
						if ms.HeapAlloc > before {
							retained += ms.HeapAlloc - before
						}
						for i := range resources {
							stored += uint64(len(resources[i].compressedManifest))
							capacity += uint64(cap(resources[i].compressedManifest))
						}
						b.StartTimer()
					}
					runtime.KeepAlive(resources)
					n := float64(b.N) * float64(manifestsPerOp)
					b.ReportMetric(float64(stored)/n, "stored-B/manifest")
					b.ReportMetric(float64(capacity)/n, "capacity-B/manifest")
					b.ReportMetric(float64(retained)/n, "retained-B/manifest")
				})
			}
		}
	}
}
