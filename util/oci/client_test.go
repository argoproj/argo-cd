package oci

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"

	argoio "github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/io/files"
)

type layerConf struct {
	desc  v1.Descriptor
	bytes []byte
}

func generateManifest(t *testing.T, store *memory.Store, layerDescs ...layerConf) string {
	configBlob := []byte("Hello config")
	configDesc := content.NewDescriptorFromBytes(v1.MediaTypeImageConfig, configBlob)

	var layers []v1.Descriptor

	for _, layer := range layerDescs {
		layers = append(layers, layer.desc)
	}

	manifestBlob, err := json.Marshal(v1.Manifest{
		Config:    configDesc,
		Layers:    layers,
		Versioned: specs.Versioned{SchemaVersion: 2},
	})
	require.NoError(t, err)
	manifestDesc := content.NewDescriptorFromBytes(v1.MediaTypeImageManifest, manifestBlob)

	for _, layer := range layerDescs {
		require.NoError(t, store.Push(context.Background(), layer.desc, bytes.NewReader(layer.bytes)))
	}

	require.NoError(t, store.Push(context.Background(), configDesc, bytes.NewReader(configBlob)))
	require.NoError(t, store.Push(context.Background(), manifestDesc, bytes.NewReader(manifestBlob)))
	require.NoError(t, store.Tag(context.Background(), manifestDesc, manifestDesc.Digest.String()))

	return manifestDesc.Digest.String()
}

func createGzippedTarWithContent(t *testing.T, filename, content string) []byte {
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name: filename,
		Mode: 0o644,
		Size: int64(len(content)),
	}))
	_, err := tw.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gzw.Close())

	return buf.Bytes()
}

func Test_nativeOCIClient_Extract(t *testing.T) {
	cacheDir := argoio.NewRandomizedTempPaths(t.TempDir())

	type fields struct {
		creds             Creds
		repoURL           string
		tagsFunc          func(context.Context, string) (tags []string, err error)
		allowedMediaTypes []string
	}
	type args struct {
		project                         string
		manifestMaxExtractedSize        int64
		disableManifestMaxExtractedSize bool
		digestFunc                      func(*memory.Store) string
		postValidationFunc              func(string, string, Client, fields, args)
	}
	tests := []struct {
		name          string
		fields        fields
		args          args
		expectedError error
	}{
		{
			name: "extraction fails due to size limit",
			fields: fields{
				allowedMediaTypes: []string{v1.MediaTypeImageLayerGzip},
			},
			args: args{
				digestFunc: func(store *memory.Store) string {
					layerBlob := createGzippedTarWithContent(t, "some-path", "some content")
					return generateManifest(t, store, layerConf{content.NewDescriptorFromBytes(v1.MediaTypeImageLayerGzip, layerBlob), layerBlob})
				},
				manifestMaxExtractedSize:        10,
				disableManifestMaxExtractedSize: false,
			},
			expectedError: fmt.Errorf("could not decompress layer: error while iterating on tar reader: unexpected EOF"),
		},
		{
			name: "extraction fails due to multiple layers",
			fields: fields{
				allowedMediaTypes: []string{v1.MediaTypeImageLayerGzip},
			},
			args: args{
				digestFunc: func(store *memory.Store) string {
					layerBlob := createGzippedTarWithContent(t, "some-path", "some content")
					otherLayerBlob := createGzippedTarWithContent(t, "some-other-path", "some other content")
					return generateManifest(t, store, layerConf{content.NewDescriptorFromBytes(v1.MediaTypeImageLayerGzip, layerBlob), layerBlob}, layerConf{content.NewDescriptorFromBytes(v1.MediaTypeImageLayerGzip, otherLayerBlob), otherLayerBlob})
				},
				manifestMaxExtractedSize:        1000,
				disableManifestMaxExtractedSize: false,
			},
			expectedError: fmt.Errorf("expected only a single oci layer, got 2"),
		},
		{
			name: "extraction fails due to invalid media type",
			fields: fields{
				allowedMediaTypes: []string{"application/vnd.different.media.type"},
			},
			args: args{
				digestFunc: func(store *memory.Store) string {
					layerBlob := "Hello layer"
					return generateManifest(t, store, layerConf{content.NewDescriptorFromBytes(v1.MediaTypeImageLayerGzip, []byte(layerBlob)), []byte(layerBlob)})
				},
				manifestMaxExtractedSize:        1000,
				disableManifestMaxExtractedSize: false,
			},
			expectedError: fmt.Errorf("oci layer media type application/vnd.oci.image.layer.v1.tar+gzip is not in the list of allowed media types"),
		},
		{
			name: "extraction fails due to non-existent digest",
			fields: fields{
				allowedMediaTypes: []string{"application/vnd.cncf.helm.chart.content.v1.tar+gzip"},
			},
			args: args{
				digestFunc: func(store *memory.Store) string {
					return "sha256:nonexistentdigest"
				},
				manifestMaxExtractedSize:        1000,
				disableManifestMaxExtractedSize: false,
			},
			expectedError: fmt.Errorf("not found"),
		},
		{
			name: "extraction with helm chart",
			fields: fields{
				allowedMediaTypes: []string{"application/vnd.cncf.helm.chart.content.v1.tar+gzip"},
			},
			args: args{
				digestFunc: func(store *memory.Store) string {
					helmBlob := createGzippedTarWithContent(t, "Chart.yaml", "some content")
					layerBlob := createGzippedTarWithContent(t, "chart.tar.gz", string(helmBlob))
					return generateManifest(t, store, layerConf{content.NewDescriptorFromBytes("application/vnd.cncf.helm.chart.content.v1.tar+gzip", layerBlob), layerBlob})
				},
				postValidationFunc: func(sha string, path string, _ Client, _ fields, _ args) {
					tempDir, err := files.CreateTempDir(os.TempDir())
					defer os.RemoveAll(tempDir)
					require.NoError(t, err)
					chartDir, err := os.ReadDir(path)
					require.NoError(t, err)
					require.Len(t, chartDir, 1)
					require.Equal(t, "chart.tar.gz", chartDir[0].Name())
					tarBall, err := os.Open(filepath.Join(path, chartDir[0].Name()))
					require.NoError(t, err)
					err = files.Untgz(tempDir, tarBall, math.MaxInt64, false)
					require.NoError(t, err)
					unpacked, err := os.ReadDir(tempDir)
					require.NoError(t, err)
					require.Len(t, unpacked, 1)
					require.Equal(t, "Chart.yaml", unpacked[0].Name())
					chartYaml, err := os.Open(filepath.Join(tempDir, unpacked[0].Name()))
					require.NoError(t, err)
					contents, err := io.ReadAll(chartYaml)
					require.NoError(t, err)
					require.Equal(t, "some content", string(contents))
				},
				manifestMaxExtractedSize:        1000,
				disableManifestMaxExtractedSize: false,
			},
		},
		{
			name: "extraction with standard gzip layer",
			fields: fields{
				allowedMediaTypes: []string{v1.MediaTypeImageLayerGzip},
			},
			args: args{
				digestFunc: func(store *memory.Store) string {
					layerBlob := createGzippedTarWithContent(t, "foo.yaml", "some content")
					return generateManifest(t, store, layerConf{content.NewDescriptorFromBytes(v1.MediaTypeImageLayerGzip, layerBlob), layerBlob})
				},
				postValidationFunc: func(sha string, path string, _ Client, _ fields, _ args) {
					manifestDir, err := os.ReadDir(path)
					require.NoError(t, err)
					require.Len(t, manifestDir, 1)
					require.Equal(t, "foo.yaml", manifestDir[0].Name())
					f, err := os.Open(filepath.Join(path, manifestDir[0].Name()))
					require.NoError(t, err)
					contents, err := io.ReadAll(f)
					require.NoError(t, err)
					require.Equal(t, "some content", string(contents))
				},
				manifestMaxExtractedSize:        1000,
				disableManifestMaxExtractedSize: false,
			},
		},
		{
			name: "extraction with standard gzip layer using cache",
			fields: fields{
				allowedMediaTypes: []string{v1.MediaTypeImageLayerGzip},
			},
			args: args{
				digestFunc: func(store *memory.Store) string {
					layerBlob := createGzippedTarWithContent(t, "foo.yaml", "some content")
					return generateManifest(t, store, layerConf{content.NewDescriptorFromBytes(v1.MediaTypeImageLayerGzip, layerBlob), layerBlob})
				},
				manifestMaxExtractedSize:        1000,
				disableManifestMaxExtractedSize: false,
				postValidationFunc: func(sha string, _ string, _ Client, fields fields, args args) {
					store := memory.New()
					c := newClientWithLock(fields.repoURL, fields.creds, globalLock, store, fields.tagsFunc, fields.allowedMediaTypes, WithImagePaths(cacheDir))
					_, gotCloser, err := c.Extract(context.Background(), sha, args.project, args.manifestMaxExtractedSize, args.disableManifestMaxExtractedSize)
					require.NoError(t, err)
					require.NoError(t, gotCloser.Close())
				},
			},
		},
		{
			name: "extraction with standard gzip layer using cache, invalid project",
			fields: fields{
				allowedMediaTypes: []string{v1.MediaTypeImageLayerGzip},
			},
			args: args{
				digestFunc: func(store *memory.Store) string {
					layerBlob := createGzippedTarWithContent(t, "foo.yaml", "some content")
					return generateManifest(t, store, layerConf{content.NewDescriptorFromBytes(v1.MediaTypeImageLayerGzip, layerBlob), layerBlob})
				},
				manifestMaxExtractedSize:        1000,
				disableManifestMaxExtractedSize: false,
				postValidationFunc: func(sha string, _ string, _ Client, fields fields, args args) {
					store := memory.New()
					c := newClientWithLock(fields.repoURL, fields.creds, globalLock, store, fields.tagsFunc, fields.allowedMediaTypes, WithImagePaths(cacheDir))
					_, _, err := c.Extract(context.Background(), sha, "non-existent-project", args.manifestMaxExtractedSize, args.disableManifestMaxExtractedSize)
					require.Error(t, err)
					require.Equal(t, fmt.Errorf("not found"), err)
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := memory.New()
			sha := tt.args.digestFunc(store)
			if tt.args.project == "" {
				tt.args.project = tt.name
			}

			c := newClientWithLock(tt.fields.repoURL, tt.fields.creds, globalLock, store, tt.fields.tagsFunc, tt.fields.allowedMediaTypes, WithImagePaths(cacheDir))
			path, gotCloser, err := c.Extract(context.Background(), sha, tt.args.project, tt.args.manifestMaxExtractedSize, tt.args.disableManifestMaxExtractedSize)

			if tt.expectedError != nil {
				require.EqualError(t, err, tt.expectedError.Error())
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, path)
			require.NotNil(t, gotCloser)

			exists, err := fileExists(path)
			require.True(t, exists)
			require.NoError(t, err)

			if tt.args.postValidationFunc != nil {
				tt.args.postValidationFunc(sha, path, c, tt.fields, tt.args)
			}

			require.NoError(t, gotCloser.Close())

			exists, err = fileExists(path)
			require.False(t, exists)
			require.NoError(t, err)
		})
	}
}

func Test_nativeOCIClient_ResolveRevision(t *testing.T) {
	store := memory.New()
	data := []byte("")
	descriptor := v1.Descriptor{
		MediaType: "",
		Digest:    digest.FromBytes(data),
	}
	require.NoError(t, store.Push(context.Background(), descriptor, bytes.NewReader(data)))
	require.NoError(t, store.Tag(context.Background(), descriptor, "latest"))
	require.NoError(t, store.Tag(context.Background(), descriptor, "1.2.0"))
	require.NoError(t, store.Tag(context.Background(), descriptor, descriptor.Digest.String()))

	type fields struct {
		creds             Creds
		repoURL           string
		repo              oras.ReadOnlyTarget
		tagsFunc          func(context.Context, string) (tags []string, err error)
		allowedMediaTypes []string
	}
	tests := []struct {
		name           string
		fields         fields
		revision       string
		noCache        bool
		expectedDigest string
		expectedError  error
	}{
		{
			name:     "resolve semantic version constraint",
			revision: "^1.0.0",
			fields: fields{repo: store, tagsFunc: func(context.Context, string) (tags []string, err error) {
				return []string{"1.0.0", "1.1.0", "1.2.0", "2.0.0"}, nil
			}},
			expectedDigest: descriptor.Digest.String(),
		},
		{
			name:     "resolve exact version",
			revision: "1.2.0",
			fields: fields{repo: store, tagsFunc: func(context.Context, string) (tags []string, err error) {
				return []string{"1.0.0", "1.1.0", "1.2.0", "2.0.0"}, nil
			}},
			expectedDigest: descriptor.Digest.String(),
		},
		{
			name:     "resolve digest directly",
			revision: descriptor.Digest.String(),
			fields: fields{repo: store, tagsFunc: func(context.Context, string) (tags []string, err error) {
				return []string{}, fmt.Errorf("this should not be invoked")
			}},
			expectedDigest: descriptor.Digest.String(),
		},
		{
			name:     "no matching version for constraint",
			revision: "^3.0.0",
			fields: fields{repo: store, tagsFunc: func(context.Context, string) (tags []string, err error) {
				return []string{"1.0.0", "1.1.0", "1.2.0", "2.0.0"}, nil
			}},
			expectedError: fmt.Errorf("no version for constraints: constraint not found in 4 tags"),
		},
		{
			name:     "error fetching tags",
			revision: "^1.0.0",
			fields: fields{repo: store, tagsFunc: func(context.Context, string) (tags []string, err error) {
				return []string{}, fmt.Errorf("some random error")
			}},
			expectedError: fmt.Errorf("error fetching tags: failed to get tags: some random error"),
		},
		{
			name:     "error resolving digest",
			revision: "sha256:abc123",
			fields: fields{repo: store, tagsFunc: func(context.Context, string) (tags []string, err error) {
				return []string{"1.0.0", "1.1.0", "1.2.0", "2.0.0"}, nil
			}},
			expectedError: fmt.Errorf("cannot get digest: not found"),
		},
		// new tests
		{
			name:     "resolve latest tag",
			revision: "latest",
			fields: fields{repo: store, tagsFunc: func(context.Context, string) (tags []string, err error) {
				return []string{"1.0.0", "1.1.0", "1.2.0", "2.0.0", "latest"}, nil
			}},
			expectedDigest: descriptor.Digest.String(),
		},
		{
			name:     "resolve with complex semver constraint",
			revision: ">=1.0.0 <2.0.0",
			fields: fields{repo: store, tagsFunc: func(context.Context, string) (tags []string, err error) {
				return []string{"0.9.0", "1.0.0", "1.1.0", "1.2.0", "2.0.0", "2.1.0"}, nil
			}},
			expectedDigest: descriptor.Digest.String(),
		},
		{
			name:     "resolve with only non-semver tags",
			revision: "^1.0.0",
			fields: fields{repo: store, tagsFunc: func(context.Context, string) (tags []string, err error) {
				return []string{"latest", "stable", "prod", "dev"}, nil
			}},
			expectedError: fmt.Errorf("no version for constraints: constraint not found in 4 tags"),
		},
		{
			name:     "resolve with empty tag list",
			revision: "^1.0.0",
			fields: fields{repo: store, tagsFunc: func(context.Context, string) (tags []string, err error) {
				return []string{}, nil
			}},
			expectedError: fmt.Errorf("no version for constraints: constraint not found in 0 tags"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newClientWithLock(tt.fields.repoURL, tt.fields.creds, globalLock, tt.fields.repo, tt.fields.tagsFunc, tt.fields.allowedMediaTypes)
			got, err := c.ResolveRevision(context.Background(), tt.revision, tt.noCache)
			if tt.expectedError != nil {
				require.EqualError(t, err, tt.expectedError.Error())
				return
			}

			require.NoError(t, err)
			if got != tt.expectedDigest {
				t.Errorf("ResolveRevision() got = %v, expectedDigest %v", got, tt.expectedDigest)
			}
		})
	}
}
