package oci

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/opencontainers/go-digest"
	"oras.land/oras-go/v2/content/oci"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/Masterminds/semver/v3"
	"github.com/argoproj/pkg/sync"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v2/util/cache"
	argoio "github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/io/files"
	"github.com/argoproj/argo-cd/v2/util/proxy"

	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

var (
	globalLock = sync.NewKeyLock()
	indexLock  = sync.NewKeyLock()
)

var _ Client = &nativeOCIClient{}

type indexCache interface {
	SetHelmIndex(repo string, indexData []byte) error
	GetHelmIndex(repo string, indexData *[]byte) error
}

// Client is a generic oci client interface
type Client interface {
	ResolveRevision(ctx context.Context, revision string, noCache bool) (string, error)
	DigestMetadata(ctx context.Context, digest, project string) (*v1.Manifest, error)
	CleanCache(revision string, project string) error
	Extract(ctx context.Context, revision string, project string, manifestMaxExtractedSize int64, disableManifestMaxExtractedSize bool) (string, argoio.Closer, error)
	TestRepo(ctx context.Context) (bool, error)
}

type Creds struct {
	Username           string
	Password           string
	CAPath             string
	CertData           []byte
	KeyData            []byte
	InsecureSkipVerify bool
	InsecureHttpOnly   bool
}

type ClientOpts func(c *nativeOCIClient)

func WithIndexCache(indexCache indexCache) ClientOpts {
	return func(c *nativeOCIClient) {
		c.indexCache = indexCache
	}
}

func WithImagePaths(repoCachePaths argoio.TempPaths) ClientOpts {
	return func(c *nativeOCIClient) {
		c.repoCachePaths = repoCachePaths
	}
}

func NewClient(repoURL string, creds Creds, proxy, noProxy string, layerMediaTypes []string, opts ...ClientOpts) (Client, error) {
	return NewClientWithLock(repoURL, creds, globalLock, proxy, noProxy, layerMediaTypes, opts...)
}

func NewClientWithLock(repoURL string, creds Creds, repoLock sync.KeyLock, proxyUrl, noProxy string, layerMediaTypes []string, opts ...ClientOpts) (Client, error) {
	ociRepo := strings.TrimPrefix(repoURL, "oci://")
	repo, err := remote.NewRepository(ociRepo)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize repository: %w", err)
	}

	repo.PlainHTTP = creds.InsecureHttpOnly

	var tlsConf *tls.Config
	if !repo.PlainHTTP {
		tlsConf, err = newTLSConfig(creds)
		if err != nil {
			return nil, fmt.Errorf("failed setup tlsConfig: %w", err)
		}
	}

	client := &http.Client{Transport: &http.Transport{
		Proxy:             proxy.GetCallback(proxyUrl, noProxy),
		TLSClientConfig:   tlsConf,
		DisableKeepAlives: true,
	}}
	repo.Client = &auth.Client{
		Client: client,
		Cache:  nil,
		Credential: auth.StaticCredential(repo.Reference.Registry, auth.Credential{
			Username: creds.Username,
			Password: creds.Password,
		}),
	}

	return newClientWithLock(ociRepo, creds, repoLock, repo, func(ctx context.Context, last string) ([]string, error) {
		var t []string
		err := repo.Tags(ctx, last, func(tags []string) error {
			t = append(t, tags...)
			return nil
		})

		return t, err
	}, layerMediaTypes, opts...), nil
}

func newClientWithLock(repoURL string, creds Creds, repoLock sync.KeyLock, repo oras.ReadOnlyTarget, tagsFunc func(context.Context, string) ([]string, error), layerMediaTypes []string, opts ...ClientOpts) Client {
	c := &nativeOCIClient{
		creds:             creds,
		repoURL:           repoURL,
		repoLock:          repoLock,
		repo:              repo,
		tagsFunc:          tagsFunc,
		allowedMediaTypes: layerMediaTypes,
	}
	for i := range opts {
		opts[i](c)
	}
	return c
}

// nativeOCIClient implements Client interface using oras-go
type nativeOCIClient struct {
	creds             Creds
	repoURL           string
	repo              oras.ReadOnlyTarget
	tagsFunc          func(context.Context, string) ([]string, error)
	repoLock          sync.KeyLock
	indexCache        indexCache
	repoCachePaths    argoio.TempPaths
	allowedMediaTypes []string
}

// TestRepo verifies that the remote OCI repo can be connected to.
func (c *nativeOCIClient) TestRepo(ctx context.Context) (bool, error) {
	_, err := c.tagsFunc(ctx, "")
	return err == nil, err
}

func (c *nativeOCIClient) Extract(ctx context.Context, digest string, project string, manifestMaxExtractedSize int64, disableManifestMaxExtractedSize bool) (string, argoio.Closer, error) {
	cachedPath, err := c.getCachedPath(digest, project)
	if err != nil {
		return "", nil, err
	}

	c.repoLock.Lock(cachedPath)
	defer c.repoLock.Unlock(cachedPath)

	exists, err := fileExists(cachedPath)
	if err != nil {
		return "", nil, err
	}

	if !exists {
		ociManifest, err := getOCIManifest(ctx, digest, c.repo)
		if err != nil {
			return "", nil, err
		}

		if len(ociManifest.Layers) != 1 {
			return "", nil, fmt.Errorf("expected only a single oci layer, got %d", len(ociManifest.Layers))
		}

		if !slices.Contains(c.allowedMediaTypes, ociManifest.Layers[0].MediaType) {
			return "", nil, fmt.Errorf("oci layer media type %s is not in the list of allowed media types", ociManifest.Layers[0].MediaType)
		}

		err = saveCompressedImageToPath(ctx, digest, c.repo, cachedPath)
		if err != nil {
			return "", nil, err
		}
	}

	maxSize := manifestMaxExtractedSize
	if disableManifestMaxExtractedSize {
		maxSize = math.MaxInt64
	}

	manifestsDir, err := extractContentToManifestsDir(ctx, cachedPath, digest, maxSize, c.allowedMediaTypes)
	if err != nil {
		return manifestsDir, nil, err
	}

	return manifestsDir, argoio.NewCloser(func() error {
		return os.RemoveAll(manifestsDir)
	}), nil
}

func (c *nativeOCIClient) getCachedPath(version, project string) (string, error) {
	keyData, err := json.Marshal(map[string]string{"url": c.repoURL, "project": project, "version": version})
	if err != nil {
		return "", err
	}
	return c.repoCachePaths.GetPath(string(keyData))
}

// CleanCache is invoked on a hard-refresh or when the manifest cache has expired. This removes the OCI image from the cached path.
func (c *nativeOCIClient) CleanCache(revision, project string) error {
	cachePath, err := c.getCachedPath(revision, project)
	if err != nil {
		return err
	}
	return os.RemoveAll(cachePath)
}

// DigestMetadata extracts the OCI manifest for a given revision and returns it to the caller.
func (c *nativeOCIClient) DigestMetadata(ctx context.Context, digest, project string) (*v1.Manifest, error) {
	path, err := c.getCachedPath(digest, project)
	if err != nil {
		return nil, err
	}

	repo, err := oci.NewFromTar(ctx, path)
	if err != nil {
		return nil, err
	}

	return getOCIManifest(ctx, digest, repo)
}

func (c *nativeOCIClient) ResolveRevision(ctx context.Context, revision string, noCache bool) (string, error) {
	constraints, err := semver.NewConstraint(revision)
	if err == nil {
		tags, err := c.getTags(ctx, noCache)
		if err != nil {
			return "", fmt.Errorf("error fetching tags: %w", err)
		}
		version, err := tags.MaxVersion(constraints)
		if err != nil {
			return "", fmt.Errorf("no version for constraints: %w", err)
		}
		return c.resolveDigest(ctx, version.String())
	}

	return c.resolveDigest(ctx, revision)
}

func (c *nativeOCIClient) getTags(ctx context.Context, noCache bool) (*TagsList, error) {
	indexLock.Lock(c.repoURL)
	defer indexLock.Unlock(c.repoURL)

	var data []byte
	if !noCache && c.indexCache != nil {
		if err := c.indexCache.GetHelmIndex(c.repoURL, &data); err != nil && !errors.Is(err, cache.ErrCacheMiss) {
			log.Warnf("Failed to load index cache for repo: %s: %s", c.repoLock, err)
		}
	}

	tags := &TagsList{}
	if len(data) == 0 {
		start := time.Now()
		result, err := c.tagsFunc(ctx, "")
		if err != nil {
			return nil, fmt.Errorf("failed to get tags: %w", err)
		}

		for _, tag := range result {
			// By convention: Change underscore (_) back to plus (+) to get valid SemVer
			convertedTag := strings.ReplaceAll(tag, "_", "+")
			tags.Tags = append(tags.Tags, convertedTag)
		}

		log.WithFields(
			log.Fields{"seconds": time.Since(start).Seconds(), "repo": c.repoURL},
		).Info("took to get tags")

		if c.indexCache != nil {
			if err := c.indexCache.SetHelmIndex(c.repoURL, data); err != nil {
				log.Warnf("Failed to store tags list cache for repo: %s: %s", c.repoURL, err)
			}
		}
	} else {
		err := json.Unmarshal(data, tags)
		if err != nil {
			return nil, fmt.Errorf("failed to decode tags: %w", err)
		}
	}

	return tags, nil
}

// resolveDigest resolves a digest from a tag.
func (c *nativeOCIClient) resolveDigest(ctx context.Context, revision string) (string, error) {
	descriptor, err := c.repo.Resolve(ctx, revision)
	if err != nil {
		exists, _ := c.repo.Exists(ctx, v1.Descriptor{Digest: digest.Digest(revision)})
		if exists {
			return revision, nil
		}
		return "", fmt.Errorf("cannot get digest: %w", err)
	}

	return descriptor.Digest.String(), nil
}

func newTLSConfig(creds Creds) (*tls.Config, error) {
	tlsConfig := &tls.Config{InsecureSkipVerify: creds.InsecureSkipVerify}

	if creds.CAPath != "" {
		caData, err := os.ReadFile(creds.CAPath)
		if err != nil {
			return nil, err
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caData)
		tlsConfig.RootCAs = caCertPool
	}

	// If a client cert & key is provided then configure TLS config accordingly.
	if len(creds.CertData) > 0 && len(creds.KeyData) > 0 {
		cert, err := tls.X509KeyPair(creds.CertData, creds.KeyData)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	// nolint:staticcheck
	tlsConfig.BuildNameToCertificate()

	return tlsConfig, nil
}

func fileExists(filePath string) (bool, error) {
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		} else {
			return false, err
		}
	}
	return true, nil
}

func isHelmOCI(mediaType string) bool {
	return mediaType == "application/vnd.cncf.helm.chart.content.v1.tar+gzip"
}

func isCompressedLayer(mediaType string) bool {
	return strings.HasSuffix(mediaType, "tar+gzip") || strings.HasSuffix(mediaType, "tar")
}

func createTarFile(from, to string) error {
	f, err := os.Create(to)
	if err != nil {
		return err
	}

	_, err = files.Tar(from, nil, nil, f)
	if err != nil {
		_ = os.RemoveAll(to)
	}
	return f.Close()
}

// saveCompressedImageToPath downloads a remote OCI image on a given digest and stores it as a TAR file in cachedPath.
func saveCompressedImageToPath(ctx context.Context, digest string, repo oras.ReadOnlyTarget, cachedPath string) error {
	tempDir, err := files.CreateTempDir(os.TempDir())
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	store, err := oci.New(tempDir)
	if err != nil {
		return err
	}

	// Copy remote repo at the given digest to the scratch dir.
	_, err = oras.Copy(ctx, repo, digest, store, digest, oras.DefaultCopyOptions)
	if err != nil {
		return err
	}

	// Save contents to tar file
	return createTarFile(tempDir, cachedPath)
}

// extractContentToManifestsDir looks up a locally stored OCI image, and extracts the embedded compressed layer which contains
// K8s manifests to a temporary directory
func extractContentToManifestsDir(ctx context.Context, cachedPath, digest string, maxSize int64, allowedMediaTypes []string) (string, error) {
	manifestsDir, err := files.CreateTempDir(os.TempDir())
	if err != nil {
		return manifestsDir, err
	}

	ociReadOnlyStore, err := oci.NewFromTar(ctx, cachedPath)
	if err != nil {
		return manifestsDir, err
	}

	tempDir, err := files.CreateTempDir(os.TempDir())
	if err != nil {
		return manifestsDir, err
	}
	defer os.RemoveAll(tempDir)

	fs, err := newCompressedLayerFileStore(manifestsDir, tempDir, maxSize, allowedMediaTypes)
	if err != nil {
		return manifestsDir, err
	}
	defer fs.Close()

	// copies the whole artifact to the tempdir, here compressedLayerFileStore.Push will be called
	_, err = oras.Copy(ctx, ociReadOnlyStore, digest, fs, digest, oras.DefaultCopyOptions)
	return manifestsDir, err
}

type compressedLayerExtracterStore struct {
	*file.Store
	tempDir           string
	dest              string
	maxSize           int64
	allowedMediaTypes []string
}

func newCompressedLayerFileStore(dest, tempDir string, maxSize int64, allowedMediaTypes []string) (*compressedLayerExtracterStore, error) {
	f, err := file.New(tempDir)
	if err != nil {
		return nil, err
	}

	return &compressedLayerExtracterStore{f, tempDir, dest, maxSize, allowedMediaTypes}, nil
}

// Push looks in all the layers of an OCI image. Once it finds a layer that is compressed, it extracts the layer to a tempDir
// and then renames the temp dir to the directory where the repo-server expects to find k8s manifests.
func (s *compressedLayerExtracterStore) Push(ctx context.Context, desc v1.Descriptor, content io.Reader) error {
	if isCompressedLayer(desc.MediaType) {
		tempDir, err := files.CreateTempDir(os.TempDir())
		if err != nil {
			return err
		}
		defer os.RemoveAll(tempDir)

		if strings.HasSuffix(desc.MediaType, "tar+gzip") {
			err = files.Untgz(tempDir, content, s.maxSize, false)
		} else {
			err = files.Untar(tempDir, content, s.maxSize, false)
		}

		if err != nil {
			return fmt.Errorf("could not decompress layer: %w", err)
		}

		infos, err := os.ReadDir(tempDir)
		if err != nil {
			return err
		}

		if isHelmOCI(desc.MediaType) {
			// For a Helm chart we expect a single tarfile in the directory
			if len(infos) != 1 {
				return fmt.Errorf("expected 1 file, found %v", len(infos))
			}
		}

		if len(infos) == 1 && infos[0].IsDir() {
			// Here we assume that this is a directory which has been decompressed. We need to move the contents of
			// the dir into our intended destination.
			srcDir := filepath.Join(tempDir, infos[0].Name())
			return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
				if path != srcDir {
					// Calculate the relative path from srcDir
					relPath, err := filepath.Rel(srcDir, path)
					if err != nil {
						return err
					}

					dstPath := filepath.Join(s.dest, relPath)
					// Move the file by renaming it
					if d.IsDir() {
						info, err := d.Info()
						if err != nil {
							return err
						}

						return os.MkdirAll(dstPath, info.Mode())
					}

					return os.Rename(path, dstPath)
				}

				return nil
			})
		}

		err = os.Remove(s.dest)
		if err != nil {
			return err
		}

		// For any other OCI content, we assume that this should be rendered as-is
		return os.Rename(tempDir, s.dest)
	}

	return s.Store.Push(ctx, desc, content)
}

func getOCIManifest(ctx context.Context, digest string, repo oras.ReadOnlyTarget) (*v1.Manifest, error) {
	desc, err := repo.Resolve(ctx, digest)
	if err != nil {
		return nil, err
	}

	rc, err := repo.Fetch(ctx, desc)
	if err != nil {
		return nil, err
	}

	manifest := v1.Manifest{}
	decoder := json.NewDecoder(rc)
	err = decoder.Decode(&manifest)
	if err != nil {
		return nil, err
	}

	return &manifest, nil
}
