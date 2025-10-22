package util

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/config"
)

func ConstructApplicationSet(ctx context.Context, fileURL string) ([]*argoprojiov1alpha1.ApplicationSet, error) {
	if fileURL != "" {
		return constructAppsetFromFileURL(ctx, fileURL)
	}
	return nil, nil
}

func constructAppsetFromFileURL(ctx context.Context, fileURL string) ([]*argoprojiov1alpha1.ApplicationSet, error) {
	appset := make([]*argoprojiov1alpha1.ApplicationSet, 0)
	// read uri
	err := readAppsetFromURI(ctx, fileURL, &appset)
	if err != nil {
		return nil, fmt.Errorf("error reading applicationset from file %s: %w", fileURL, err)
	}

	return appset, nil
}

func readAppsetFromURI(ctx context.Context, fileURL string, appset *[]*argoprojiov1alpha1.ApplicationSet) error {
	readFilePayload := func(ctx context.Context) ([]byte, error) {
		parsedURL, err := url.ParseRequestURI(fileURL)
		if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
			return os.ReadFile(fileURL)
		}
		return config.ReadRemoteFile(ctx, fileURL)
	}

	yml, err := readFilePayload(ctx)
	if err != nil {
		return fmt.Errorf("error reading file payload: %w", err)
	}

	return readAppset(yml, appset)
}

func readAppset(yml []byte, appsets *[]*argoprojiov1alpha1.ApplicationSet) error {
	yamls, err := kube.SplitYAMLToString(yml)
	if err != nil {
		return fmt.Errorf("error splitting YAML to string: %w", err)
	}

	for _, yml := range yamls {
		var appset argoprojiov1alpha1.ApplicationSet
		err = config.Unmarshal([]byte(yml), &appset)
		if err != nil {
			return fmt.Errorf("error unmarshalling appset: %w", err)
		}
		*appsets = append(*appsets, &appset)
	}
	// we reach here if there is no error found while reading the Application Set
	return nil
}
