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

	"github.com/Masterminds/semver"
	argoexec "github.com/argoproj/pkg/exec"
	"github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/config"
)

var (
	globalLock = util.NewKeyLock()
)

type Creds struct {
	Username string
	Password string
	CAPath   string
	CertData []byte
	KeyData  []byte
}

type Client interface {
	CleanChartCache(chart string, version *semver.Version) error
	ExtractChart(chart string, version *semver.Version) (string, util.Closer, error)
	GetIndex() (*Index, error)
}

func NewClient(repoURL string, creds Creds) Client {
	return NewClientWithLock(repoURL, creds, globalLock)
}

func NewClientWithLock(repoURL string, creds Creds, repoLock *util.KeyLock) Client {
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
	repoLock *util.KeyLock
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

func (c *nativeHelmChart) CleanChartCache(chart string, version *semver.Version) error {
	return os.RemoveAll(c.getChartPath(chart, version))
}

func (c *nativeHelmChart) ExtractChart(chart string, version *semver.Version) (string, util.Closer, error) {
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

		// (1) because `helm fetch` downloads an arbitrary file name, we download to an empty temp directory
		tempDest, err := ioutil.TempDir("", "helm")
		if err != nil {
			return "", nil, err
		}
		defer func() { _ = os.RemoveAll(tempDest) }()
		_, err = helmCmd.Fetch(c.repoURL, chart, version.String(), tempDest, c.creds)
		if err != nil {
			return "", nil, err
		}
		// (2) then we assume that the only file downloaded into the directory is the tgz file
		// and we move that to where we want it
		infos, err := ioutil.ReadDir(tempDest)
		if err != nil {
			return "", nil, err
		}
		if len(infos) != 1 {
			return "", nil, fmt.Errorf("expected 1 file, found %v", len(infos))
		}
		err = os.Rename(filepath.Join(tempDest, infos[0].Name()), chartPath)
		if err != nil {
			return "", nil, err
		}
	}
	// untar helm chart into throw away temp directory which should be deleted as soon as no longer needed
	tempDir, err := ioutil.TempDir("", "helm")
	if err != nil {
		return "", nil, err
	}
	cmd := exec.Command("tar", "-zxvf", chartPath)
	cmd.Dir = tempDir
	_, err = argoexec.RunCommandExt(cmd, config.CmdOpts())
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return "", nil, err
	}
	return path.Join(tempDir, chart), util.NewCloser(func() error {
		return os.RemoveAll(tempDir)
	}), nil
}

func (c *nativeHelmChart) GetIndex() (*Index, error) {
	cachedIndex, found := indexCache.Get(c.repoURL)
	if found {
		log.WithFields(log.Fields{"url": c.repoURL}).Debug("index cache hit")
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
		return nil, errors.New("failed to get index: " + resp.Status)
	}

	index := &Index{}
	err = yaml.NewDecoder(resp.Body).Decode(index)

	log.WithFields(log.Fields{"seconds": time.Since(start).Seconds()}).Info("took to get index")

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

func (c *nativeHelmChart) getChartPath(chart string, version *semver.Version) string {
	return path.Join(c.repoPath, fmt.Sprintf("%s-%v.tgz", chart, version))
}
