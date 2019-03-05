package repos

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

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
		return errors.New(fmt.Sprintf("expected 200, got %d", resp.StatusCode))
	}
	return nil
}

func (c helmClient) Root() string {
	return c.root
}

func (c helmClient) Init() error {
	return nil
}

func (c helmClient) Fetch() error {
	return nil
}

func (c helmClient) Checkout(path, revision string) error {

	url := c.repoURL + "/" + path + "-" + revision + ".tgz"
	log.Infof("Helm checkout url=%s, root=%s", url, c.root)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}

	//noinspection GoUnhandledErrorResult
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.New(fmt.Sprintf("expected 200 status code, got %d", resp.StatusCode))
	}

	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()

		switch {
		case err == io.EOF:
			return nil
		case err != nil:
			return err
		case header == nil:
			continue
		}

		target := filepath.Join(c.root, header.Name)

		switch header.Typeflag {

		case tar.TypeReg:
			dir := filepath.Dir(target)
			if _, err := os.Stat(dir); err != nil {
				if err := os.MkdirAll(dir, 0755); err != nil {
					return err
				}
			}
			file, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			if _, err := io.Copy(file, tr); err != nil {
				return err
			}

			file.Close()
		}
	}
}

func (c helmClient) ResolveRevision(revision string) (string, error) {
	return "", nil
}

func (c helmClient) LsFiles(path string) ([]string, error) {
	return make([]string, 0), nil
}

func (c helmClient) LatestRevision(revision string) (string, error) {
	return revision, nil
}
