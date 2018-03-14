package commands

import (
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

// isSupportedURL checks if a URL is of a supported type for loading manifests.
func hasSupportedManifestURLScheme(url string) bool {
	for _, scheme := range []string{"https://", "http://"} {
		if lowercaseUrl := strings.ToLower(url); strings.HasPrefix(lowercaseUrl, scheme) {
			return true
		}
	}
	return false
}

// readLocalFile reads a file from disk and returns its contents as a byte array.
func readLocalFile(path string) (data []byte, err error) {
	data, err = ioutil.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	return
}

// readRemoteFile issues a GET request to retrieve the contents of the specified URL as a byte array.
func readRemoteFile(url string) (data []byte, err error) {
	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	data, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	return
}
