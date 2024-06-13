package oci

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Masterminds/semver/v3"
	"github.com/argoproj/argo-cd/v2/util/cache"
	argoio "github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/proxy"
	"github.com/argoproj/pkg/sync"
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/url"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"os"
	"strings"
	"time"
)

var (
	globalLock = sync.NewKeyLock()
	indexLock  = sync.NewKeyLock()
)

var _ Client = &nativeOciClient{}

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

type ClientOpts func(c *nativeOciClient)

func WithIndexCache(indexCache indexCache) ClientOpts {
	return func(c *nativeOciClient) {
		c.indexCache = indexCache
	}
}

func WithChartPaths(repoCachePaths argoio.TempPaths) ClientOpts {
	return func(c *nativeOciClient) {
		c.repoCachePaths = repoCachePaths
	}
}
func NewClient(repoURL string, creds Creds, proxy string, opts ...ClientOpts) (Client, error) {
	return NewClientWithLock(repoURL, creds, globalLock, proxy, opts...)
}
func NewClientWithLock(repoURL string, creds Creds, repoLock sync.KeyLock, proxyUrl string, opts ...ClientOpts) (Client, error) {
	ociRepo := strings.TrimPrefix(repoURL, "oci://")
	repo, err := remote.NewRepository(ociRepo)

	if err != nil {
		return nil, fmt.Errorf("failed to initialize repository: %v", err)
	}

	repo.PlainHTTP = creds.InsecureHttpOnly

	ociUri, err := url.Parse(repoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse registry host: %v", err)
	}

	var tlsConf *tls.Config
	if !repo.PlainHTTP {
		tlsConf, err = newTLSConfig(creds)
		if err != nil {
			return nil, fmt.Errorf("failed setup tlsConfig: %v", err)
		}
	}

	var ociRegistry string
	if ociUri.Port() != "" {
		ociRegistry = fmt.Sprintf("%s:%s", ociUri.Hostname(), ociUri.Port())
	} else {
		ociRegistry = ociUri.Hostname()
	}

	client := &http.Client{Transport: &http.Transport{
		Proxy:             proxy.GetCallback(proxyUrl),
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

	c := &nativeOciClient{
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

// nativeOciClient implements Client interface using oras-go
type nativeOciClient struct {
	creds          Creds
	repoURL        string
	proxy          string
	repo           *remote.Repository
	repoLock       sync.KeyLock
	indexCache     indexCache
	repoCachePaths argoio.TempPaths
}

func (c *nativeOciClient) TestRepo(ctx context.Context) (bool, error) {
	err := c.repo.Tags(ctx, "", func(tags []string) error {
		return nil
	})
	return err == nil, err
}

func (c *nativeOciClient) Extract(ctx context.Context, revision string, project string, manifestMaxExtractedSize int64, disableManifestMaxExtractedSize bool) (string, argoio.Closer, error) {
	cachedPath, err := c.getCachedPath(revision, project)
	if err != nil {
		return "", nil, err
	}

	c.repoLock.Lock(cachedPath)
	defer c.repoLock.Unlock(cachedPath)

	exists, err := fileExist(cachedPath)
	if err != nil {
		return "", nil, err
	}

	fs, err := file.New(cachedPath)
	if err != nil {
		return "", nil, err
	}

	if !exists {
		_, err = oras.Copy(ctx, c.repo, revision, fs, revision, oras.DefaultCopyOptions)
		if err != nil {
			return "", nil, err
		}
	}

	return cachedPath, fs, nil
}

func (c *nativeOciClient) getCachedPath(version, project string) (string, error) {
	keyData, err := json.Marshal(map[string]string{"url": c.repoURL, "project": project, "version": version})
	if err != nil {
		return "", err
	}
	return c.repoCachePaths.GetPath(string(keyData))
}

func (c *nativeOciClient) CleanCache(revision, project string) error {
	cachePath, err := c.getCachedPath(revision, project)
	if err != nil {
		return err
	}
	return os.RemoveAll(cachePath)
}

func (c *nativeOciClient) ResolveDigest(ctx context.Context, revision string) (string, error) {
	descriptor, err := c.repo.Resolve(ctx, revision)
	if err != nil {
		return "", fmt.Errorf("cannot get digest: %v", err)
	}

	return descriptor.Digest.String(), nil
}

func (c *nativeOciClient) ResolveRevision(ctx context.Context, revision string, noCache bool) (string, error) {
	constraints, err := semver.NewConstraint(revision)
	if err == nil {
		tags, err := c.GetTags(ctx, noCache)
		version, err := tags.MaxVersion(constraints)
		if err != nil {
			return "", fmt.Errorf("no version for constraints: %v", err)
		}

		return version.String(), nil
	}

	return revision, nil

}

func (c *nativeOciClient) GetTags(ctx context.Context, noCache bool) (*TagsList, error) {
	indexLock.Lock(c.repoURL)
	defer indexLock.Unlock(c.repoURL)

	var data []byte
	if !noCache && c.indexCache != nil {
		if err := c.indexCache.GetHelmIndex(c.repoURL, &data); err != nil && !errors.Is(err, cache.ErrCacheMiss) {
			log.Warnf("Failed to load index cache for repo: %s: %v", c.repoLock, err)
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
			return nil, fmt.Errorf("failed to get tags: %v", err)
		}
		log.WithFields(
			log.Fields{"seconds": time.Since(start).Seconds(), "repo": c.repoURL},
		).Info("took to get tags")

		if c.indexCache != nil {
			if err := c.indexCache.SetHelmIndex(c.repoURL, data); err != nil {
				log.Warnf("Failed to store tags list cache for repo: %s: %v", c.repoURL, err)
			}
		}
	} else {
		err := json.Unmarshal(data, tags)
		if err != nil {
			return nil, fmt.Errorf("failed to decode tags: %v", err)
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
