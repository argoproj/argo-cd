package helm

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	executil "github.com/argoproj/argo-cd/v2/util/exec"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/argoproj/pkg/sync"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/util/cache"
	argoio "github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/io/files"
	"github.com/argoproj/argo-cd/v2/util/proxy"
	"github.com/argoproj/argo-cd/v2/util/text"
)

var (
	globalLock = sync.NewKeyLock()
	indexLock  = sync.NewKeyLock()
)

type Creds struct {
	Username           string
	Password           string
	CAPath             string
	CertData           []byte
	KeyData            []byte
	InsecureSkipVerify bool
}

type indexCache interface {
	SetHelmIndex(repo string, indexData []byte) error
	GetHelmIndex(repo string, indexData *[]byte) error
}

type Client interface {
	CleanChartCache(chart string, version string) error
	ExtractChart(chart string, version string, passCredentials bool, manifestMaxExtractedSize int64, disableManifestMaxExtractedSize bool) (string, argoio.Closer, error)
	GetIndex(noCache bool) (*Index, error)
	GetTags(chart string, noCache bool) (*TagsList, error)
	TestHelmOCI() (bool, error)
}

type ClientOpts func(c *nativeHelmChart)

func WithIndexCache(indexCache indexCache) ClientOpts {
	return func(c *nativeHelmChart) {
		c.indexCache = indexCache
	}
}

func WithChartPaths(chartPaths argoio.TempPaths) ClientOpts {
	return func(c *nativeHelmChart) {
		c.chartCachePaths = chartPaths
	}
}

func NewClient(repoURL string, creds Creds, enableOci bool, proxy string, opts ...ClientOpts) Client {
	return NewClientWithLock(repoURL, creds, globalLock, enableOci, proxy, opts...)
}

func NewClientWithLock(repoURL string, creds Creds, repoLock sync.KeyLock, enableOci bool, proxy string, opts ...ClientOpts) Client {
	c := &nativeHelmChart{
		repoURL:         repoURL,
		creds:           creds,
		repoLock:        repoLock,
		enableOci:       enableOci,
		proxy:           proxy,
		chartCachePaths: argoio.NewRandomizedTempPaths(os.TempDir()),
	}
	for i := range opts {
		opts[i](c)
	}
	return c
}

var _ Client = &nativeHelmChart{}

type nativeHelmChart struct {
	chartCachePaths argoio.TempPaths
	repoURL         string
	creds           Creds
	repoLock        sync.KeyLock
	enableOci       bool
	indexCache      indexCache
	proxy           string
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

func (c *nativeHelmChart) CleanChartCache(chart string, version string) error {
	cachePath, err := c.getCachedChartPath(chart, version)
	if err != nil {
		return err
	}
	return os.RemoveAll(cachePath)
}

func untarChart(tempDir string, cachedChartPath string, manifestMaxExtractedSize int64, disableManifestMaxExtractedSize bool) error {
	if disableManifestMaxExtractedSize {
		cmd := exec.Command("tar", "-zxvf", cachedChartPath)
		cmd.Dir = tempDir
		_, err := executil.Run(cmd)
		return err
	}
	reader, err := os.Open(cachedChartPath)
	if err != nil {
		return err
	}
	return files.Untgz(tempDir, reader, manifestMaxExtractedSize, false)
}

func (c *nativeHelmChart) ExtractChart(chart string, version string, passCredentials bool, manifestMaxExtractedSize int64, disableManifestMaxExtractedSize bool) (string, argoio.Closer, error) {
	// always use Helm V3 since we don't have chart content to determine correct Helm version
	helmCmd, err := NewCmdWithVersion("", HelmV3, c.enableOci, c.proxy)

	if err != nil {
		return "", nil, err
	}
	defer helmCmd.Close()

	_, err = helmCmd.Init()
	if err != nil {
		return "", nil, err
	}

	// throw away temp directory that stores extracted chart and should be deleted as soon as no longer needed by returned closer
	tempDir, err := files.CreateTempDir(os.TempDir())
	if err != nil {
		return "", nil, err
	}

	cachedChartPath, err := c.getCachedChartPath(chart, version)
	if err != nil {
		return "", nil, err
	}

	c.repoLock.Lock(cachedChartPath)
	defer c.repoLock.Unlock(cachedChartPath)

	// check if chart tar is already downloaded
	exists, err := fileExist(cachedChartPath)
	if err != nil {
		return "", nil, err
	}

	if !exists {
		// create empty temp directory to extract chart from the registry
		tempDest, err := files.CreateTempDir(os.TempDir())
		if err != nil {
			return "", nil, err
		}
		defer func() { _ = os.RemoveAll(tempDest) }()

		if c.enableOci {
			if c.creds.Password != "" && c.creds.Username != "" {
				_, err = helmCmd.RegistryLogin(c.repoURL, c.creds)
				if err != nil {
					return "", nil, err
				}

				defer func() {
					_, _ = helmCmd.RegistryLogout(c.repoURL, c.creds)
				}()
			}

			// 'helm pull' ensures that chart is downloaded into temp directory
			_, err = helmCmd.PullOCI(c.repoURL, chart, version, tempDest)
			if err != nil {
				return "", nil, err
			}
		} else {
			_, err = helmCmd.Fetch(c.repoURL, chart, version, tempDest, c.creds, passCredentials)
			if err != nil {
				return "", nil, err
			}
		}

		// 'helm pull/fetch' file downloads chart into the tgz file and we move that to where we want it
		infos, err := os.ReadDir(tempDest)
		if err != nil {
			return "", nil, err
		}
		if len(infos) != 1 {
			return "", nil, fmt.Errorf("expected 1 file, found %v", len(infos))
		}

		err = os.Rename(filepath.Join(tempDest, infos[0].Name()), cachedChartPath)
		if err != nil {
			return "", nil, err
		}
	}

	err = untarChart(tempDir, cachedChartPath, manifestMaxExtractedSize, disableManifestMaxExtractedSize)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return "", nil, err
	}
	return path.Join(tempDir, normalizeChartName(chart)), argoio.NewCloser(func() error {
		return os.RemoveAll(tempDir)
	}), nil
}

func (c *nativeHelmChart) GetIndex(noCache bool) (*Index, error) {
	indexLock.Lock(c.repoURL)
	defer indexLock.Unlock(c.repoURL)

	var data []byte
	if !noCache && c.indexCache != nil {
		if err := c.indexCache.GetHelmIndex(c.repoURL, &data); err != nil && err != cache.ErrCacheMiss {
			log.Warnf("Failed to load index cache for repo: %s: %v", c.repoURL, err)
		}
	}

	if len(data) == 0 {
		start := time.Now()
		var err error
		data, err = c.loadRepoIndex()
		if err != nil {
			return nil, err
		}
		log.WithFields(log.Fields{"seconds": time.Since(start).Seconds()}).Info("took to get index")

		if c.indexCache != nil {
			if err := c.indexCache.SetHelmIndex(c.repoURL, data); err != nil {
				log.Warnf("Failed to store index cache for repo: %s: %v", c.repoURL, err)
			}
		}
	}

	index := &Index{}
	err := yaml.NewDecoder(bytes.NewBuffer(data)).Decode(index)
	if err != nil {
		return nil, err
	}

	return index, nil
}

func (c *nativeHelmChart) TestHelmOCI() (bool, error) {
	start := time.Now()

	tmpDir, err := os.MkdirTemp("", "helm")
	if err != nil {
		return false, err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	helmCmd, err := NewCmdWithVersion(tmpDir, HelmV3, c.enableOci, c.proxy)
	if err != nil {
		return false, err
	}
	defer helmCmd.Close()

	// Looks like there is no good way to test access to OCI repo if credentials are not provided
	// just assume it is accessible
	if c.creds.Username != "" && c.creds.Password != "" {
		_, err = helmCmd.RegistryLogin(c.repoURL, c.creds)
		if err != nil {
			return false, err
		}
		defer func() {
			_, _ = helmCmd.RegistryLogout(c.repoURL, c.creds)
		}()

		log.WithFields(log.Fields{"seconds": time.Since(start).Seconds()}).Info("took to test helm oci repository")
	}
	return true, nil
}

func (c *nativeHelmChart) loadRepoIndex() ([]byte, error) {
	indexURL, err := getIndexURL(c.repoURL)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, indexURL, nil)
	if err != nil {
		return nil, err
	}
	if c.creds.Username != "" || c.creds.Password != "" {
		// only basic supported
		req.SetBasicAuth(c.creds.Username, c.creds.Password)
	}

	tlsConf, err := newTLSConfig(c.creds)
	if err != nil {
		return nil, err
	}

	tr := &http.Transport{
		Proxy:           proxy.GetCallback(c.proxy),
		TLSClientConfig: tlsConf,
		DisableKeepAlives: true,
	}
	client := http.Client{Transport: tr}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to get index: " + resp.Status)
	}
	return io.ReadAll(resp.Body)
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

// Normalize a chart name for file system use, that is, if chart name is foo/bar/baz, returns the last component as chart name.
func normalizeChartName(chart string) string {
	strings.Join(strings.Split(chart, "/"), "_")
	_, nc := path.Split(chart)
	// We do not want to return the empty string or something else related to filesystem access
	// Instead, return original string
	if nc == "" || nc == "." || nc == ".." {
		return chart
	}
	return nc
}

func (c *nativeHelmChart) getCachedChartPath(chart string, version string) (string, error) {
	keyData, err := json.Marshal(map[string]string{"url": c.repoURL, "chart": chart, "version": version})
	if err != nil {
		return "", err
	}
	return c.chartCachePaths.GetPath(string(keyData))
}

// Ensures that given OCI registries URL does not have protocol
func IsHelmOciRepo(repoURL string) bool {
	if repoURL == "" {
		return false
	}
	parsed, err := url.Parse(repoURL)
	// the URL parser treat hostname as either path or opaque if scheme is not specified, so hostname must be empty
	return err == nil && parsed.Host == ""
}

func getIndexURL(rawURL string) (string, error) {
	indexFile := "index.yaml"
	repoURL, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	repoURL.Path = path.Join(repoURL.Path, indexFile)
	repoURL.RawPath = path.Join(repoURL.RawPath, indexFile)
	return repoURL.String(), nil
}

func getTagsListURL(rawURL string, chart string) (string, error) {
	repoURL, err := url.Parse(strings.Trim(rawURL, "/"))
	if err != nil {
		return "", fmt.Errorf("unable to parse repo url: %v", err)
	}
	tagsPathFormat := "%s/v2/%s/tags/list"
	repoURL.Scheme = "https"
	repoURL.Path = fmt.Sprintf(tagsPathFormat, repoURL.Path, chart)
	repoURL.RawPath = fmt.Sprintf(tagsPathFormat, repoURL.RawPath, url.PathEscape(chart))
	return repoURL.String(), nil
}

func (c *nativeHelmChart) getTags(chart string) ([]byte, error) {
	nextURL, err := getTagsListURL(c.repoURL, chart)
	if err != nil {
		return nil, fmt.Errorf("failed to get tag list url: %v", err)
	}

	allTags := &TagsList{}
	var data []byte
	for nextURL != "" {
		log.Debugf("fetching %s tags from %s", chart, sanitizeLog(text.Trunc(nextURL, 100)))
		data, nextURL, err = c.getTagsFromUrl(nextURL)
		if err != nil {
			return nil, fmt.Errorf("failed tags part: %v", err)
		}

		tags := &TagsList{}
		err := json.Unmarshal(data, tags)
		if err != nil {
			return nil, fmt.Errorf("unable to decode json: %v", err)
		}
		allTags.Tags = append(allTags.Tags, tags.Tags...)
	}
	data, err = json.Marshal(allTags)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tag json: %w", err)
	}
	return data, nil
}

func getNextUrl(resp *http.Response) (string, error) {
	link := resp.Header.Get("Link")
	if link == "" {
		return "", nil
	}
	if link[0] != '<' {
		return "", fmt.Errorf("invalid next link %q: missing '<'", link)
	}
	if i := strings.IndexByte(link, '>'); i == -1 {
		return "", fmt.Errorf("invalid next link %q: missing '>'", link)
	} else {
		link = link[1:i]
	}
	linkURL, err := resp.Request.URL.Parse(link)
	if err != nil {
		return "", err
	}
	return linkURL.String(), nil
}

func sanitizeLog(input string) string {
	sanitized := strings.ReplaceAll(input, "\r", "")
	sanitized = strings.ReplaceAll(sanitized, "\n", "")
	return sanitized
}

func (c *nativeHelmChart) getTagsFromUrl(tagsURL string) ([]byte, string, error) {
	req, err := http.NewRequest(http.MethodGet, tagsURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed create request: %v", err)
	}
	req.Header.Add("Accept", `application/json`)
	if c.creds.Username != "" || c.creds.Password != "" {
		// only basic supported
		req.SetBasicAuth(c.creds.Username, c.creds.Password)
	}

	tlsConf, err := newTLSConfig(c.creds)
	if err != nil {
		return nil, "", fmt.Errorf("failed setup tlsConfig: %v", err)
	}

	tr := &http.Transport{
		Proxy:           proxy.GetCallback(c.proxy),
		TLSClientConfig: tlsConf,
		DisableKeepAlives: true,
	}
	client := http.Client{Transport: tr}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("request failed: %v", err)
	}
	defer func() {
		if err = resp.Body.Close(); err != nil {
			log.WithFields(log.Fields{
				common.SecurityField:    common.SecurityMedium,
				common.SecurityCWEField: 775,
			}).Errorf("error closing response %q: %v", text.Trunc(tagsURL, 100), err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		data, err := io.ReadAll(resp.Body)
		var responseExcerpt string
		if err != nil {
			responseExcerpt = fmt.Sprintf("err: %v", err)
		} else {
			responseExcerpt = text.Trunc(string(data), 100)
		}
		return nil, "", fmt.Errorf("invalid response: %s %s", resp.Status, responseExcerpt)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read body: %v", err)
	}
	nextUrl, err := getNextUrl(resp)
	return data, nextUrl, err
}

func (c *nativeHelmChart) GetTags(chart string, noCache bool) (*TagsList, error) {
	tagsURL, err := getTagsListURL(c.repoURL, chart)
	if err != nil {
		return nil, fmt.Errorf("invalid tags url: %v", err)
	}
	indexLock.Lock(tagsURL)
	defer indexLock.Unlock(tagsURL)

	var data []byte
	if !noCache && c.indexCache != nil {
		if err := c.indexCache.GetHelmIndex(tagsURL, &data); err != nil && err != cache.ErrCacheMiss {
			log.Warnf("Failed to load index cache for repo: %s: %v", tagsURL, err)
		}
	}

	if len(data) == 0 {
		start := time.Now()
		var err error
		data, err = c.getTags(chart)
		if err != nil {
			return nil, fmt.Errorf("failed to get tags: %v", err)
		}
		log.WithFields(
			log.Fields{"seconds": time.Since(start).Seconds(), "chart": chart, "repo": c.repoURL},
		).Info("took to get tags")

		if c.indexCache != nil {
			if err := c.indexCache.SetHelmIndex(tagsURL, data); err != nil {
				log.Warnf("Failed to store tags list cache for repo: %s: %v", tagsURL, err)
			}
		}
	}

	tags := &TagsList{}
	err = json.Unmarshal(data, tags)
	if err != nil {
		return nil, fmt.Errorf("failed to decode tags: %v", err)
	}

	return tags, nil
}
