package oci

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	imagev1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"

	utilio "github.com/argoproj/argo-cd/v3/util/io"
	"github.com/argoproj/argo-cd/v3/util/io/files"
)

type layerConf struct {
	desc  imagev1.Descriptor
	bytes []byte
}

func generateManifest(t *testing.T, store *memory.Store, layerDescs ...layerConf) string {
	t.Helper()
	return generateManifestWithConfig(t, store, imagev1.MediaTypeImageConfig, layerDescs...)
}

func generateManifestWithConfig(t *testing.T, store *memory.Store, configMediaType string, layerDescs ...layerConf) string {
	t.Helper()
	configBlob := []byte("Hello config")
	configDesc := content.NewDescriptorFromBytes(configMediaType, configBlob)

	var layers []imagev1.Descriptor

	for _, layer := range layerDescs {
		layers = append(layers, layer.desc)
	}

	manifestBlob, err := json.Marshal(imagev1.Manifest{
		Config:    configDesc,
		Layers:    layers,
		Versioned: specs.Versioned{SchemaVersion: 2},
	})
	require.NoError(t, err)
	manifestDesc := content.NewDescriptorFromBytes(imagev1.MediaTypeImageManifest, manifestBlob)

	for _, layer := range layerDescs {
		require.NoError(t, store.Push(t.Context(), layer.desc, bytes.NewReader(layer.bytes)))
	}

	require.NoError(t, store.Push(t.Context(), configDesc, bytes.NewReader(configBlob)))
	require.NoError(t, store.Push(t.Context(), manifestDesc, bytes.NewReader(manifestBlob)))
	require.NoError(t, store.Tag(t.Context(), manifestDesc, manifestDesc.Digest.String()))

	return manifestDesc.Digest.String()
}

func createGzippedTarWithContent(t *testing.T, filename, content string) []byte {
	t.Helper()
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

func addFileToDirectory(t *testing.T, dir, filename, content string) {
	t.Helper()

	filePath := filepath.Join(dir, filename)
	err := os.WriteFile(filePath, []byte(content), 0o644)
	require.NoError(t, err)
}

func Test_nativeOCIClient_Extract(t *testing.T) {
	cacheDir := utilio.NewRandomizedTempPaths(t.TempDir())

	type fields struct {
		repoURL           string
		tagsFunc          func(context.Context, string) (tags []string, err error)
		allowedMediaTypes []string
	}
	type args struct {
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
				allowedMediaTypes: []string{imagev1.MediaTypeImageLayerGzip},
			},
			args: args{
				digestFunc: func(store *memory.Store) string {
					layerBlob := createGzippedTarWithContent(t, "some-path", "some content")
					return generateManifest(t, store, layerConf{content.NewDescriptorFromBytes(imagev1.MediaTypeImageLayerGzip, layerBlob), layerBlob})
				},
				manifestMaxExtractedSize:        10,
				disableManifestMaxExtractedSize: false,
			},
			expectedError: errors.New("cannot extract contents of oci image with revision sha256:1b6dfd71e2b35c2f35dffc39007c2276f3c0e235cbae4c39cba74bd406174e22: failed to perform \"Push\" on destination: could not decompress layer: error while iterating on tar reader: unexpected EOF"),
		},
		{
			name: "extraction fails due to multiple content layers",
			fields: fields{
				allowedMediaTypes: []string{imagev1.MediaTypeImageLayerGzip},
			},
			args: args{
				digestFunc: func(store *memory.Store) string {
					layerBlob := createGzippedTarWithContent(t, "some-path", "some content")
					otherLayerBlob := createGzippedTarWithContent(t, "some-other-path", "some other content")
					return generateManifest(t, store, layerConf{content.NewDescriptorFromBytes(imagev1.MediaTypeImageLayerGzip, layerBlob), layerBlob}, layerConf{content.NewDescriptorFromBytes(imagev1.MediaTypeImageLayerGzip, otherLayerBlob), otherLayerBlob})
				},
				manifestMaxExtractedSize:        1000,
				disableManifestMaxExtractedSize: false,
			},
			expectedError: errors.New("expected only a single oci content layer, got 2"),
		},
		{
			name: "extraction with multiple layers, but just a single content layer",
			fields: fields{
				allowedMediaTypes: []string{imagev1.MediaTypeImageLayerGzip},
			},
			args: args{
				digestFunc: func(store *memory.Store) string {
					layerBlob := createGzippedTarWithContent(t, "some-path", "some content")
					return generateManifest(t, store, layerConf{content.NewDescriptorFromBytes(imagev1.MediaTypeImageLayerGzip, layerBlob), layerBlob}, layerConf{content.NewDescriptorFromBytes("application/vnd.cncf.helm.chart.provenance.v1.prov", []byte{}), []byte{}})
				},
				manifestMaxExtractedSize:        1000,
				disableManifestMaxExtractedSize: false,
			},
		},
		{
			name: "extraction fails due to invalid media type",
			fields: fields{
				allowedMediaTypes: []string{"application/vnd.different.media.type"},
			},
			args: args{
				digestFunc: func(store *memory.Store) string {
					layerBlob := "Hello layer"
					return generateManifest(t, store, layerConf{content.NewDescriptorFromBytes(imagev1.MediaTypeImageLayerGzip, []byte(layerBlob)), []byte(layerBlob)})
				},
				manifestMaxExtractedSize:        1000,
				disableManifestMaxExtractedSize: false,
			},
			expectedError: errors.New("oci layer media type application/vnd.oci.image.layer.v1.tar+gzip is not in the list of allowed media types"),
		},
		{
			name: "extraction fails due to non-existent digest",
			fields: fields{
				allowedMediaTypes: []string{"application/vnd.cncf.helm.chart.content.v1.tar+gzip"},
			},
			args: args{
				digestFunc: func(_ *memory.Store) string {
					return "sha256:nonexistentdigest"
				},
				manifestMaxExtractedSize:        1000,
				disableManifestMaxExtractedSize: false,
			},
			expectedError: errors.New("error resolving oci repo from digest, sha256:nonexistentdigest: not found"),
		},
		{
			name: "extraction with helm chart",
			fields: fields{
				allowedMediaTypes: []string{"application/vnd.cncf.helm.chart.content.v1.tar+gzip"},
			},
			args: args{
				digestFunc: func(store *memory.Store) string {
					chartDir := t.TempDir()
					chartName := "mychart"

					parent := filepath.Join(chartDir, "parent")
					require.NoError(t, os.Mkdir(parent, 0o755))

					chartPath := filepath.Join(parent, chartName)
					require.NoError(t, os.Mkdir(chartPath, 0o755))

					addFileToDirectory(t, chartPath, "Chart.yaml", "some content")

					temp, err := os.CreateTemp(t.TempDir(), "")
					require.NoError(t, err)
					defer temp.Close()
					_, err = files.Tgz(parent, nil, nil, temp)
					require.NoError(t, err)
					_, err = temp.Seek(0, io.SeekStart)
					require.NoError(t, err)
					all, err := io.ReadAll(temp)

					require.NoError(t, err)

					return generateManifest(t, store, layerConf{content.NewDescriptorFromBytes("application/vnd.cncf.helm.chart.content.v1.tar+gzip", all), all})
				},
				postValidationFunc: func(_, path string, _ Client, _ fields, _ args) {
					tempDir, err := files.CreateTempDir(os.TempDir())
					defer os.RemoveAll(tempDir)
					require.NoError(t, err)
					chartDir, err := os.ReadDir(path)
					require.NoError(t, err)
					require.Len(t, chartDir, 1)
					require.Equal(t, "Chart.yaml", chartDir[0].Name())
					chartYaml, err := os.Open(filepath.Join(path, chartDir[0].Name()))
					require.NoError(t, err)
					contents, err := io.ReadAll(chartYaml)
					require.NoError(t, err)
					require.Equal(t, "some content", string(contents))
				},
				manifestMaxExtractedSize:        10000,
				disableManifestMaxExtractedSize: false,
			},
		},
		{
			name: "extraction with standard gzip layer",
			fields: fields{
				allowedMediaTypes: []string{imagev1.MediaTypeImageLayerGzip},
			},
			args: args{
				digestFunc: func(store *memory.Store) string {
					layerBlob := createGzippedTarWithContent(t, "foo.yaml", "some content")
					return generateManifest(t, store, layerConf{content.NewDescriptorFromBytes(imagev1.MediaTypeImageLayerGzip, layerBlob), layerBlob})
				},
				postValidationFunc: func(_, path string, _ Client, _ fields, _ args) {
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
			name: "extraction with docker rootfs tar.gzip layer",
			fields: fields{
				allowedMediaTypes: []string{"application/vnd.docker.image.rootfs.diff.tar.gzip"},
			},
			args: args{
				digestFunc: func(store *memory.Store) string {
					layerBlob := createGzippedTarWithContent(t, "foo.yaml", "some content")
					return generateManifest(t, store, layerConf{content.NewDescriptorFromBytes("application/vnd.docker.image.rootfs.diff.tar.gzip", layerBlob), layerBlob})
				},
				postValidationFunc: func(_, path string, _ Client, _ fields, _ args) {
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
				allowedMediaTypes: []string{imagev1.MediaTypeImageLayerGzip},
			},
			args: args{
				digestFunc: func(store *memory.Store) string {
					layerBlob := createGzippedTarWithContent(t, "foo.yaml", "some content")
					return generateManifest(t, store, layerConf{content.NewDescriptorFromBytes(imagev1.MediaTypeImageLayerGzip, layerBlob), layerBlob})
				},
				manifestMaxExtractedSize:        1000,
				disableManifestMaxExtractedSize: false,
				postValidationFunc: func(sha string, _ string, _ Client, fields fields, args args) {
					store := memory.New()
					c := newClientWithLock(fields.repoURL, globalLock, store, fields.tagsFunc, func(_ context.Context) error {
						return nil
					}, fields.allowedMediaTypes,
						WithImagePaths(cacheDir),
						WithManifestMaxExtractedSize(args.manifestMaxExtractedSize),
						WithDisableManifestMaxExtractedSize(args.disableManifestMaxExtractedSize),
						WithEventHandlers(fakeEventHandlers(t, fields.repoURL)))
					_, gotCloser, err := c.Extract(t.Context(), sha)
					require.NoError(t, err)
					require.NoError(t, gotCloser.Close())
				},
			},
		},
		{
			name: "helm chart with multiple layers (provenance + chart content) should succeed",
			fields: fields{
				allowedMediaTypes: []string{"application/vnd.cncf.helm.chart.content.v1.tar+gzip"},
			},
			args: args{
				digestFunc: func(store *memory.Store) string {
					chartDir := t.TempDir()
					chartName := "mychart"

					parent := filepath.Join(chartDir, "parent")
					require.NoError(t, os.Mkdir(parent, 0o755))

					chartPath := filepath.Join(parent, chartName)
					require.NoError(t, os.Mkdir(chartPath, 0o755))

					addFileToDirectory(t, chartPath, "Chart.yaml", "helm chart content")

					temp, err := os.CreateTemp(t.TempDir(), "")
					require.NoError(t, err)
					defer temp.Close()
					_, err = files.Tgz(parent, nil, nil, temp)
					require.NoError(t, err)
					_, err = temp.Seek(0, io.SeekStart)
					require.NoError(t, err)
					chartBlob, err := io.ReadAll(temp)
					require.NoError(t, err)

					// Create provenance layer
					provenanceBlob := []byte("provenance data")

					return generateManifestWithConfig(t, store, "application/vnd.cncf.helm.config.v1+json",
						layerConf{content.NewDescriptorFromBytes("application/vnd.cncf.helm.chart.content.v1.tar+gzip", chartBlob), chartBlob},
						layerConf{content.NewDescriptorFromBytes("application/vnd.cncf.helm.chart.provenance.v1.prov", provenanceBlob), provenanceBlob})
				},
				postValidationFunc: func(_, path string, _ Client, _ fields, _ args) {
					// Verify only chart content was extracted, not provenance
					chartDir, err := os.ReadDir(path)
					require.NoError(t, err)
					require.Len(t, chartDir, 1)
					require.Equal(t, "Chart.yaml", chartDir[0].Name())

					chartYaml, err := os.Open(filepath.Join(path, chartDir[0].Name()))
					require.NoError(t, err)
					contents, err := io.ReadAll(chartYaml)
					require.NoError(t, err)
					require.Equal(t, "helm chart content", string(contents))
				},
				manifestMaxExtractedSize:        10000,
				disableManifestMaxExtractedSize: false,
			},
		},
		{
			name: "helm chart with multiple layers (attestation + provenance + chart content) should succeed",
			fields: fields{
				allowedMediaTypes: []string{"application/vnd.cncf.helm.chart.content.v1.tar+gzip"},
			},
			args: args{
				digestFunc: func(store *memory.Store) string {
					chartDir := t.TempDir()
					chartName := "mychart"

					parent := filepath.Join(chartDir, "parent")
					require.NoError(t, os.Mkdir(parent, 0o755))

					chartPath := filepath.Join(parent, chartName)
					require.NoError(t, os.Mkdir(chartPath, 0o755))

					addFileToDirectory(t, chartPath, "Chart.yaml", "multi-layer chart")
					addFileToDirectory(t, chartPath, "values.yaml", "key: value")

					temp, err := os.CreateTemp(t.TempDir(), "")
					require.NoError(t, err)
					defer temp.Close()
					_, err = files.Tgz(parent, nil, nil, temp)
					require.NoError(t, err)
					_, err = temp.Seek(0, io.SeekStart)
					require.NoError(t, err)
					chartBlob, err := io.ReadAll(temp)
					require.NoError(t, err)

					// Create multiple non-content layers
					attestationBlob := []byte("attestation data")
					provenanceBlob := []byte("provenance data")

					return generateManifestWithConfig(t, store, "application/vnd.cncf.helm.config.v1+json",
						layerConf{content.NewDescriptorFromBytes("application/vnd.in-toto+json", attestationBlob), attestationBlob},
						layerConf{content.NewDescriptorFromBytes("application/vnd.cncf.helm.chart.content.v1.tar+gzip", chartBlob), chartBlob},
						layerConf{content.NewDescriptorFromBytes("application/vnd.cncf.helm.chart.provenance.v1.prov", provenanceBlob), provenanceBlob})
				},
				postValidationFunc: func(_, path string, _ Client, _ fields, _ args) {
					// Verify only chart content was extracted
					chartDir, err := os.ReadDir(path)
					require.NoError(t, err)
					require.Len(t, chartDir, 2) // Chart.yaml and values.yaml

					files := make(map[string]bool)
					for _, f := range chartDir {
						files[f.Name()] = true
					}
					require.True(t, files["Chart.yaml"])
					require.True(t, files["values.yaml"])

					// Ensure no provenance or attestation files were extracted
					require.False(t, files["provenance"])
					require.False(t, files["attestation"])
				},
				manifestMaxExtractedSize:        10000,
				disableManifestMaxExtractedSize: false,
			},
		},
		{
			name: "helm chart with only provenance layer should fail (no chart content)",
			fields: fields{
				allowedMediaTypes: []string{"application/vnd.cncf.helm.chart.content.v1.tar+gzip"},
			},
			args: args{
				digestFunc: func(store *memory.Store) string {
					provenanceBlob := []byte("provenance data")
					return generateManifestWithConfig(t, store, "application/vnd.cncf.helm.config.v1+json",
						layerConf{content.NewDescriptorFromBytes("application/vnd.cncf.helm.chart.provenance.v1.prov", provenanceBlob), provenanceBlob})
				},
				manifestMaxExtractedSize:        1000,
				disableManifestMaxExtractedSize: false,
			},
			expectedError: errors.New("expected only a single oci content layer, got 0"),
		},
		{
			name: "non-helm OCI with multiple content layers should still fail",
			fields: fields{
				allowedMediaTypes: []string{imagev1.MediaTypeImageLayerGzip},
			},
			args: args{
				digestFunc: func(store *memory.Store) string {
					layerBlob1 := createGzippedTarWithContent(t, "file1.yaml", "content1")
					layerBlob2 := createGzippedTarWithContent(t, "file2.yaml", "content2")
					// Using standard image config, not Helm config
					return generateManifest(t, store,
						layerConf{content.NewDescriptorFromBytes(imagev1.MediaTypeImageLayerGzip, layerBlob1), layerBlob1},
						layerConf{content.NewDescriptorFromBytes(imagev1.MediaTypeImageLayerGzip, layerBlob2), layerBlob2})
				},
				manifestMaxExtractedSize:        1000,
				disableManifestMaxExtractedSize: false,
			},
			expectedError: errors.New("expected only a single oci content layer, got 2"),
		},
		{
			name: "helm chart with extra content layer should succeed and ignore extra layer",
			fields: fields{
				allowedMediaTypes: []string{"application/vnd.cncf.helm.chart.content.v1.tar+gzip", imagev1.MediaTypeImageLayerGzip},
			},
			args: args{
				digestFunc: func(store *memory.Store) string {
					chartDir := t.TempDir()
					chartName := "mychart"

					parent := filepath.Join(chartDir, "parent")
					require.NoError(t, os.Mkdir(parent, 0o755))

					chartPath := filepath.Join(parent, chartName)
					require.NoError(t, os.Mkdir(chartPath, 0o755))

					addFileToDirectory(t, chartPath, "Chart.yaml", "chart with extra docker layer")

					temp, err := os.CreateTemp(t.TempDir(), "")
					require.NoError(t, err)
					defer temp.Close()
					_, err = files.Tgz(parent, nil, nil, temp)
					require.NoError(t, err)
					_, err = temp.Seek(0, io.SeekStart)
					require.NoError(t, err)
					chartBlob, err := io.ReadAll(temp)
					require.NoError(t, err)

					// Extra OCI layer that Docker/some registries add
					extraLayerBlob := createGzippedTarWithContent(t, "extra.txt", "extra layer content")

					// Helm chart with proper Helm content layer + extra OCI layer that should be ignored
					return generateManifestWithConfig(t, store, "application/vnd.cncf.helm.config.v1+json",
						layerConf{content.NewDescriptorFromBytes("application/vnd.cncf.helm.chart.content.v1.tar+gzip", chartBlob), chartBlob},
						layerConf{content.NewDescriptorFromBytes(imagev1.MediaTypeImageLayerGzip, extraLayerBlob), extraLayerBlob})
				},
				postValidationFunc: func(_, path string, _ Client, _ fields, _ args) {
					// Verify only Helm chart content was extracted, not the extra OCI layer
					chartDir, err := os.ReadDir(path)
					require.NoError(t, err)
					require.Len(t, chartDir, 1)
					require.Equal(t, "Chart.yaml", chartDir[0].Name())

					chartYaml, err := os.Open(filepath.Join(path, chartDir[0].Name()))
					require.NoError(t, err)
					contents, err := io.ReadAll(chartYaml)
					require.NoError(t, err)
					require.Equal(t, "chart with extra docker layer", string(contents))
				},
				manifestMaxExtractedSize:        10000,
				disableManifestMaxExtractedSize: false,
			},
		},
		{
			name: "helm chart with extra OCI layer + provenance should extract only helm chart content",
			fields: fields{
				allowedMediaTypes: []string{"application/vnd.cncf.helm.chart.content.v1.tar+gzip", imagev1.MediaTypeImageLayerGzip},
			},
			args: args{
				digestFunc: func(store *memory.Store) string {
					chartDir := t.TempDir()
					chartName := "mychart"

					parent := filepath.Join(chartDir, "parent")
					require.NoError(t, os.Mkdir(parent, 0o755))

					chartPath := filepath.Join(parent, chartName)
					require.NoError(t, os.Mkdir(chartPath, 0o755))

					templatesPath := filepath.Join(chartPath, "templates")
					require.NoError(t, os.Mkdir(templatesPath, 0o755))

					addFileToDirectory(t, chartPath, "Chart.yaml", "multi-layer helm chart")
					addFileToDirectory(t, templatesPath, "deployment.yaml", "apiVersion: apps/v1")

					temp, err := os.CreateTemp(t.TempDir(), "")
					require.NoError(t, err)
					defer temp.Close()
					_, err = files.Tgz(parent, nil, nil, temp)
					require.NoError(t, err)
					_, err = temp.Seek(0, io.SeekStart)
					require.NoError(t, err)
					chartBlob, err := io.ReadAll(temp)
					require.NoError(t, err)

					provenanceBlob := []byte("provenance data")
					extraLayerBlob := createGzippedTarWithContent(t, "extra.txt", "extra oci layer")

					// Helm chart with: Helm content layer + extra OCI layer + provenance
					// Only the Helm content layer should be extracted
					return generateManifestWithConfig(t, store, "application/vnd.cncf.helm.config.v1+json",
						layerConf{content.NewDescriptorFromBytes("application/vnd.cncf.helm.chart.content.v1.tar+gzip", chartBlob), chartBlob},
						layerConf{content.NewDescriptorFromBytes(imagev1.MediaTypeImageLayerGzip, extraLayerBlob), extraLayerBlob},
						layerConf{content.NewDescriptorFromBytes("application/vnd.cncf.helm.chart.provenance.v1.prov", provenanceBlob), provenanceBlob})
				},
				postValidationFunc: func(_, path string, _ Client, _ fields, _ args) {
					// Verify only Helm chart content was extracted
					entries, err := os.ReadDir(path)
					require.NoError(t, err)
					require.Len(t, entries, 2) // Chart.yaml and templates dir

					files := make(map[string]bool)
					for _, e := range entries {
						files[e.Name()] = true
					}
					require.True(t, files["Chart.yaml"])
					require.True(t, files["templates"])

					// Verify Chart.yaml content
					chartYaml, err := os.ReadFile(filepath.Join(path, "Chart.yaml"))
					require.NoError(t, err)
					require.YAMLEq(t, "multi-layer helm chart", string(chartYaml))

					// Verify templates/deployment.yaml exists
					deploymentYaml, err := os.ReadFile(filepath.Join(path, "templates", "deployment.yaml"))
					require.NoError(t, err)
					require.YAMLEq(t, "apiVersion: apps/v1", string(deploymentYaml))

					// Ensure extra OCI layer and provenance were not extracted
					require.False(t, files["extra.txt"])
					require.False(t, files["provenance"])
				},
				manifestMaxExtractedSize:        10000,
				disableManifestMaxExtractedSize: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := memory.New()
			sha := tt.args.digestFunc(store)

			c := newClientWithLock(tt.fields.repoURL, globalLock, store, tt.fields.tagsFunc, func(_ context.Context) error {
				return nil
			}, tt.fields.allowedMediaTypes,
				WithImagePaths(cacheDir),
				WithManifestMaxExtractedSize(tt.args.manifestMaxExtractedSize),
				WithDisableManifestMaxExtractedSize(tt.args.disableManifestMaxExtractedSize),
				WithEventHandlers(fakeEventHandlers(t, tt.fields.repoURL)))
			path, gotCloser, err := c.Extract(t.Context(), sha)

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
	descriptor := imagev1.Descriptor{
		MediaType: "",
		Digest:    digest.FromBytes(data),
	}
	require.NoError(t, store.Push(t.Context(), descriptor, bytes.NewReader(data)))
	require.NoError(t, store.Tag(t.Context(), descriptor, "latest"))
	require.NoError(t, store.Tag(t.Context(), descriptor, "1.2.0"))
	require.NoError(t, store.Tag(t.Context(), descriptor, "v1.2.0"))
	require.NoError(t, store.Tag(t.Context(), descriptor, descriptor.Digest.String()))

	type fields struct {
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
				return []string{}, errors.New("this should not be invoked")
			}},
			expectedDigest: descriptor.Digest.String(),
		},
		{
			name:     "no matching version for constraint",
			revision: "^3.0.0",
			fields: fields{repo: store, tagsFunc: func(context.Context, string) (tags []string, err error) {
				return []string{"1.0.0", "1.1.0", "1.2.0", "2.0.0"}, nil
			}},
			expectedError: errors.New("no version for constraints: version matching constraint not found in 4 tags"),
		},
		{
			name:     "error fetching tags",
			revision: "^1.0.0",
			fields: fields{repo: store, tagsFunc: func(context.Context, string) (tags []string, err error) {
				return []string{}, errors.New("some random error")
			}},
			expectedError: errors.New("error fetching tags: failed to get tags: some random error"),
		},
		{
			name:     "error resolving digest",
			revision: "sha256:abc123",
			fields: fields{repo: store, tagsFunc: func(context.Context, string) (tags []string, err error) {
				return []string{"1.0.0", "1.1.0", "1.2.0", "2.0.0"}, nil
			}},
			expectedError: errors.New("cannot get digest for revision sha256:abc123: sha256:abc123: not found"),
		},
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
			expectedError: errors.New("no version for constraints: version matching constraint not found in 4 tags"),
		},
		{
			name:     "resolve explicit tag",
			revision: "v1.2.0",
			fields: fields{repo: store, tagsFunc: func(context.Context, string) (tags []string, err error) {
				return []string{}, errors.New("this should not be invoked")
			}},
			expectedError:  nil,
			expectedDigest: descriptor.Digest.String(),
		},
		{
			name:     "resolve with empty tag list",
			revision: "^1.0.0",
			fields: fields{repo: store, tagsFunc: func(context.Context, string) (tags []string, err error) {
				return []string{}, nil
			}},
			expectedError: errors.New("no version for constraints: version matching constraint not found in 0 tags"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newClientWithLock(tt.fields.repoURL, globalLock, tt.fields.repo, tt.fields.tagsFunc, func(_ context.Context) error {
				return nil
			}, tt.fields.allowedMediaTypes, WithEventHandlers(fakeEventHandlers(t, tt.fields.repoURL)))
			got, err := c.ResolveRevision(t.Context(), tt.revision, tt.noCache)
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

func TestFetchHelmChartAndProvenance_ChartLayerNotFound(t *testing.T) {
	store := memory.New()
	provenanceBlob := []byte("-----BEGIN PGP SIGNED MESSAGE-----\nprovenance\n-----BEGIN PGP SIGNATURE-----\n-----END PGP SIGNATURE-----")
	digest := generateManifestWithConfig(t, store, helmOCIConfigType,
		layerConf{content.NewDescriptorFromBytes(helmOCIProvType, provenanceBlob), provenanceBlob})

	cacheDir := utilio.NewRandomizedTempPaths(t.TempDir())
	c := newClientWithLock("ghcr.io/test/charts", globalLock, store, func(context.Context, string) ([]string, error) { return nil, nil },
		func(context.Context) error { return nil }, nil,
		WithImagePaths(cacheDir))

	chartContent, provContent, chartFilename, err := c.FetchHelmChartAndProvenance(t.Context(), digest)
	require.Error(t, err)
	require.Nil(t, chartContent)
	require.Nil(t, provContent)
	require.Empty(t, chartFilename)
	require.Contains(t, err.Error(), "helm chart content layer not found")
}

func TestFetchHelmChartAndProvenance_MultipleChartLayers(t *testing.T) {
	store := memory.New()
	chartBlob := createGzippedTarWithContent(t, "Chart.yaml", "chart content")
	chartBlob2 := createGzippedTarWithContent(t, "values.yaml", "values content")
	digest := generateManifestWithConfig(t, store, helmOCIConfigType,
		layerConf{content.NewDescriptorFromBytes(helmOCILayerType, chartBlob), chartBlob},
		layerConf{content.NewDescriptorFromBytes(helmOCILayerType, chartBlob2), chartBlob2})

	cacheDir := utilio.NewRandomizedTempPaths(t.TempDir())
	c := newClientWithLock("ghcr.io/test/charts", globalLock, store, func(context.Context, string) ([]string, error) { return nil, nil },
		func(context.Context) error { return nil }, nil,
		WithImagePaths(cacheDir))

	chartContent, provContent, chartFilename, err := c.FetchHelmChartAndProvenance(t.Context(), digest)
	require.Error(t, err)
	require.Nil(t, chartContent)
	require.Nil(t, provContent)
	require.Empty(t, chartFilename)
	require.Contains(t, err.Error(), "expected a single helm chart content layer, found multiple")
}

func TestFetchHelmChartAndProvenance_Success(t *testing.T) {
	store := memory.New()
	chartBlob := createGzippedTarWithContent(t, "Chart.yaml", "helm chart content")
	provBlob := []byte("-----BEGIN PGP SIGNED MESSAGE-----\nHash: SHA256\n\nmychart-1.0.0.tgz: sha256:abc123\n-----BEGIN PGP SIGNATURE-----\n-----END PGP SIGNATURE-----")
	chartDesc := content.NewDescriptorFromBytes(helmOCILayerType, chartBlob)
	chartDesc.Annotations = map[string]string{"org.opencontainers.image.title": "mychart-1.0.0.tgz"}
	digest := generateManifestWithConfig(t, store, helmOCIConfigType,
		layerConf{chartDesc, chartBlob},
		layerConf{content.NewDescriptorFromBytes(helmOCIProvType, provBlob), provBlob})

	cacheDir := utilio.NewRandomizedTempPaths(t.TempDir())
	c := newClientWithLock("ghcr.io/test/charts", globalLock, store, func(context.Context, string) ([]string, error) { return nil, nil },
		func(context.Context) error { return nil }, nil,
		WithImagePaths(cacheDir))

	chartContent, provContent, chartFilename, err := c.FetchHelmChartAndProvenance(t.Context(), digest)
	require.NoError(t, err)
	require.Equal(t, chartBlob, chartContent)
	require.Equal(t, provBlob, provContent)
	require.Equal(t, "mychart-1.0.0.tgz", chartFilename)
}

func TestFetchHelmChartAndProvenance_SuccessWithoutProvenance(t *testing.T) {
	store := memory.New()
	chartBlob := createGzippedTarWithContent(t, "Chart.yaml", "chart only")
	digest := generateManifestWithConfig(t, store, helmOCIConfigType,
		layerConf{content.NewDescriptorFromBytes(helmOCILayerType, chartBlob), chartBlob})

	cacheDir := utilio.NewRandomizedTempPaths(t.TempDir())
	c := newClientWithLock("ghcr.io/test/charts", globalLock, store, func(context.Context, string) ([]string, error) { return nil, nil },
		func(context.Context) error { return nil }, nil,
		WithImagePaths(cacheDir))

	chartContent, provContent, chartFilename, err := c.FetchHelmChartAndProvenance(t.Context(), digest)
	require.NoError(t, err)
	require.Equal(t, chartBlob, chartContent)
	require.Nil(t, provContent)
	require.Empty(t, chartFilename)
}

func fakeEventHandlers(t *testing.T, repoURL string) EventHandlers {
	t.Helper()
	return EventHandlers{
		OnExtract:         func(repo string) func() { return func() { require.Equal(t, repoURL, repo) } },
		OnResolveRevision: func(repo string) func() { return func() { require.Equal(t, repoURL, repo) } },
		OnDigestMetadata:  func(repo string) func() { return func() { require.Equal(t, repoURL, repo) } },
		OnTestRepo:        func(repo string) func() { return func() { require.Equal(t, repoURL, repo) } },
		OnGetTags:         func(repo string) func() { return func() { require.Equal(t, repoURL, repo) } },
		OnGetTagsFail: func(repo string) func() {
			return func() { require.Equal(t, repoURL, repo) }
		},
		OnExtractFail: func(repo string) func(revision string) {
			return func(_ string) { require.Equal(t, repoURL, repo) }
		},
		OnResolveRevisionFail: func(repo string) func(revision string) {
			return func(_ string) { require.Equal(t, repoURL, repo) }
		},
		OnDigestMetadataFail: func(repo string) func(revision string) {
			return func(_ string) { require.Equal(t, repoURL, repo) }
		},
	}
}
