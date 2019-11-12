package helm

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/argoproj/argo-cd/engine/util/misc"

	argoexec "github.com/argoproj/pkg/exec"
	"github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

var (
	indexCache = cache.New(5*time.Minute, 5*time.Minute)
	globalLock = misc.NewKeyLock()
)

type Entry struct {
	Version string
	Created time.Time
}

type Index struct {
	Entries map[string][]Entry
}

type Creds struct {
	Username string
	Password string
	CAPath   string
	CertData []byte
	KeyData  []byte
}

type Client interface {
	CleanChartCache(chart string, version string) error
	ExtractChart(chart string, version string) (string, misc.Closer, error)
	GetIndex() (*Index, error)
}

func NewClient(repoURL string, creds Creds) Client {
	return NewClientWithLock(repoURL, creds, globalLock)
}

func NewClientWithLock(repoURL string, creds Creds, repoLock *misc.KeyLock) Client {
	return &nativeHelmChart{
		repoURL:  repoURL,
		creds:    creds,
		repoPath: filepath.Join(os.TempDir(), strings.Replace(repoURL, "/", "_", -1)),
		repoLock: repoLock,
	}
}

type nativeHelmChart struct {
	repoPath string
	repoURL  string
	creds    Creds
	repoLock *misc.KeyLock
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

func (c *nativeHelmChart) helmChartRepoPath() error {
	c.repoLock.Lock(c.repoPath)
	defer c.repoLock.Unlock(c.repoPath)

	err := os.Mkdir(c.repoPath, 0700)
	if err != nil && !os.IsExist(err) {
		return err
	}
	return nil
}

func (c *nativeHelmChart) CleanChartCache(chart string, version string) error {
	return os.RemoveAll(c.getChartPath(chart, version))
}

func (c *nativeHelmChart) ExtractChart(chart string, version string) (string, misc.Closer, error) {
	err := c.helmChartRepoPath()
	if err != nil {
		return "", nil, err
	}
	chartPath := c.getChartPath(chart, version)

	c.repoLock.Lock(chartPath)
	defer c.repoLock.Unlock(chartPath)

	exists, err := fileExist(chartPath)
	if err != nil {
		return "", nil, err
	}
	if !exists {
		helmCmd, err := NewCmd(c.repoPath)
		if err != nil {
			return "", nil, err
		}
		defer helmCmd.Close()

		_, err = helmCmd.Init()
		if err != nil {
			return "", nil, err
		}

		_, err = helmCmd.RepoUpdate()
		if err != nil {
			return "", nil, err
		}

		// download chart tar file into persistent helm repository directory
		_, err = helmCmd.Fetch(c.repoURL, chart, version, c.creds)
		if err != nil {
			return "", nil, err
		}
	}
	// untar helm chart into throw away temp directory which should be deleted as soon as no longer needed
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		return "", nil, err
	}
	cmd := exec.Command("tar", "-zxvf", chartPath)
	cmd.Dir = tempDir
	_, err = argoexec.RunCommandExt(cmd, argoexec.CmdOpts{})
	if err != nil {
		_ = os.RemoveAll(tempDir)
	}
	return path.Join(tempDir, chart), misc.NewCloser(func() error {
		return os.RemoveAll(tempDir)
	}), nil
}

func (c *nativeHelmChart) GetIndex() (*Index, error) {
	cachedIndex, found := indexCache.Get(c.repoURL)
	if found {
		log.WithFields(log.Fields{"url": c.repoURL}).Debug("Index cache hit")
		i := cachedIndex.(Index)
		return &i, nil
	}

	start := time.Now()
	repoURL, err := url.Parse(c.repoURL)
	if err != nil {
		return nil, err
	}
	repoURL.Path = path.Join(repoURL.Path, "index.yaml")

	req, err := http.NewRequest("GET", repoURL.String(), nil)
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
	tr := &http.Transport{TLSClientConfig: tlsConf}
	client := http.Client{Transport: tr}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return nil, errors.New("failed to get Index: " + resp.Status)
	}

	index := &Index{}
	err = yaml.NewDecoder(resp.Body).Decode(index)

	log.WithFields(log.Fields{"seconds": time.Since(start).Seconds()}).Info("took to get Index")

	indexCache.Set(c.repoURL, *index, cache.DefaultExpiration)

	return index, err
}

func newTLSConfig(creds Creds) (*tls.Config, error) {
	tlsConfig := &tls.Config{InsecureSkipVerify: false}

	if creds.CAPath != "" {
		caData, err := ioutil.ReadFile(creds.CAPath)
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
	tlsConfig.BuildNameToCertificate()

	return tlsConfig, nil
}

func (c *nativeHelmChart) getChartPath(chart string, version string) string {
	return path.Join(c.repoPath, fmt.Sprintf("%s-%s.tgz", chart, version))
}
