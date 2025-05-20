package kustomize

import (
	"fmt"
	"net/url"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/util/git"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

type ResolveRevisionFunc func(repo, revision string, creds git.Creds) (string, error)

func (k *kustomize) GetCacheKeyWithComponents(revision string, source *v1alpha1.ApplicationSourceKustomize, resolveRevisionFunc ResolveRevisionFunc) (string, error) {
	cacheKey := ""

	revisionsToResolve := map[string]string{}

	for _, c := range source.Components {
		_, err := url.Parse(c)
		if err != nil {
			continue // local files are not part of the cache key
		}

		_, _, path, ref := parseGitURL(c)
		if ref == "" {
			ref = "HEAD"
		}

		cleanRepoURL := c
		if path != "" {
			suffixToTrim := c
			if searchedValueIndex := strings.Index(c, path); searchedValueIndex != -1 {
				suffixToTrim = c[:searchedValueIndex]
			}
			cleanRepoURL = strings.TrimSuffix(suffixToTrim, "/")
		}

		revisionsToResolve[cleanRepoURL] = ref
	}

	for component, ref := range revisionsToResolve {
		rev, err := resolveRevisionFunc(component, ref, k.creds)
		if err != nil {
			log.WithError(err).
				WithField("url", component).
				Warn("failed to resolve revision of component from url, ignoring in cache key")
			continue
		}
		if cacheKey != "" {
			cacheKey += "|"
		}
		cacheKey += fmt.Sprintf("%s|%s", component, rev)
	}

	return fmt.Sprintf("%s|%s", revision, cacheKey), nil
}
