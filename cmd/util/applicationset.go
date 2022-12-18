package util

import (
	"fmt"
	"net/url"
	"os"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/config"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
)

func ConstructApplicationSet(fileURL string) ([]*argoprojiov1alpha1.ApplicationSet, error) {
	if fileURL != "" {
		return constructAppsetFromFileUrl(fileURL)
	}
	return nil, nil
}

func constructAppsetFromFileUrl(fileURL string) ([]*argoprojiov1alpha1.ApplicationSet, error) {
	appset := make([]*argoprojiov1alpha1.ApplicationSet, 0)
	// read uri
	err := readAppsetFromURI(fileURL, &appset)
	if err != nil {
		return nil, fmt.Errorf("error reading applicationset from file %s: %s", fileURL, err)
	}

	return appset, nil
}

func readAppsetFromURI(fileURL string, appset *[]*argoprojiov1alpha1.ApplicationSet) error {

	readFilePayload := func() ([]byte, error) {
		parsedURL, err := url.ParseRequestURI(fileURL)
		if err != nil || !(parsedURL.Scheme == "http" || parsedURL.Scheme == "https") {
			return os.ReadFile(fileURL)
		}
		return config.ReadRemoteFile(fileURL)
	}

	yml, err := readFilePayload()
	if err != nil {
		return err
	}

	return readAppset(yml, appset)
}

func readAppset(yml []byte, appsets *[]*argoprojiov1alpha1.ApplicationSet) error {
	yamls, err := kube.SplitYAMLToString(yml)
	if err != nil {
		return err
	}

	for _, yml := range yamls {
		var appset argoprojiov1alpha1.ApplicationSet
		err = config.Unmarshal([]byte(yml), &appset)
		if err != nil {
			return err
		}
		*appsets = append(*appsets, &appset)

	}

	return err
}
