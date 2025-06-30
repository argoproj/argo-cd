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
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	securejoin "github.com/cyphar/filepath-securejoin"
	imagev1 "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content/oci"

	"github.com/argoproj/argo-cd/v3/util/versions"

	"github.com/argoproj/pkg/sync"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/util/cache"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
	"github.com/argoproj/argo-cd/v3/util/io/files"
	"github.com/argoproj/argo-cd/v3/util/proxy"

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

	// DigestMetadata retrieves an OCI manifest for a given digest.
	DigestMetadata(ctx context.Context, digest string) (*imagev1.Manifest, error)

	// CleanCache is invoked on a hard-refresh or when the manifest cache has expired. This removes the OCI image from
	// the cached path, which is looked up by the specified revision.
	CleanCache(revision string) error

	// Extract retrieves and unpacks the contents of an OCI image identified by the specified revision.
	// If successful, the extracted contents are extracted to a randomized tempdir.
	Extract(ctx context.Context, revision string) (string, utilio.Closer, error)

	// TestRepo verifies the connectivity and accessibility of the repository.
	TestRepo(ctx context.Context) (bool, error)

	// GetTags retrieves the list of tags for the repository.
	GetTags(ctx context.Context, noCache bool) ([]string, error)
}

type Creds struct {
	Username           string
	Password           string
	CAPath             string
	CertData           []byte
	KeyData            []byte
	InsecureSkipVerify bool
	InsecureHTTPOnly   bool
}

type ClientOpts func(c *nativeOCIClient)

func WithIndexCache(indexCache tagsCache) ClientOpts {
	return func(c *nativeOCIClient) {
		c.tagsCache = indexCache
	}
}

func WithImagePaths(repoCachePaths utilio.TempPaths) ClientOpts {
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

func NewClientWithLock(repoURL string, creds Creds, repoLock sync.KeyLock, proxyURL, noProxy string, layerMediaTypes []string, opts ...ClientOpts) (Client, error) {
	ociRepo := strings.TrimPrefix(repoURL, "oci://")
	repo, err := remote.NewRepository(ociRepo)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize repository: %w", err)
	}

	repo.PlainHTTP = creds.InsecureHTTPOnly

	var tlsConf *tls.Config
	if !repo.PlainHTTP {
		tlsConf, err = newTLSConfig(creds)
		if err != nil {
			return nil, fmt.Errorf("failed setup tlsConfig: %w", err)
		}
	}

	client := &http.Client{
		Transport: &http.Transport{
			Proxy:             proxy.GetCallback(proxyURL, noProxy),
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

	parsed, err := url.Parse(repoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse oci repo url: %w", err)
	}

	reg, err := remote.NewRegistry(parsed.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to setup registry config: %w", err)
	}
	reg.PlainHTTP = repo.PlainHTTP
	reg.Client = repo.Client
	return newClientWithLock(ociRepo, repoLock, repo, func(ctx context.Context, last string) ([]string, error) {
		var t []string

		err := repo.Tags(ctx, last, func(tags []string) error {
			t = append(t, tags...)
			return nil
		})

		return t, err
	}, reg.Ping, layerMediaTypes, opts...), nil
}

func newClientWithLock(repoURL string, repoLock sync.KeyLock, repo oras.ReadOnlyTarget, tagsFunc func(context.Context, string) ([]string, error), pingFunc func(ctx context.Context) error, layerMediaTypes []string, opts ...ClientOpts) Client {
	c := &nativeOCIClient{
		repoURL:           repoURL,
		repoLock:          repoLock,
		repo:              repo,
		tagsFunc:          tagsFunc,
		pingFunc:          pingFunc,
		allowedMediaTypes: layerMediaTypes,
	}
	for i := range opts {
		opts[i](c)
	}
	return c
}

// nativeOCIClient implements Client interface using oras-go
type nativeOCIClient struct {
	repoURL                         string
	repo                            oras.ReadOnlyTarget
	tagsFunc                        func(context.Context, string) ([]string, error)
	repoLock                        sync.KeyLock
	tagsCache                       tagsCache
	repoCachePaths                  utilio.TempPaths
	allowedMediaTypes               []string
	manifestMaxExtractedSize        int64
	disableManifestMaxExtractedSize bool
	pingFunc                        func(ctx context.Context) error
}

// TestRepo verifies that the remote OCI repo can be connected to.
func (c *nativeOCIClient) TestRepo(ctx context.Context) (bool, error) {
	err := c.pingFunc(ctx)
	return err == nil, err
}

func (c *nativeOCIClient) Extract(ctx context.Context, digest string) (string, utilio.Closer, error) {
	cachedPath, err := c.getCachedPath(digest)
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

	manifestsDir, err := extractContentToManifestsDir(ctx, cachedPath, digest, maxSize)
	if err != nil {
		return manifestsDir, nil, fmt.Errorf("cannot extract contents of oci image with revision %s: %w", digest, err)
	}

	return manifestsDir, utilio.NewCloser(func() error {
		return os.RemoveAll(manifestsDir)
	}), nil
}

func (c *nativeOCIClient) getCachedPath(version string) (string, error) {
	keyData, err := json.Marshal(map[string]string{"url": c.repoURL, "version": version})
	if err != nil {
		return "", err
	}
	return c.repoCachePaths.GetPath(string(keyData))
}

func (c *nativeOCIClient) CleanCache(revision string) error {
	cachePath, err := c.getCachedPath(revision)
	if err != nil {
		return fmt.Errorf("error cleaning oci path for revision %s: %w", revision, err)
	}
	return os.RemoveAll(cachePath)
}

// DigestMetadata extracts the OCI manifest for a given revision and returns it to the caller.
func (c *nativeOCIClient) DigestMetadata(ctx context.Context, digest string) (*imagev1.Manifest, error) {
	path, err := c.getCachedPath(digest)
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
	digest, err := c.resolveDigest(ctx, revision) // Lookup explicit revision
	if err != nil {
		// If the revision is not a semver constraint, just return the error
		if !versions.IsConstraint(revision) {
			return digest, err
		}

		tags, err := c.GetTags(ctx, noCache)
		if err != nil {
			return "", fmt.Errorf("error fetching tags: %w", err)
		}

		// Look to see if revision is a semver constraint
		version, err := versions.MaxVersion(revision, tags)
		if err != nil {
			return "", fmt.Errorf("no version for constraints: %w", err)
		}
		// Look up the digest for the resolved version
		return c.resolveDigest(ctx, version)
	}

	return digest, nil
}

func (c *nativeOCIClient) GetTags(ctx context.Context, noCache bool) ([]string, error) {
	indexLock.Lock(c.repoURL)
	defer indexLock.Unlock(c.repoURL)

	var data []byte
	if !noCache && c.tagsCache != nil {
		if err := c.tagsCache.GetOCITags(c.repoURL, &data); err != nil && !errors.Is(err, cache.ErrCacheMiss) {
			log.Warnf("Failed to load index cache for repo: %s: %s", c.repoLock, err)
		}
	}

	var tags []string
	if len(data) == 0 {
		start := time.Now()
		result, err := c.tagsFunc(ctx, "")
		if err != nil {
			return nil, fmt.Errorf("failed to get tags: %w", err)
		}

		for _, tag := range result {
			// By convention: Change underscore (_) back to plus (+) to get valid SemVer
			convertedTag := strings.ReplaceAll(tag, "_", "+")
			tags = append(tags, convertedTag)
		}

		log.WithFields(
			log.Fields{"seconds": time.Since(start).Seconds(), "repo": c.repoURL},
		).Info("took to get tags")

		if c.tagsCache != nil {
			if err := c.tagsCache.SetOCITags(c.repoURL, data); err != nil {
				log.Warnf("Failed to store tags list cache for repo: %s: %s", c.repoURL, err)
			}
		}
	} else if err := json.Unmarshal(data, &tags); err != nil {
		return nil, fmt.Errorf("failed to decode tags: %w", err)
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
	//nolint:staticcheck
	tlsConfig.BuildNameToCertificate()

	return tlsConfig, nil
}

func fileExists(filePath string) (bool, error) {
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func isCompressedLayer(mediaType string) bool {
	return strings.HasSuffix(mediaType, "tar+gzip") || strings.HasSuffix(mediaType, "tar")
}

func createTarFile(from, to string) error {
	f, err := os.Create(to)
	if err != nil {
		return err
	}
	if _, err = files.Tar(from, nil, nil, f); err != nil {
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
	if _, err = oras.Copy(ctx, repo, digest, store, digest, oras.DefaultCopyOptions); err != nil {
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
func extractContentToManifestsDir(ctx context.Context, cachedPath, digest string, maxSize int64) (string, error) {
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

	fs, err := newCompressedLayerFileStore(manifestsDir, tempDir, maxSize)
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
	dest    string
	maxSize int64
}

func newCompressedLayerFileStore(dest, tempDir string, maxSize int64) (*compressedLayerExtracterStore, error) {
	f, err := file.New(tempDir)
	if err != nil {
		return nil, err
	}

	return &compressedLayerExtracterStore{f, dest, maxSize}, nil
}

func isHelmOCI(mediaType string) bool {
	return mediaType == "application/vnd.cncf.helm.chart.content.v1.tar+gzip"
}

// Push looks in all the layers of an OCI image. Once it finds a layer that is compressed, it extracts the layer to a tempDir
// and then renames the temp dir to the directory where the repo-server expects to find k8s manifests.
func (s *compressedLayerExtracterStore) Push(ctx context.Context, desc imagev1.Descriptor, content io.Reader) error {
	if isCompressedLayer(desc.MediaType) {
		srcDir, err := files.CreateTempDir(os.TempDir())
		if err != nil {
			return err
		}
		defer os.RemoveAll(srcDir)

		if strings.HasSuffix(desc.MediaType, "tar+gzip") {
			err = files.Untgz(srcDir, content, s.maxSize, false)
		} else {
			err = files.Untar(srcDir, content, s.maxSize, false)
		}

		if err != nil {
			return fmt.Errorf("could not decompress layer: %w", err)
		}

		if isHelmOCI(desc.MediaType) {
			infos, err := os.ReadDir(srcDir)
			if err != nil {
				return err
			}

			// For a Helm chart we expect a single directory
			if len(infos) != 1 || !infos[0].IsDir() {
				return fmt.Errorf("expected 1 directory, found %v", len(infos))
			}

			// For Helm charts, we will move the contents of the unpacked directory to the root of its final destination
			srcDir, err = securejoin.SecureJoin(srcDir, infos[0].Name())
			if err != nil {
				return err
			}
		}

		return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, _ error) error {
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

	return s.Store.Push(ctx, desc, content)
}

func getOCIManifest(ctx context.Context, digest string, repo oras.ReadOnlyTarget) (*imagev1.Manifest, error) {
	desc, err := repo.Resolve(ctx, digest)
	if err != nil {
		return nil, fmt.Errorf("error resolving oci repo from digest, %w", err)
	}

	rc, err := repo.Fetch(ctx, desc)
	if err != nil {
		return nil, fmt.Errorf("error fetching oci manifest for digest %s: %w", digest, err)
	}

	manifest := imagev1.Manifest{}
	decoder := json.NewDecoder(rc)
	if err = decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("error decoding oci manifest for digest %s: %w", digest, err)
	}

	return &manifest, nil
}
