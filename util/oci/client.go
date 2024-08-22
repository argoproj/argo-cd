package oci

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/argoproj/argo-cd/v2/util/cache"
	argoio "github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/io/files"
	"github.com/argoproj/argo-cd/v2/util/proxy"
	"github.com/argoproj/pkg/sync"
	log "github.com/sirupsen/logrus"

	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/content/oci"
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
	GetTags(ctx context.Context, noCache bool) (*TagsList, error)
	ResolveDigest(ctx context.Context, revision string) (string, error)
	ResolveRevision(ctx context.Context, revision string, noCache bool) (string, error)
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

func WithChartPaths(repoCachePaths argoio.TempPaths) ClientOpts {
	return func(c *nativeOCIClient) {
		c.repoCachePaths = repoCachePaths
	}
}

func NewClient(repoURL string, creds Creds, proxy, noProxy string, opts ...ClientOpts) (Client, error) {
	return NewClientWithLock(repoURL, creds, globalLock, proxy, noProxy, opts...)
}

func NewClientWithLock(repoURL string, creds Creds, repoLock sync.KeyLock, proxyUrl, noProxy string, opts ...ClientOpts) (Client, error) {
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

	ociRegistry := repo.Reference.Registry

	client := &http.Client{Transport: &http.Transport{
		Proxy:             proxy.GetCallback(proxyUrl, noProxy),
		TLSClientConfig:   tlsConf,
		DisableKeepAlives: true,
	}}
	repo.Client = &auth.Client{
		Client: client,
		Cache:  nil,
		Credential: auth.StaticCredential(ociRegistry, auth.Credential{
			Username: creds.Username,
			Password: creds.Password,
		}),
	}

	c := &nativeOCIClient{
		creds:    creds,
		repoURL:  ociRepo,
		proxy:    proxyUrl,
		repoLock: repoLock,
		repo:     repo,
	}
	for i := range opts {
		opts[i](c)
	}
	return c, nil
}

// nativeOCIClient implements Client interface using oras-go
type nativeOCIClient struct {
	creds          Creds
	repoURL        string
	proxy          string
	repo           *remote.Repository
	repoLock       sync.KeyLock
	indexCache     indexCache
	repoCachePaths argoio.TempPaths
}

func (c *nativeOCIClient) TestRepo(ctx context.Context) (bool, error) {
	err := c.repo.Tags(ctx, "", func(tags []string) error {
		return nil
	})
	return err == nil, err
}

func (c *nativeOCIClient) Extract(ctx context.Context, digest string, project string, manifestMaxExtractedSize int64, disableManifestMaxExtractedSize bool) (string, argoio.Closer, error) {
	cachedPath, err := c.getCachedPath(digest, project)
	if err != nil {
		return "", nil, err
	}

	c.repoLock.Lock(cachedPath)
	defer c.repoLock.Unlock(cachedPath)

	exists, err := fileExist(cachedPath)
	if err != nil {
		return "", nil, err
	}

	if !exists {
		// Create scratch dir for the OCI store. This will be put into a TAR archive.
		tempDir, err := files.CreateTempDir(os.TempDir())
		if err != nil {
			return "", nil, err
		}
		defer os.RemoveAll(tempDir)

		store, err := oci.New(tempDir)
		if err != nil {
			return "", nil, err
		}

		// Copy remote repo at the given digest to the scratch dir.
		_, err = oras.Copy(ctx, c.repo, digest, store, digest, oras.DefaultCopyOptions)
		if err != nil {
			return "", nil, err
		}

		err = createTarFile(tempDir, cachedPath)
		if err != nil {
			return "", nil, err
		}
	}

	// Create OCI store from the TAR file stored at cachedPath.
	store, err := oci.NewFromTar(ctx, cachedPath)
	if err != nil {
		return "", nil, err
	}

	// Extract manifests that we want to use to a temp dir. This will be removed once the repo-server has generated and
	// cached the manifests.
	manifestsDir, closer, err := copyManifestsToTempDir(ctx, store, digest)
	if err != nil {
		return "", nil, err
	}

	return manifestsDir, argoio.NewCloser(func() error {
		closer.Close()
		return os.RemoveAll(manifestsDir)
	}), nil
}

func createTarFile(from, to string) error {
	f, err := os.Create(to)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = files.Tar(from, nil, nil, f)
	if err != nil {
		_ = os.RemoveAll(to)
	}
	return err
}

func copyManifestsToTempDir(ctx context.Context, store oras.ReadOnlyTarget, digest string) (string, io.Closer, error) {
	manifestsDir, err := files.CreateTempDir(os.TempDir())
	if err != nil {
		return manifestsDir, nil, err
	}

	fs, err := file.New(manifestsDir)
	if err != nil {
		return manifestsDir, fs, err
	}

	_, err = oras.Copy(ctx, store, digest, fs, digest, oras.DefaultCopyOptions)
	if err != nil {
		return manifestsDir, fs, err
	}

	return manifestsDir, fs, nil
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
		return err
	}
	return os.RemoveAll(cachePath)
}

func (c *nativeOCIClient) ResolveDigest(ctx context.Context, revision string) (string, error) {
	descriptor, err := c.repo.Resolve(ctx, revision)
	if err != nil {
		return "", fmt.Errorf("cannot get digest: %w", err)
	}

	return descriptor.Digest.String(), nil
}

func (c *nativeOCIClient) ResolveRevision(ctx context.Context, revision string, noCache bool) (string, error) {
	constraints, err := semver.NewConstraint(revision)
	if err == nil {
		tags, err := c.GetTags(ctx, noCache)
		if err != nil {
			return "", fmt.Errorf("error fetching tags: %w", err)
		}
		version, err := tags.MaxVersion(constraints)
		if err != nil {
			return "", fmt.Errorf("no version for constraints: %w", err)
		}
		return version.String(), nil
	}

	return revision, nil
}

func (c *nativeOCIClient) GetTags(ctx context.Context, noCache bool) (*TagsList, error) {
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
		err := c.repo.Tags(ctx, "", func(tagsResult []string) error {
			for _, tag := range tagsResult {
				// By convention: Change underscore (_) back to plus (+) to get valid SemVer
				convertedTag := strings.ReplaceAll(tag, "_", "+")
				tags.Tags = append(tags.Tags, convertedTag)
			}

			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get tags: %w", err)
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

func fileExist(filePath string) (bool, error) {
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		} else {
			return false, err
		}
	}
	return true, nil
}
