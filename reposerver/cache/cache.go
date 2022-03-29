package cache

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/argoproj/gitops-engine/pkg/utils/text"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/util/argo"
	cacheutil "github.com/argoproj/argo-cd/v2/util/cache"
	"github.com/argoproj/argo-cd/v2/util/env"
	"github.com/argoproj/argo-cd/v2/util/hash"
)

var ErrCacheMiss = cacheutil.ErrCacheMiss

type Cache struct {
	cache                   *cacheutil.Cache
	repoCacheExpiration     time.Duration
	revisionCacheExpiration time.Duration
}

// ClusterRuntimeInfo holds cluster runtime information
type ClusterRuntimeInfo interface {
	// GetApiVersions returns supported api versions
	GetApiVersions() []string
	// GetKubeVersion returns cluster API version
	GetKubeVersion() string
}

func NewCache(cache *cacheutil.Cache, repoCacheExpiration time.Duration, revisionCacheExpiration time.Duration) *Cache {
	return &Cache{cache, repoCacheExpiration, revisionCacheExpiration}
}

func AddCacheFlagsToCmd(cmd *cobra.Command, opts ...func(client *redis.Client)) func() (*Cache, error) {
	var repoCacheExpiration time.Duration
	var revisionCacheExpiration time.Duration

	cmd.Flags().DurationVar(&repoCacheExpiration, "repo-cache-expiration", env.ParseDurationFromEnv("ARGOCD_REPO_CACHE_EXPIRATION", 24*time.Hour, 0, math.MaxInt64), "Cache expiration for repo state, incl. app lists, app details, manifest generation, revision meta-data")
	cmd.Flags().DurationVar(&revisionCacheExpiration, "revision-cache-expiration", env.ParseDurationFromEnv("ARGOCD_RECONCILIATION_TIMEOUT", 3*time.Minute, 0, math.MaxInt64), "Cache expiration for cached revision")

	repoFactory := cacheutil.AddCacheFlagsToCmd(cmd, opts...)

	return func() (*Cache, error) {
		cache, err := repoFactory()
		if err != nil {
			return nil, err
		}
		return NewCache(cache, repoCacheExpiration, revisionCacheExpiration), nil
	}
}

func appSourceKey(appSrc *appv1.ApplicationSource) uint32 {
	appSrc = appSrc.DeepCopy()
	if !appSrc.IsHelm() {
		appSrc.RepoURL = ""        // superseded by commitSHA
		appSrc.TargetRevision = "" // superseded by commitSHA
	}
	appSrcStr, _ := json.Marshal(appSrc)
	return hash.FNVa(string(appSrcStr))
}

func clusterRuntimeInfoKey(info ClusterRuntimeInfo) uint32 {
	if info == nil {
		return 0
	}
	apiVersions := info.GetApiVersions()
	sort.Slice(apiVersions, func(i, j int) bool {
		return apiVersions[i] < apiVersions[j]
	})
	return hash.FNVa(info.GetKubeVersion() + "|" + strings.Join(apiVersions, ","))
}

func listApps(repoURL, revision string) string {
	return fmt.Sprintf("ldir|%s|%s", repoURL, revision)
}

func (c *Cache) ListApps(repoUrl, revision string) (map[string]string, error) {
	res := make(map[string]string)
	err := c.cache.GetItem(listApps(repoUrl, revision), &res)
	return res, err
}

func (c *Cache) SetApps(repoUrl, revision string, apps map[string]string) error {
	return c.cache.SetItem(listApps(repoUrl, revision), apps, c.repoCacheExpiration, apps == nil)
}

func helmIndexRefsKey(repo string) string {
	return fmt.Sprintf("helm-index|%s", repo)
}

// SetHelmIndex stores helm repository index.yaml content to cache
func (c *Cache) SetHelmIndex(repo string, indexData []byte) error {
	return c.cache.SetItem(helmIndexRefsKey(repo), indexData, c.revisionCacheExpiration, false)
}

// GetHelmIndex retrieves helm repository index.yaml content from cache
func (c *Cache) GetHelmIndex(repo string, indexData *[]byte) error {
	return c.cache.GetItem(helmIndexRefsKey(repo), indexData)
}

func gitRefsKey(repo string) string {
	return fmt.Sprintf("git-refs|%s", repo)
}

// SetGitReferences saves resolved Git repository references to cache
func (c *Cache) SetGitReferences(repo string, references []*plumbing.Reference) error {
	var input [][2]string
	for i := range references {
		input = append(input, references[i].Strings())
	}
	return c.cache.SetItem(gitRefsKey(repo), input, c.revisionCacheExpiration, false)
}

// GetGitReferences retrieves resolved Git repository references from cache
func (c *Cache) GetGitReferences(repo string, references *[]*plumbing.Reference) error {
	var input [][2]string
	if err := c.cache.GetItem(gitRefsKey(repo), &input); err != nil {
		return err
	}
	var res []*plumbing.Reference
	for i := range input {
		res = append(res, plumbing.NewReferenceFromStrings(input[i][0], input[i][1]))
	}
	*references = res
	return nil
}

func manifestCacheKey(revision string, appSrc *appv1.ApplicationSource, namespace string, trackingMethod string, appLabelKey string, appName string, info ClusterRuntimeInfo) string {
	trackingKey := appLabelKey
	if text.FirstNonEmpty(trackingMethod, string(argo.TrackingMethodLabel)) != string(argo.TrackingMethodLabel) {
		trackingKey = trackingMethod + ":" + trackingKey
	}
	return fmt.Sprintf("mfst|%s|%s|%s|%s|%d", trackingKey, appName, revision, namespace, appSourceKey(appSrc)+clusterRuntimeInfoKey(info))
}

func (c *Cache) GetManifests(revision string, appSrc *appv1.ApplicationSource, clusterInfo ClusterRuntimeInfo, namespace string, trackingMethod string, appLabelKey string, appName string, res *CachedManifestResponse) error {
	err := c.cache.GetItem(manifestCacheKey(revision, appSrc, namespace, trackingMethod, appLabelKey, appName, clusterInfo), res)

	if err != nil {
		return err
	}

	hash, err := res.generateCacheEntryHash()
	if err != nil {
		return fmt.Errorf("Unable to generate hash value: %s", err)
	}

	// If cached result does not have manifests or the expected hash of the cache entry does not match the actual hash value...
	if hash != res.CacheEntryHash || res.ManifestResponse == nil && res.MostRecentError == "" {
		log.Warnf("Manifest hash did not match expected value or cached manifests response is empty, treating as a cache miss: %s", appName)

		err = c.DeleteManifests(revision, appSrc, clusterInfo, namespace, trackingMethod, appLabelKey, appName)
		if err != nil {
			return fmt.Errorf("Unable to delete manifest after hash mismatch, %v", err)
		}

		// Treat hash mismatches as cache misses, so that the underlying resource is reacquired
		return ErrCacheMiss
	}

	// The expected hash matches the actual hash, so remove the hash from the returned value
	res.CacheEntryHash = ""

	return nil
}

func (c *Cache) SetManifests(revision string, appSrc *appv1.ApplicationSource, clusterInfo ClusterRuntimeInfo, namespace string, trackingMethod string, appLabelKey string, appName string, res *CachedManifestResponse) error {

	// Generate and apply the cache entry hash, before writing
	if res != nil {
		res = res.shallowCopy()
		hash, err := res.generateCacheEntryHash()
		if err != nil {
			return fmt.Errorf("Unable to generate hash value: %s", err)
		}
		res.CacheEntryHash = hash
	}

	return c.cache.SetItem(manifestCacheKey(revision, appSrc, namespace, trackingMethod, appLabelKey, appName, clusterInfo), res, c.repoCacheExpiration, res == nil)
}

func (c *Cache) DeleteManifests(revision string, appSrc *appv1.ApplicationSource, clusterInfo ClusterRuntimeInfo, namespace string, trackingMethod string, appLabelKey string, appName string) error {
	return c.cache.SetItem(manifestCacheKey(revision, appSrc, namespace, trackingMethod, appLabelKey, appName, clusterInfo), "", c.repoCacheExpiration, true)
}

func appDetailsCacheKey(revision string, appSrc *appv1.ApplicationSource, trackingMethod appv1.TrackingMethod) string {
	if trackingMethod == "" {
		trackingMethod = argo.TrackingMethodLabel
	}
	return fmt.Sprintf("appdetails|%s|%d|%s", revision, appSourceKey(appSrc), trackingMethod)
}

func (c *Cache) GetAppDetails(revision string, appSrc *appv1.ApplicationSource, res *apiclient.RepoAppDetailsResponse, trackingMethod appv1.TrackingMethod) error {
	return c.cache.GetItem(appDetailsCacheKey(revision, appSrc, trackingMethod), res)
}

func (c *Cache) SetAppDetails(revision string, appSrc *appv1.ApplicationSource, res *apiclient.RepoAppDetailsResponse, trackingMethod appv1.TrackingMethod) error {
	return c.cache.SetItem(appDetailsCacheKey(revision, appSrc, trackingMethod), res, c.repoCacheExpiration, res == nil)
}

func revisionMetadataKey(repoURL, revision string) string {
	return fmt.Sprintf("revisionmetadata|%s|%s", repoURL, revision)
}

func (c *Cache) GetRevisionMetadata(repoURL, revision string) (*appv1.RevisionMetadata, error) {
	item := &appv1.RevisionMetadata{}
	return item, c.cache.GetItem(revisionMetadataKey(repoURL, revision), item)
}

func (c *Cache) SetRevisionMetadata(repoURL, revision string, item *appv1.RevisionMetadata) error {
	return c.cache.SetItem(revisionMetadataKey(repoURL, revision), item, c.repoCacheExpiration, false)
}

func (cmr *CachedManifestResponse) shallowCopy() *CachedManifestResponse {
	if cmr == nil {
		return nil
	}

	return &CachedManifestResponse{
		CacheEntryHash:                  cmr.CacheEntryHash,
		FirstFailureTimestamp:           cmr.FirstFailureTimestamp,
		ManifestResponse:                cmr.ManifestResponse,
		MostRecentError:                 cmr.MostRecentError,
		NumberOfCachedResponsesReturned: cmr.NumberOfCachedResponsesReturned,
		NumberOfConsecutiveFailures:     cmr.NumberOfConsecutiveFailures,
	}
}

func (cmr *CachedManifestResponse) generateCacheEntryHash() (string, error) {

	// Copy, then remove the old hash
	copy := cmr.shallowCopy()
	copy.CacheEntryHash = ""

	// Hash the JSON representation into a base-64-encoded FNV 64a (we don't need a cryptographic hash algorithm, since this is only for detecting data corruption)
	bytes, err := json.Marshal(copy)
	if err != nil {
		return "", err
	}
	h := fnv.New64a()
	_, err = h.Write(bytes)
	if err != nil {
		return "", err
	}
	fnvHash := h.Sum(nil)
	return base64.URLEncoding.EncodeToString(fnvHash), nil

}

// CachedManifestResponse represents a cached result of a previous manifest generation operation, including the caching
// of a manifest generation error, plus additional information on previous failures
type CachedManifestResponse struct {

	// NOTE: When adding fields to this struct, you MUST also update shallowCopy()

	CacheEntryHash                  string                      `json:"cacheEntryHash"`
	ManifestResponse                *apiclient.ManifestResponse `json:"manifestResponse"`
	MostRecentError                 string                      `json:"mostRecentError"`
	FirstFailureTimestamp           int64                       `json:"firstFailureTimestamp"`
	NumberOfConsecutiveFailures     int                         `json:"numberOfConsecutiveFailures"`
	NumberOfCachedResponsesReturned int                         `json:"numberOfCachedResponsesReturned"`
}
