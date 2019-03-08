package repos

import (
	"archive/tar"
	"compress/gzip"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

type helmClient struct {
	// URL
	repoURL string
	// destination
	root     string
	username string
	password string
}

func (f *factory) newHelmClient(repoURL, path, username, password string) (Client, error) {
	return helmClient{repoURL, path, username, password}, nil
}

func (c helmClient) Test() error {
	resp, err := http.Get(c.repoURL)
	if err != nil {
		return err
	}
	// we cannot check HTTP status code, it may be 404
	return resp.Body.Close()
}

func (c helmClient) Root() string {
	return c.root
}

func (c helmClient) Checkout(path, chartVersion string) (string, error) {

	chartName := c.chartName(path)

	url := c.repoURL + "/" + chartName + "-" + chartVersion + ".tgz"

	log.Infof("Helm checkout url=%s, root=%s, path=%s", url, c.root, path)

	_, err := exec.Command("rm", "-rf", c.root).Output()
	if err != nil {
		return "", fmt.Errorf("unable to clean repo at %s: %v", c.root, err)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	if c.username != "" {
		req.Header["Authorization"] = []string{"Basic " + base64.StdEncoding.EncodeToString([]byte(c.username+":"+c.password))}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return "", errors.New(fmt.Sprintf("unable to checkout Helm chart %s, expected 200 status code, got %d", url, resp.StatusCode))
	}

	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return "", err
	}
	defer func() { _ = gzr.Close() }()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()

		switch {
		case err == io.EOF:
			return chartVersion, nil
		case err != nil:
			return "", errors.New(fmt.Sprintf("unable to checkout Helm chart %s, %v", url, err))
		case header == nil:
			continue
		}

		target := filepath.Join(c.root, header.Name)

		switch header.Typeflag {

		case tar.TypeReg:
			dir := filepath.Dir(target)
			if _, err := os.Stat(dir); err != nil {
				if err := os.MkdirAll(dir, 0755); err != nil {
					return "", errors.New(fmt.Sprintf("unable to checkout Helm chart %s, %v", url, err))
				}
			}
			file, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return "", errors.New(fmt.Sprintf("unable to checkout Helm chart %s, %v", url, err))
			}

			if _, err := io.Copy(file, tr); err != nil {
				return "", errors.New(fmt.Sprintf("unable to checkout Helm chart %s, %v", url, err))
			}

			_ = file.Close()
		}
	}
}

func (c helmClient) chartName(path string) string {
	return strings.Split(path, "/")[0]
}

func (c helmClient) ResolveRevision(revision string) (string, error) {
	return revision, nil
}

func (c helmClient) LsFiles(path string) ([]string, error) {

	chartName := c.chartName(path)
	files, err := ioutil.ReadDir(filepath.Join(c.root, chartName))
	if err != nil {
		return nil, err
	}

	var names = make([]string, 0)
	for _, f := range files {
		name := filepath.Join(chartName, f.Name())
		if ok, _ := filepath.Match(path, name); ok {
			names = append(names, name)
		}
	}

	return names, nil
}
