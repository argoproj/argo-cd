package registry

import (
	"testing"

	"github.com/argoproj/argo-cd/v2/image-updater/options"

	"github.com/distribution/distribution/v3/manifest/schema1"
	"github.com/stretchr/testify/require"
)

func Test_TagMetadata(t *testing.T) {
	t.Run("Check for correct error handling when manifest contains no history", func(t *testing.T) {
		meta1 := &schema1.SignedManifest{
			Manifest: schema1.Manifest{
				History: []schema1.History{},
			},
		}
		ep, err := GetRegistryEndpoint("")
		require.NoError(t, err)
		client, err := NewClient(ep, "", "")
		require.NoError(t, err)
		_, err = client.TagMetadata(meta1, &options.ManifestOptions{})
		require.Error(t, err)
	})

	t.Run("Check for correct error handling when manifest contains invalid history", func(t *testing.T) {
		meta1 := &schema1.SignedManifest{
			Manifest: schema1.Manifest{
				History: []schema1.History{
					{
						V1Compatibility: `{"created": {"something": "notastring"}}`,
					},
				},
			},
		}

		ep, err := GetRegistryEndpoint("")
		require.NoError(t, err)
		client, err := NewClient(ep, "", "")
		require.NoError(t, err)
		_, err = client.TagMetadata(meta1, &options.ManifestOptions{})
		require.Error(t, err)
	})

	t.Run("Check for correct error handling when manifest contains invalid history", func(t *testing.T) {
		meta1 := &schema1.SignedManifest{
			Manifest: schema1.Manifest{
				History: []schema1.History{
					{
						V1Compatibility: `{"something": "something"}`,
					},
				},
			},
		}

		ep, err := GetRegistryEndpoint("")
		require.NoError(t, err)
		client, err := NewClient(ep, "", "")
		require.NoError(t, err)
		_, err = client.TagMetadata(meta1, &options.ManifestOptions{})
		require.Error(t, err)

	})

	t.Run("Check for correct error handling when time stamp cannot be parsed", func(t *testing.T) {
		ts := "invalid"
		meta1 := &schema1.SignedManifest{
			Manifest: schema1.Manifest{
				History: []schema1.History{
					{
						V1Compatibility: `{"created":"` + ts + `"}`,
					},
				},
			},
		}
		ep, err := GetRegistryEndpoint("")
		require.NoError(t, err)
		client, err := NewClient(ep, "", "")
		require.NoError(t, err)
		_, err = client.TagMetadata(meta1, &options.ManifestOptions{})
		require.Error(t, err)
	})
}
