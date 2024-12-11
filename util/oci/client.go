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
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	securejoin "github.com/cyphar/filepath-securejoin"
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

type tagsCache interface {
	SetOCITags(repo string, indexData []byte) error
	GetOCITags(repo string, indexData *[]byte) error
}

// Client is a generic OCI client interface that provides methods for interacting with an OCI (Open Container Initiative) registry.
type Client interface {
	// ResolveRevision resolves a tag, digest, or semantic version constraint to a concrete digest.
	// If noCache is true, the resolution bypasses the local tags cache and queries the remote registry.
	// If the revision is already a digest, it is returned as-is.
	ResolveRevision(ctx context.Context, revision string, noCache bool) (string, error)

	// DigestMetadata retrieves an OCI manifest for a given digest and project.
	DigestMetadata(ctx context.Context, digest, project string) (*v1.Manifest, error)

	// CleanCache is invoked on a hard-refresh or when the manifest cache has expired. This removes the OCI image from
	// the cached path, which is looked up by the specified revision and project.
	CleanCache(revision string, project string) error

	// Extract retrieves and unpacks the contents of an OCI image identified by the specified revision and project.
	// If successful, the extracted contents are extracted to a randomized tempdir.
	Extract(ctx context.Context, revision string, project string) (string, argoio.Closer, error)

	// TestRepo verifies the connectivity and accessibility of the repository.
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

func WithIndexCache(indexCache tagsCache) ClientOpts {
	return func(c *nativeOCIClient) {
		c.tagsCache = indexCache
	}
}

func WithImagePaths(repoCachePaths argoio.TempPaths) ClientOpts {
	return func(c *nativeOCIClient) {
		c.repoCachePaths = repoCachePaths
	}
}

func WithManifestMaxExtractedSize(manifestMaxExtractedSize int64) ClientOpts {
	return func(c *nativeOCIClient) {
		c.manifestMaxExtractedSize = manifestMaxExtractedSize
	}
}

func WithDisableManifestMaxExtractedSize(disableManifestMaxExtractedSize bool) ClientOpts {
	return func(c *nativeOCIClient) {
		c.disableManifestMaxExtractedSize = disableManifestMaxExtractedSize
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

	client := &http.Client{
		Transport: &http.Transport{
			Proxy:             proxy.GetCallback(proxyUrl, noProxy),
			TLSClientConfig:   tlsConf,
			DisableKeepAlives: true,
		},
		/*
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return errors.New("redirects are not allowed")
			},
		*/
	}
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
	creds                           Creds
	repoURL                         string
	repo                            oras.ReadOnlyTarget
	tagsFunc                        func(context.Context, string) ([]string, error)
	repoLock                        sync.KeyLock
	tagsCache                       tagsCache
	repoCachePaths                  argoio.TempPaths
	allowedMediaTypes               []string
	manifestMaxExtractedSize        int64
	disableManifestMaxExtractedSize bool
}

// TestRepo verifies that the remote OCI repo can be connected to.
func (c *nativeOCIClient) TestRepo(ctx context.Context) (bool, error) {
	_, err := c.tagsFunc(ctx, "")
	return err == nil, err
}

func (c *nativeOCIClient) Extract(ctx context.Context, digest string, project string) (string, argoio.Closer, error) {
	cachedPath, err := c.getCachedPath(digest, project)
	if err != nil {
		return "", nil, fmt.Errorf("error getting oci path for digest %s: %w", digest, err)
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
			return "", nil, fmt.Errorf("could not save oci digest %s: %w", digest, err)
		}
	}

	maxSize := c.manifestMaxExtractedSize
	if c.disableManifestMaxExtractedSize {
		maxSize = math.MaxInt64
	}

	manifestsDir, err := extractContentToManifestsDir(ctx, cachedPath, digest, maxSize, c.allowedMediaTypes)
	if err != nil {
		return manifestsDir, nil, fmt.Errorf("cannot extract contents of oci image with revision %s: %w", digest, err)
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

func (c *nativeOCIClient) CleanCache(revision, project string) error {
	cachePath, err := c.getCachedPath(revision, project)
	if err != nil {
		return fmt.Errorf("error cleaning oci path for revision %s: %w", revision, err)
	}
	return os.RemoveAll(cachePath)
}

// DigestMetadata extracts the OCI manifest for a given revision and returns it to the caller.
func (c *nativeOCIClient) DigestMetadata(ctx context.Context, digest, project string) (*v1.Manifest, error) {
	path, err := c.getCachedPath(digest, project)
	if err != nil {
		return nil, fmt.Errorf("error fetching oci metadata path for digest %s: %w", digest, err)
	}

	repo, err := oci.NewFromTar(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("error extracting oci image for digest %s: %w", digest, err)
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
	if !noCache && c.tagsCache != nil {
		if err := c.tagsCache.GetOCITags(c.repoURL, &data); err != nil && !errors.Is(err, cache.ErrCacheMiss) {
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

		if c.tagsCache != nil {
			if err := c.tagsCache.SetOCITags(c.repoURL, data); err != nil {
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
		return "", fmt.Errorf("cannot get digest for revision %s: %w", revision, err)
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

	// Remove redundant ingest folder; this is an artifact from the oras.Copy call above
	err = os.RemoveAll(path.Join(tempDir, "ingest"))
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
			srcDir, err := securejoin.SecureJoin(tempDir, infos[0].Name())
			if err != nil {
				return err
			}

			return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
				if path != srcDir {
					// Calculate the relative path from srcDir
					relPath, err := filepath.Rel(srcDir, path)
					if err != nil {
						return err
					}

					dstPath, err := securejoin.SecureJoin(s.dest, relPath)
					if err != nil {
						return err
					}

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
		return nil, fmt.Errorf("error resolving oci repo from digest %s: %w", digest, err)
	}

	rc, err := repo.Fetch(ctx, desc)
	if err != nil {
		return nil, fmt.Errorf("error fetching oci manifest for digest %s: %w", digest, err)
	}

	manifest := v1.Manifest{}
	decoder := json.NewDecoder(rc)
	err = decoder.Decode(&manifest)
	if err != nil {
		return nil, fmt.Errorf("error decoding oci manifest for digest %s: %w", digest, err)
	}

	return &manifest, nil
}
