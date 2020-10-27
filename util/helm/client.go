package helm

import (
	"bytes"
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
	"github.com/argoproj/pkg/sync"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	executil "github.com/argoproj/argo-cd/util/exec"
	"github.com/argoproj/argo-cd/util/io"
)

var (
	globalLock = sync.NewKeyLock()
)

type Creds struct {
	Username           string
	Password           string
	CAPath             string
	CertData           []byte
	KeyData            []byte
	InsecureSkipVerify bool
}

type Client interface {
	CleanChartCache(chart string, version *semver.Version) error
	ExtractChart(chart string, version *semver.Version) (string, io.Closer, error)
	GetIndex() (*Index, error)
	TestHelmOCI() (bool, error)
}

func NewClient(repoURL string, creds Creds, enableOci bool) Client {
	return NewClientWithLock(repoURL, creds, globalLock, enableOci)
}

func NewClientWithLock(repoURL string, creds Creds, repoLock sync.KeyLock, enableOci bool) Client {
	return &nativeHelmChart{
		repoURL:   repoURL,
		creds:     creds,
		repoPath:  filepath.Join(os.TempDir(), strings.Replace(repoURL, "/", "_", -1)),
		repoLock:  repoLock,
		enableOci: enableOci,
	}
}

type nativeHelmChart struct {
	repoPath  string
	repoURL   string
	creds     Creds
	repoLock  sync.KeyLock
	enableOci bool
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

func (c *nativeHelmChart) ensureHelmChartRepoPath() error {
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

func (c *nativeHelmChart) ExtractChart(chart string, version *semver.Version) (string, io.Closer, error) {
	err := c.ensureHelmChartRepoPath()
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
		// always use Helm V3 since we don't have chart content to determine correct Helm version
		helmCmd, err := NewCmdWithVersion(c.repoPath, HelmV3, c.enableOci)

		if err != nil {
			return "", nil, err
		}
		defer helmCmd.Close()

		_, err = helmCmd.Init()
		if err != nil {
			return "", nil, err
		}

		if c.enableOci {
			_, err = helmCmd.Login(c.repoURL, c.creds)
			if err != nil {
				return "", nil, err
			}
			defer func() {
				_, _ = helmCmd.Logout(c.repoURL, c.creds)
			}()
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

		if c.enableOci {
			_, err = helmCmd.Export(c.repoURL, chart, version.String(), tempDest)
			if err != nil {
				return "", nil, err
			}
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
	_, err = executil.Run(cmd)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return "", nil, err
	}
	return path.Join(tempDir, normalizeChartName(chart)), io.NewCloser(func() error {
		return os.RemoveAll(tempDir)
	}), nil
}

func (c *nativeHelmChart) GetIndex() (*Index, error) {
	start := time.Now()

	data, err := c.loadRepoIndex()
	if err != nil {
		return nil, err
	}

	index := &Index{}
	err = yaml.NewDecoder(bytes.NewBuffer(data)).Decode(index)
	if err != nil {
		return nil, err
	}

	log.WithFields(log.Fields{"seconds": time.Since(start).Seconds()}).Info("took to get index")

	return index, nil
}

func (c *nativeHelmChart) TestHelmOCI() (bool, error) {
	start := time.Now()

	tmpDir, err := ioutil.TempDir("", "helm")
	if err != nil {
		return false, err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	helmCmd, err := NewCmdWithVersion(tmpDir, HelmV3, c.enableOci)
	if err != nil {
		return false, err
	}
	defer helmCmd.Close()

	_, err = helmCmd.Login(c.repoURL, c.creds)
	if err != nil {
		return false, err
	}
	defer func() {
		_, _ = helmCmd.Logout(c.repoURL, c.creds)
	}()

	log.WithFields(log.Fields{"seconds": time.Since(start).Seconds()}).Info("took to test helm oci repository")

	return true, nil
}

func (c *nativeHelmChart) loadRepoIndex() ([]byte, error) {
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
	tr := &http.Transport{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: tlsConf,
	}
	client := http.Client{Transport: tr}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return nil, errors.New("failed to get index: " + resp.Status)
	}
	return ioutil.ReadAll(resp.Body)
}

func newTLSConfig(creds Creds) (*tls.Config, error) {
	tlsConfig := &tls.Config{InsecureSkipVerify: creds.InsecureSkipVerify}

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
	// nolint:staticcheck
	tlsConfig.BuildNameToCertificate()

	return tlsConfig, nil
}

// Normalize a chart name for file system use, that is, if chart name is foo/bar/baz, returns the last component as chart name.
func normalizeChartName(chart string) string {
	_, nc := path.Split(chart)
	// We do not want to return the empty string or something else related to filesystem access
	// Instead, return original string
	if nc == "" || nc == "." || nc == ".." {
		return chart
	}
	return nc
}

func (c *nativeHelmChart) getChartPath(chart string, version *semver.Version) string {
	if repoNamespace, chartName, isHelmOci := IsHelmOci(chart); isHelmOci {
		return path.Join(c.repoPath, fmt.Sprintf("%s-%s-%v.tgz", repoNamespace, normalizeChartName(chartName), version))
	}
	return path.Join(c.repoPath, fmt.Sprintf("%s-%v.tgz", normalizeChartName(chart), version))
}

func IsHelmOci(chart string) (string, string, bool) {
	chartArray := strings.Split(chart, "/")
	if len(chartArray) == 2 {
		return chartArray[0], chartArray[1], true
	}
	return "", "", false
}
