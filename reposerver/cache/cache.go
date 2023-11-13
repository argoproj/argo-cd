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
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
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
var ErrCacheKeyLocked = cacheutil.ErrCacheKeyLocked

var CacheOptsDefaults = &CacheOpts{
	RepoCacheExpiration:           env.ParseDurationFromEnv("ARGOCD_REPO_CACHE_EXPIRATION", 24*time.Hour, 0, math.MaxInt64),
	RevisionCacheExpiration:       env.ParseDurationFromEnv("ARGOCD_RECONCILIATION_TIMEOUT", 3*time.Minute, 0, math.MaxInt64),
	RevisionCacheLockWaitEnabled:  env.ParseBoolFromEnv("ARGOCD_REVISION_CACHE_LOCK_WAIT_ENABLED", false),
	RevisionCacheLockTimeout:      env.ParseDurationFromEnv("ARGOCD_REVISION_CACHE_LOCK_TIMEOUT", 10*time.Second, 0, math.MaxInt64),
	RevisionCacheLockWaitInterval: env.ParseDurationFromEnv("ARGOCD_REVISION_CACHE_LOCK_WAIT_INTERVAL", 1*time.Second, 0, math.MaxInt64),
}

type CacheOpts struct {
	RepoCacheExpiration           time.Duration
	RevisionCacheExpiration       time.Duration
	RevisionCacheLockWaitEnabled  bool
	RevisionCacheLockTimeout      time.Duration
	RevisionCacheLockWaitInterval time.Duration
}

type Cache struct {
	cache                         *cacheutil.Cache
	repoCacheExpiration           time.Duration
	revisionCacheExpiration       time.Duration
	revisionCacheLockWaitEnabled  bool
	revisionCacheLockTimeout      time.Duration
	revisionCacheLockWaitInterval time.Duration
}

// ClusterRuntimeInfo holds cluster runtime information
type ClusterRuntimeInfo interface {
	// GetApiVersions returns supported api versions
	GetApiVersions() []string
	// GetKubeVersion returns cluster API version
	GetKubeVersion() string
}

func NewCache(cache *cacheutil.Cache, cacheOpts *CacheOpts) *Cache {
	if cacheOpts == nil {
		cacheOpts = CacheOptsDefaults
	}
	if cacheOpts.RevisionCacheLockWaitEnabled && cacheOpts.RevisionCacheLockWaitInterval > cacheOpts.RevisionCacheLockTimeout {
		log.Warnf("revision-cache-lock-wait-interval (%s) is greater than revision-cache-lock-timeout (%s) reducing interval to revision-cache-lock-timeout", cacheOpts.RevisionCacheLockWaitInterval, cacheOpts.RevisionCacheLockTimeout)
		cacheOpts.RevisionCacheLockWaitInterval = cacheOpts.RevisionCacheLockTimeout
	}
	return &Cache{cache,
		cacheOpts.RepoCacheExpiration,
		cacheOpts.RevisionCacheExpiration,
		cacheOpts.RevisionCacheLockWaitEnabled,
		cacheOpts.RevisionCacheLockTimeout,
		cacheOpts.RevisionCacheLockWaitInterval}
}

func AddCacheFlagsToCmd(cmd *cobra.Command, opts ...func(client *redis.Client)) func() (*Cache, error) {
	var cacheOpts *CacheOpts = &CacheOpts{}

	cmd.Flags().DurationVar(&cacheOpts.RepoCacheExpiration, "repo-cache-expiration", CacheOptsDefaults.RepoCacheExpiration, "Cache expiration for repo state, incl. app lists, app details, manifest generation, revision meta-data")
	cmd.Flags().DurationVar(&cacheOpts.RevisionCacheExpiration, "revision-cache-expiration", CacheOptsDefaults.RevisionCacheExpiration, "Cache expiration for cached revision")
	cmd.Flags().BoolVar(&cacheOpts.RevisionCacheLockWaitEnabled, "enable-revision-cache-lock", CacheOptsDefaults.RevisionCacheLockWaitEnabled, "Enables a lock to prevent duplicate git requests during revision cache updates")
	cmd.Flags().DurationVar(&cacheOpts.RevisionCacheLockTimeout, "revision-cache-lock-timeout", CacheOptsDefaults.RevisionCacheLockTimeout, "Specifies the max amount of time each routine making identical git requests will wait for the git reference cache to update before making a new git requests")
	cmd.Flags().DurationVar(&cacheOpts.RevisionCacheLockWaitInterval, "revision-cache-lock-wait-interval", CacheOptsDefaults.RevisionCacheLockWaitInterval, "Specifies the amount of time between each check to see if the git cache refs lock has been released")

	repoFactory := cacheutil.AddCacheFlagsToCmd(cmd, opts...)

	return func() (*Cache, error) {
		cache, err := repoFactory()
		if err != nil {
			return nil, fmt.Errorf("error adding cache flags to cmd: %w", err)
		}
		return NewCache(cache, cacheOpts), nil
	}
}

type refTargetForCacheKey struct {
	RepoURL        string `json:"repoURL"`
	Project        string `json:"project"`
	TargetRevision string `json:"targetRevision"`
	Chart          string `json:"chart"`
}

func refTargetForCacheKeyFromRefTarget(refTarget *appv1.RefTarget) refTargetForCacheKey {
	return refTargetForCacheKey{
		RepoURL:        refTarget.Repo.Repo,
		Project:        refTarget.Repo.Project,
		TargetRevision: refTarget.TargetRevision,
		Chart:          refTarget.Chart,
	}
}

type refTargetRevisionMappingForCacheKey map[string]refTargetForCacheKey

func getRefTargetRevisionMappingForCacheKey(refTargetRevisionMapping appv1.RefTargetRevisionMapping) refTargetRevisionMappingForCacheKey {
	res := make(refTargetRevisionMappingForCacheKey)
	for k, v := range refTargetRevisionMapping {
		res[k] = refTargetForCacheKeyFromRefTarget(v)
	}
	return res
}

func appSourceKey(appSrc *appv1.ApplicationSource, srcRefs appv1.RefTargetRevisionMapping, refSourceCommitSHAs ResolvedRevisions) uint32 {
	return hash.FNVa(appSourceKeyJSON(appSrc, srcRefs, refSourceCommitSHAs))
}

// ResolvedRevisions is a map of "normalized git URL" -> "git commit SHA". When one source references another source,
// the referenced source revision may change, for example, when someone pushes a commit to the referenced branch. This
// map lets us keep track of the current revision for each referenced source.
type ResolvedRevisions map[string]string

type appSourceKeyStruct struct {
	AppSrc            *appv1.ApplicationSource            `json:"appSrc"`
	SrcRefs           refTargetRevisionMappingForCacheKey `json:"srcRefs"`
	ResolvedRevisions ResolvedRevisions                   `json:"resolvedRevisions,omitempty"`
}

func appSourceKeyJSON(appSrc *appv1.ApplicationSource, srcRefs appv1.RefTargetRevisionMapping, refSourceCommitSHAs ResolvedRevisions) string {
	appSrc = appSrc.DeepCopy()
	if !appSrc.IsHelm() {
		appSrc.RepoURL = ""        // superseded by commitSHA
		appSrc.TargetRevision = "" // superseded by commitSHA
	}
	appSrcStr, _ := json.Marshal(appSourceKeyStruct{
		AppSrc:            appSrc,
		SrcRefs:           getRefTargetRevisionMappingForCacheKey(srcRefs),
		ResolvedRevisions: refSourceCommitSHAs,
	})
	return string(appSrcStr)
}

func clusterRuntimeInfoKey(info ClusterRuntimeInfo) uint32 {
	if info == nil {
		return 0
	}
	key := clusterRuntimeInfoKeyUnhashed(info)
	return hash.FNVa(key)
}

// clusterRuntimeInfoKeyUnhashed gets the cluster runtime info for a cache key, but does not hash the info. Does not
// check if info is nil, the caller must do that.
func clusterRuntimeInfoKeyUnhashed(info ClusterRuntimeInfo) string {
	apiVersions := info.GetApiVersions()
	sort.Slice(apiVersions, func(i, j int) bool {
		return apiVersions[i] < apiVersions[j]
	})
	return info.GetKubeVersion() + "|" + strings.Join(apiVersions, ",")
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

func GitRefCacheItemToReferences(cacheItem [][2]string) []*plumbing.Reference {
	var res []*plumbing.Reference
	for i := range cacheItem {
		res = append(res, plumbing.NewReferenceFromStrings(cacheItem[i][0], cacheItem[i][1]))
	}
	return res
}

// GetGitReferences retrieves resolved Git repository references from cache
func (c *Cache) GetGitReferences(repo string, references *[]*plumbing.Reference) error {
	var input [][2]string
	if err := c.cache.GetItem(gitRefsKey(repo), &input); err != nil {
		return err
	}
	*references = GitRefCacheItemToReferences(input)
	return nil
}

// TryLockGitRefCache attempts to lock the key for the Git repository references if the key doesn't exist
func (c *Cache) TryLockGitRefCache(repo string, lockId string) {
	err := c.cache.SetCacheItem(&cacheutil.Item{
		Key:              gitRefsKey(repo),
		Object:           [][2]string{{cacheutil.CacheLockedValue, lockId}},
		Expiration:       c.revisionCacheLockTimeout,
		DisableOverwrite: true,
	}, false)
	if err != nil {
		// The key already existing does not produce an error for go-redis so we'll use the get to validate
		// In memory would produce an error, but ignoring provides a consistent flow
		log.Debug("Error setting git references cache lock: ", err)
	}
}

// GetOrLockGitReferences retrieves the git references if they exist, otherwise creates a lock and returns so the caller can populate the cache
func (c *Cache) GetOrLockGitReferences(repo string, references *[]*plumbing.Reference) (updateCache bool, lockId string, err error) {
	// feature flagged for now
	if !c.revisionCacheLockWaitEnabled {
		log.Debug("Git references cache lock is disabled")
		err = c.GetGitReferences(repo, references)
		if err == ErrCacheMiss {
			// In the previous iteration this would be fine, but since the caller now expects a valid value unless it obtained the lock
			// we need to absorb the error so that the caller continues to refresh the cache
			err = nil
		}
		return true, "", err
	}
	var input [][2]string
	myLockUUID, err := uuid.NewRandom()
	if err != nil {
		log.Debug("Error generating git references cache lock id: ", err)
		return false, "", err
	}
	// We need to be able to identify that our lock was the successful one, otherwise we'll still have duplicate requests
	myLockId := myLockUUID.String()
	waitUntil := time.Now().Add(c.revisionCacheLockTimeout)
	// Wait only the maximum amount of time configured for the lock
	for time.Now().Before(waitUntil) {
		// Attempt to get the lock
		c.TryLockGitRefCache(repo, myLockId)
		err = c.cache.GetItem(gitRefsKey(repo), &input)
		if err == nil && len(input) > 0 && len(input[0]) > 0 {
			if input[0][0] != cacheutil.CacheLockedValue {
				// Valid value in cache, convert to plumbing.Reference and return
				*references = GitRefCacheItemToReferences(input)
				return false, myLockId, err
			} else if input[0][1] == myLockId {
				// Our lock was successful
				return true, myLockId, nil
			}
		}
		// Wait for lock, valid value, or timeout
		time.Sleep(c.revisionCacheLockWaitInterval)
	}
	// Timeout waiting for lock
	log.Debug("Repository cache was unable to acquire lock or valid data within timeout")
	return true, myLockId, nil
}

// UnlockGitReferences unlocks the key for the Git repository references if needed
func (c *Cache) UnlockGitReferences(repo string, lockId string) error {
	var input [][2]string
	var err error
	if err = c.cache.GetItem(gitRefsKey(repo), &input); err == nil &&
		len(input) > 0 &&
		len(input[0]) > 1 &&
		input[0][0] == cacheutil.CacheLockedValue &&
		input[0][1] == lockId {
		// We have the lock, so remove it
		return c.cache.SetItem(gitRefsKey(repo), input, 0, true)
	}
	return err
}

// refSourceCommitSHAs is a list of resolved revisions for each ref source. This allows us to invalidate the cache
// when someone pushes a commit to a source which is referenced from the main source (the one referred to by `revision`).
func manifestCacheKey(revision string, appSrc *appv1.ApplicationSource, srcRefs appv1.RefTargetRevisionMapping, namespace string, trackingMethod string, appLabelKey string, appName string, info ClusterRuntimeInfo, refSourceCommitSHAs ResolvedRevisions) string {
	// TODO: this function is getting unwieldy. We should probably consolidate some of this stuff into a struct. For
	//       example, revision could be part of ResolvedRevisions. And srcRefs is probably redundant now that
	//       refSourceCommitSHAs has been added. We don't need to know the _target_ revisions of the referenced sources
	//       when the _resolved_ revisions are already part of the key.
	trackingKey := trackingKey(appLabelKey, trackingMethod)
	return fmt.Sprintf("mfst|%s|%s|%s|%s|%d", trackingKey, appName, revision, namespace, appSourceKey(appSrc, srcRefs, refSourceCommitSHAs)+clusterRuntimeInfoKey(info))
}

func trackingKey(appLabelKey string, trackingMethod string) string {
	trackingKey := appLabelKey
	if text.FirstNonEmpty(trackingMethod, string(argo.TrackingMethodLabel)) != string(argo.TrackingMethodLabel) {
		trackingKey = trackingMethod + ":" + trackingKey
	}
	return trackingKey
}

// LogDebugManifestCacheKeyFields logs all the information included in a manifest cache key. It's intended to be run
// before every manifest cache operation to help debug cache misses.
func LogDebugManifestCacheKeyFields(message string, reason string, revision string, appSrc *appv1.ApplicationSource, srcRefs appv1.RefTargetRevisionMapping, clusterInfo ClusterRuntimeInfo, namespace string, trackingMethod string, appLabelKey string, appName string, refSourceCommitSHAs ResolvedRevisions) {
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithFields(log.Fields{
			"revision":    revision,
			"appSrc":      appSourceKeyJSON(appSrc, srcRefs, refSourceCommitSHAs),
			"namespace":   namespace,
			"trackingKey": trackingKey(appLabelKey, trackingMethod),
			"appName":     appName,
			"clusterInfo": clusterRuntimeInfoKeyUnhashed(clusterInfo),
			"reason":      reason,
		}).Debug(message)
	}
}

func (c *Cache) GetManifests(revision string, appSrc *appv1.ApplicationSource, srcRefs appv1.RefTargetRevisionMapping, clusterInfo ClusterRuntimeInfo, namespace string, trackingMethod string, appLabelKey string, appName string, res *CachedManifestResponse, refSourceCommitSHAs ResolvedRevisions) error {
	err := c.cache.GetItem(manifestCacheKey(revision, appSrc, srcRefs, namespace, trackingMethod, appLabelKey, appName, clusterInfo, refSourceCommitSHAs), res)

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

		LogDebugManifestCacheKeyFields("deleting manifests cache", "manifest hash did not match or cached response is empty", revision, appSrc, srcRefs, clusterInfo, namespace, trackingMethod, appLabelKey, appName, refSourceCommitSHAs)

		err = c.DeleteManifests(revision, appSrc, srcRefs, clusterInfo, namespace, trackingMethod, appLabelKey, appName, refSourceCommitSHAs)
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

func (c *Cache) SetManifests(revision string, appSrc *appv1.ApplicationSource, srcRefs appv1.RefTargetRevisionMapping, clusterInfo ClusterRuntimeInfo, namespace string, trackingMethod string, appLabelKey string, appName string, res *CachedManifestResponse, refSourceCommitSHAs ResolvedRevisions) error {
	// Generate and apply the cache entry hash, before writing
	if res != nil {
		res = res.shallowCopy()
		hash, err := res.generateCacheEntryHash()
		if err != nil {
			return fmt.Errorf("Unable to generate hash value: %s", err)
		}
		res.CacheEntryHash = hash
	}

	return c.cache.SetItem(manifestCacheKey(revision, appSrc, srcRefs, namespace, trackingMethod, appLabelKey, appName, clusterInfo, refSourceCommitSHAs), res, c.repoCacheExpiration, res == nil)
}

func (c *Cache) DeleteManifests(revision string, appSrc *appv1.ApplicationSource, srcRefs appv1.RefTargetRevisionMapping, clusterInfo ClusterRuntimeInfo, namespace, trackingMethod, appLabelKey, appName string, refSourceCommitSHAs ResolvedRevisions) error {
	return c.cache.SetItem(manifestCacheKey(revision, appSrc, srcRefs, namespace, trackingMethod, appLabelKey, appName, clusterInfo, refSourceCommitSHAs), "", c.repoCacheExpiration, true)
}

func appDetailsCacheKey(revision string, appSrc *appv1.ApplicationSource, srcRefs appv1.RefTargetRevisionMapping, trackingMethod appv1.TrackingMethod, refSourceCommitSHAs ResolvedRevisions) string {
	if trackingMethod == "" {
		trackingMethod = argo.TrackingMethodLabel
	}
	return fmt.Sprintf("appdetails|%s|%d|%s", revision, appSourceKey(appSrc, srcRefs, refSourceCommitSHAs), trackingMethod)
}

func (c *Cache) GetAppDetails(revision string, appSrc *appv1.ApplicationSource, srcRefs appv1.RefTargetRevisionMapping, res *apiclient.RepoAppDetailsResponse, trackingMethod appv1.TrackingMethod, refSourceCommitSHAs ResolvedRevisions) error {
	return c.cache.GetItem(appDetailsCacheKey(revision, appSrc, srcRefs, trackingMethod, refSourceCommitSHAs), res)
}

func (c *Cache) SetAppDetails(revision string, appSrc *appv1.ApplicationSource, srcRefs appv1.RefTargetRevisionMapping, res *apiclient.RepoAppDetailsResponse, trackingMethod appv1.TrackingMethod, refSourceCommitSHAs ResolvedRevisions) error {
	return c.cache.SetItem(appDetailsCacheKey(revision, appSrc, srcRefs, trackingMethod, refSourceCommitSHAs), res, c.repoCacheExpiration, res == nil)
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

func revisionChartDetailsKey(repoURL, chart, revision string) string {
	return fmt.Sprintf("chartdetails|%s|%s|%s", repoURL, chart, revision)
}

func (c *Cache) GetRevisionChartDetails(repoURL, chart, revision string) (*appv1.ChartDetails, error) {
	item := &appv1.ChartDetails{}
	return item, c.cache.GetItem(revisionChartDetailsKey(repoURL, chart, revision), item)
}

func (c *Cache) SetRevisionChartDetails(repoURL, chart, revision string, item *appv1.ChartDetails) error {
	return c.cache.SetItem(revisionChartDetailsKey(repoURL, chart, revision), item, c.repoCacheExpiration, false)
}

func gitFilesKey(repoURL, revision, pattern string) string {
	return fmt.Sprintf("gitfiles|%s|%s|%s", repoURL, revision, pattern)
}

func (c *Cache) SetGitFiles(repoURL, revision, pattern string, files map[string][]byte) error {
	return c.cache.SetItem(gitFilesKey(repoURL, revision, pattern), &files, c.repoCacheExpiration, false)
}

func (c *Cache) GetGitFiles(repoURL, revision, pattern string) (map[string][]byte, error) {
	var item map[string][]byte
	return item, c.cache.GetItem(gitFilesKey(repoURL, revision, pattern), &item)
}

func gitDirectoriesKey(repoURL, revision string) string {
	return fmt.Sprintf("gitdirs|%s|%s", repoURL, revision)
}

func (c *Cache) SetGitDirectories(repoURL, revision string, directories []string) error {
	return c.cache.SetItem(gitDirectoriesKey(repoURL, revision), &directories, c.repoCacheExpiration, false)
}

func (c *Cache) GetGitDirectories(repoURL, revision string) ([]string, error) {
	var item []string
	return item, c.cache.GetItem(gitDirectoriesKey(repoURL, revision), &item)
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
