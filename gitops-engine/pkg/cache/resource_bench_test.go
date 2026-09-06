package cache

import "testing"

func BenchmarkManifestSerialization(b *testing.B) {
	un := mustToUnstructured(testDeploy())
	storageTypes := []ManifestStorageType{
		ManifestStorageJSON,
		ManifestStorageJSONIter,
		ManifestStorageMsgPack,
	}

	for _, storageType := range storageTypes {
		b.Run(string(storageType)+"/write", func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				resource := &Resource{}
				if err := resource.SetManifestWithCodec(un, storageType, ManifestCompressionNone); err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(string(storageType)+"/read", func(b *testing.B) {
			resource := &Resource{}
			if err := resource.SetManifestWithCodec(un, storageType, ManifestCompressionNone); err != nil {
				b.Fatal(err)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				manifest, err := resource.GetManifest()
				if err != nil {
					b.Fatal(err)
				}
				if manifest == nil {
					b.Fatal("expected manifest")
				}
			}
		})
	}
}
