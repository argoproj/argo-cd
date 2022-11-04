package registry

// Package registry implements functions for retrieving data from container
// registries.
//
// TODO: Refactor this package and provide mocks for better testing.

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/distribution/distribution/v3"

	"golang.org/x/sync/semaphore"

	"github.com/argoproj/argo-cd/v2/image-updater/image"
	"github.com/argoproj/argo-cd/v2/image-updater/kube"
	"github.com/argoproj/argo-cd/v2/image-updater/log"
	"github.com/argoproj/argo-cd/v2/image-updater/tag"
)

const (
	MaxMetadataConcurrency = 20
)

// GetTags returns a list of available tags for the given image
func (endpoint *RegistryEndpoint) GetTags(img *image.ContainerImage, regClient RegistryClient, vc *image.VersionConstraint) (*tag.ImageTagList, error) {
	var tagList *tag.ImageTagList = tag.NewImageTagList()
	var err error

	logCtx := vc.Options.Logger()

	// Some registries have a default namespace that is used when the image name
	// doesn't specify one. For example at Docker Hub, this is 'library'.
	var nameInRegistry string
	if len := len(strings.Split(img.ImageName, "/")); len == 1 && endpoint.DefaultNS != "" {
		nameInRegistry = endpoint.DefaultNS + "/" + img.ImageName
		logCtx.Debugf("Using canonical image name '%s' for image '%s'", nameInRegistry, img.ImageName)
	} else {
		nameInRegistry = img.ImageName
	}
	err = regClient.NewRepository(nameInRegistry)
	if err != nil {
		return nil, err
	}
	tTags, err := regClient.Tags()
	if err != nil {
		return nil, err
	}

	tags := []string{}

	// For digest strategy, we do require a version constraint
	if vc.Strategy.NeedsVersionConstraint() && vc.Constraint == "" {
		return nil, fmt.Errorf("cannot use update strategy 'digest' for image '%s' without a version constraint", img.Original())
	}

	// Loop through tags, removing those we do not want. If update strategy is
	// digest, all but the constraint tag are ignored.
	if vc.MatchFunc != nil || len(vc.IgnoreList) > 0 || vc.Strategy.WantsOnlyConstraintTag() {
		for _, t := range tTags {
			if (vc.MatchFunc != nil && !vc.MatchFunc(t, vc.MatchArgs)) || vc.IsTagIgnored(t) || (vc.Strategy.WantsOnlyConstraintTag() && t != vc.Constraint) {
				logCtx.Tracef("Removing tag %s because it either didn't match defined pattern or is ignored", t)
			} else {
				tags = append(tags, t)
			}
		}
	} else {
		tags = tTags
	}

	// In some cases, we don't need to fetch the metadata to get the creation time
	// stamp of from the image's meta data:
	//
	// - We use an update strategy other than latest or digest
	// - The registry doesn't provide meta data and has tags sorted already
	//
	// We just create a dummy time stamp according to the registry's sort mode, if
	// set.
	if (vc.Strategy != image.StrategyNewestBuild && vc.Strategy != image.StrategyDigest) || endpoint.TagListSort.IsTimeSorted() {
		for i, tagStr := range tags {
			var ts int
			if endpoint.TagListSort == TagListSortLatestFirst {
				ts = len(tags) - i
			} else if endpoint.TagListSort == TagListSortLatestLast {
				ts = i
			}
			imgTag := tag.NewImageTag(tagStr, time.Unix(int64(ts), 0), "")
			tagList.Add(imgTag)
		}
		return tagList, nil
	}

	sem := semaphore.NewWeighted(int64(MaxMetadataConcurrency))
	tagListLock := &sync.RWMutex{}

	var wg sync.WaitGroup
	wg.Add(len(tags))

	// Fetch the manifest for the tag -- we need v1, because it contains history
	// information that we require.
	i := 0
	for _, tagStr := range tags {
		i += 1
		// Look into the cache first and re-use any found item. If GetTag() returns
		// an error, we treat it as a cache miss and just go ahead to invalidate
		// the entry.
		if vc.Strategy.IsCacheable() {
			imgTag, err := endpoint.Cache.GetTag(nameInRegistry, tagStr)
			if err != nil {
				log.Warnf("invalid entry for %s:%s in cache, invalidating.", nameInRegistry, imgTag.TagName)
			} else if imgTag != nil {
				logCtx.Debugf("Cache hit for %s:%s", nameInRegistry, imgTag.TagName)
				tagListLock.Lock()
				tagList.Add(imgTag)
				tagListLock.Unlock()
				wg.Done()
				continue
			}
		}

		logCtx.Tracef("Getting manifest for image %s:%s (operation %d/%d)", nameInRegistry, tagStr, i, len(tags))

		lockErr := sem.Acquire(context.TODO(), 1)
		if lockErr != nil {
			log.Warnf("could not acquire semaphore: %v", lockErr)
			wg.Done()
			continue
		}
		logCtx.Tracef("acquired metadata semaphore")

		go func(tagStr string) {
			defer func() {
				sem.Release(1)
				wg.Done()
				log.Tracef("released semaphore and terminated waitgroup")
			}()

			var ml distribution.Manifest
			var err error

			// We first try to fetch a V2 manifest, and if that's not available we fall
			// back to fetching V1 manifest. If that fails also, we just skip this tag.
			if ml, err = regClient.ManifestForTag(tagStr); err != nil {
				logCtx.Errorf("Error fetching metadata for %s:%s - neither V1 or V2 or OCI manifest returned by registry: %v", nameInRegistry, tagStr, err)
				return
			}

			// Parse required meta data from the manifest. The metadata contains all
			// information needed to decide whether to consider this tag or not.
			ti, err := regClient.TagMetadata(ml, vc.Options)
			if err != nil {
				logCtx.Errorf("error fetching metadata for %s:%s: %v", nameInRegistry, tagStr, err)
				return
			}
			if ti == nil {
				logCtx.Debugf("No metadata found for %s:%s", nameInRegistry, tagStr)
				return
			}

			logCtx.Tracef("Found date %s", ti.CreatedAt.String())
			var imgTag *tag.ImageTag
			if vc.Strategy == image.StrategyDigest {
				imgTag = tag.NewImageTag(tagStr, ti.CreatedAt, fmt.Sprintf("sha256:%x", ti.Digest))
			} else {
				imgTag = tag.NewImageTag(tagStr, ti.CreatedAt, "")
			}
			tagListLock.Lock()
			tagList.Add(imgTag)
			tagListLock.Unlock()
			endpoint.Cache.SetTag(nameInRegistry, imgTag)
		}(tagStr)
	}

	wg.Wait()
	return tagList, err
}

func (ep *RegistryEndpoint) expireCredentials() bool {
	if ep.Credentials != "" && !ep.CredsUpdated.IsZero() && ep.CredsExpire > 0 && time.Since(ep.CredsUpdated) >= ep.CredsExpire {
		ep.Username = ""
		ep.Password = ""
		return true
	}
	return false
}

// Sets endpoint credentials for this registry from a reference to a K8s secret
func (ep *RegistryEndpoint) SetEndpointCredentials(kubeClient *kube.KubernetesClient) error {
	if ep.expireCredentials() {
		log.Debugf("expired credentials for registry %s (updated:%s, expiry:%0fs)", ep.RegistryAPI, ep.CredsUpdated, ep.CredsExpire.Seconds())
	}
	if ep.Username == "" && ep.Password == "" && ep.Credentials != "" {
		credSrc, err := image.ParseCredentialSource(ep.Credentials, false)
		if err != nil {
			return err
		}

		// For fetching credentials, we must have working Kubernetes client.
		if (credSrc.Type == image.CredentialSourcePullSecret || credSrc.Type == image.CredentialSourceSecret) && kubeClient == nil {
			log.WithContext().
				AddField("registry", ep.RegistryAPI).
				Warnf("cannot use K8s credentials without Kubernetes client")
			return fmt.Errorf("could not fetch image tags")
		}

		creds, err := credSrc.FetchCredentials(ep.RegistryAPI, kubeClient)
		if err != nil {
			return err
		}

		ep.CredsUpdated = time.Now()

		ep.Username = creds.Username
		ep.Password = creds.Password
	}

	return nil
}
