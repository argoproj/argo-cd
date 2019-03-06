package repos

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

type helmClient struct {
	// URL
	repoURL string
	// destination
	root string
}

func (f *factory) newHelmClient(repoURL, path string) (Client, error) {
	return helmClient{repoURL, path}, nil
}

func (c helmClient) Test() error {
	resp, err := http.Get(c.repoURL)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return errors.New(fmt.Sprintf("%s expected 200, got %d", c.repoURL, resp.StatusCode))
	}
	return nil
}

func (c helmClient) Root() string {
	return c.root
}

func (c helmClient) Checkout(path, chartVersion string) (string, error) {

	chartName := c.chartName(path)

	url := c.repoURL + "/" + chartName + "-" + chartVersion + ".tgz"

	log.Infof("Helm checkout url=%s, root=%s, path=%s", url, c.root, path)

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return "", errors.New(fmt.Sprintf("%s expected 200 status code, got %d", url, resp.StatusCode))
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
			return "", err
		case header == nil:
			continue
		}

		target := filepath.Join(c.root, header.Name)

		switch header.Typeflag {

		case tar.TypeReg:
			dir := filepath.Dir(target)
			if _, err := os.Stat(dir); err != nil {
				if err := os.MkdirAll(dir, 0755); err != nil {
					return "", err
				}
			}
			file, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return "", err
			}

			if _, err := io.Copy(file, tr); err != nil {
				return "", err
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
