package cache

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/text"
	"github.com/go-git/go-git/v5/plumbing"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	cacheutil "github.com/argoproj/argo-cd/v3/util/cache"
	cryptoutil "github.com/argoproj/argo-cd/v3/util/crypto"
	"github.com/argoproj/argo-cd/v3/util/env"
	"github.com/argoproj/argo-cd/v3/util/git"
	"github.com/argoproj/argo-cd/v3/util/hash"
	"github.com/argoproj/argo-cd/v3/util/helm"
)

var (
	ErrCacheMiss      = cacheutil.ErrCacheMiss
	ErrCacheKeyLocked = cacheutil.ErrCacheKeyLocked
)

type Cache struct {
	cache                    *cacheutil.Cache
	repoCacheExpiration      time.Duration
	revisionCacheExpiration  time.Duration
	revisionCacheLockTimeout time.Duration
	helmIndexCacheExpiration time.Duration
}

// GetRepoCacheExpiration returns the repository cache expiration duration.
func (c *Cache) GetRepoCacheExpiration() time.Duration {
	return c.repoCacheExpiration
}

// ClusterRuntimeInfo holds cluster runtime information
type ClusterRuntimeInfo interface {
	// GetApiVersions returns supported api versions
	GetApiVersions() []string
	// GetKubeVersion returns cluster API version
	GetKubeVersion() string
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

func NewCache(cache *cacheutil.Cache, repoCacheExpiration time.Duration, revisionCacheExpiration time.Duration, revisionCacheLockTimeout time.Duration) *Cache {
	return &Cache{cache, repoCacheExpiration, revisionCacheExpiration, revisionCacheLockTimeout, helm.DefaultIndexCacheExpiration}
}

// GetHelmIndexCacheExpiration returns the freshness window applied to cached
// helm repository index records.
func (c *Cache) GetHelmIndexCacheExpiration() time.Duration {
	return c.helmIndexCacheExpiration
}

// SetHelmIndexCacheExpiration sets the freshness window for cached helm
// repository index records. A value of 0 revalidates with the origin on
// every use; negative values fall back to the default.
func (c *Cache) SetHelmIndexCacheExpiration(expiration time.Duration) {
	if expiration < 0 {
		expiration = helm.DefaultIndexCacheExpiration
	}
	c.helmIndexCacheExpiration = expiration
}

func AddCacheFlagsToCmd(cmd *cobra.Command, opts ...cacheutil.Options) func() (*Cache, error) {
	var repoCacheExpiration time.Duration
	var revisionCacheExpiration time.Duration
	var revisionCacheLockTimeout time.Duration

	cmd.Flags().DurationVar(&repoCacheExpiration, "repo-cache-expiration", env.ParseDurationFromEnv("ARGOCD_REPO_CACHE_EXPIRATION", 24*time.Hour, 0, math.MaxInt64), "Cache expiration for repo state, incl. app lists, app details, manifest generation, revision meta-data")
	cmd.Flags().DurationVar(&revisionCacheExpiration, "revision-cache-expiration", env.ParseDurationFromEnv("ARGOCD_RECONCILIATION_TIMEOUT", 3*time.Minute, 0, math.MaxInt64), "Cache expiration for cached revision")
	cmd.Flags().DurationVar(&revisionCacheLockTimeout, "revision-cache-lock-timeout", env.ParseDurationFromEnv("ARGOCD_REVISION_CACHE_LOCK_TIMEOUT", 10*time.Second, 0, math.MaxInt64), "Cache TTL for locks to prevent duplicate requests on revisions, set to 0 to disable")

	repoFactory := cacheutil.AddCacheFlagsToCmd(cmd, opts...)

	return func() (*Cache, error) {
		cache, err := repoFactory()
		if err != nil {
			return nil, fmt.Errorf("error adding cache flags to cmd: %w", err)
		}
		return NewCache(cache, repoCacheExpiration, revisionCacheExpiration, revisionCacheLockTimeout), nil
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

func getRefTargetRevisionMappingForCacheKey(refTargetRevisionMapping appv1.RefTargetRevisionMapping, refSourceCommitSHAs ResolvedRevisions) refTargetRevisionMappingForCacheKey {
	res := make(refTargetRevisionMappingForCacheKey)

	for k, v := range refTargetRevisionMapping {
		// forcefully update TargetRevision based on refSourceCommitSHAs so that the resolved revision is always stored in the cache
		v.TargetRevision = refSourceCommitSHAs[git.NormalizeGitURL(v.Repo.Repo)]
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
		SrcRefs:           getRefTargetRevisionMappingForCacheKey(srcRefs, refSourceCommitSHAs),
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
	slices.Sort(apiVersions)
	return info.GetKubeVersion() + "|" + strings.Join(apiVersions, ",")
}

func listApps(repoURL, revision string) string {
	return fmt.Sprintf("ldir|%s|%s", repoURL, revision)
}

func (c *Cache) ListApps(repoURL, revision string) (map[string]string, error) {
	res := make(map[string]string)
	err := c.cache.GetItem(listApps(repoURL, revision), &res)
	return res, err
}

func (c *Cache) SetApps(repoURL, revision string, apps map[string]string) error {
	return c.cache.SetItem(
		listApps(repoURL, revision),
		apps,
		&cacheutil.CacheActionOpts{
			Expiration: c.repoCacheExpiration,
			Delete:     apps == nil,
		})
}

// The v3 record keys isolate credentialed fetches from anonymous ones: the
// credential-scope fingerprint is part of the key prefix, so a cache entry
// published with private credentials can never be served to a reader whose
// resolved credential snapshot differs.
func helmIndexKeyPrefix(ctx context.Context, repo string) (string, error) {
	scope, err := cryptoutil.CredentialScopeContext(ctx, "helm-index-record-v3", repo)
	if err != nil {
		return "", err
	}
	return "helm-index-v3|{" + scope + "}|", nil
}

// The record key names one atomic unit: index body plus the HTTP validators
// the origin returned with it, so a 304 can never pair a new validator with
// a previous body.
func helmIndexRecordKey(repo string) string {
	prefix, _ := helmIndexKeyPrefix(context.Background(), repo)
	return prefix + "record"
}

// helmIndexMetaKey names a small metadata-only copy of the record (no body).
// It lets readers with a verified local body confirm the shared generation
// and refresh validators without transferring the full index on every build.
// Absence of this key always falls back to the full-record read, so it is
// safe with legacy writers and partial publications.
func helmIndexMetaKey(repo string) string {
	prefix, _ := helmIndexKeyPrefix(context.Background(), repo)
	return prefix + "meta"
}

// helmIndexMeta is the metadata-only projection of HelmIndexRecord stored at
// helmIndexMetaKey. Body is never populated here; BodyDigest identifies the
// body stored under the record key. The JSON tags mirror the shared record's
// so the same payload shape round-trips through the cache client.
type helmIndexMeta = helm.HelmIndexMeta

// HelmIndexMeta is the exported metadata-only projection of a helm index
// record (no body), defined in util/helm beside the record type.

type HelmIndexRecord = helm.HelmIndexRecord

// SetHelmIndex stores helm repository index.yaml content to cache
func (c *Cache) SetHelmIndex(repo string, indexData []byte) error {
	if indexData == nil {
		// Logged as warning upstream
		return errors.New("helm index data is nil, skipping cache")
	}
	return c.SetHelmIndexRecord(repo, indexData, "", "")
}

// GetHelmIndex retrieves helm repository index.yaml content from cache.
// The refs key is a raw legacy entry without a trusted validation time, so
// only fresh versioned records are served here; unversioned entries always
// miss and force an unconditional origin GET.
func (c *Cache) GetHelmIndex(repo string, indexData *[]byte) error {
	if c.helmIndexCacheExpiration <= 0 {
		return ErrCacheMiss
	}
	record, err := c.GetHelmIndexRetainedRecord(repo)
	if err == nil && len(record.Body) > 0 {
		if !record.Fresh(c.helmIndexCacheExpiration) {
			return ErrCacheMiss
		}
		*indexData = record.Body
		return nil
	}
	if record != nil || (err != nil && !errors.Is(err, ErrCacheMiss)) {
		return err
	}
	return ErrCacheMiss // A raw legacy entry has no trusted validation time.
}

// SetHelmIndexRecord stores the index body and its HTTP validators as one
// versioned record keyed by the credential scope of the resolved fetch
// identity.
func (c *Cache) SetHelmIndexRecord(repo string, body []byte, etag, modified string) error {
	if body == nil {
		return errors.New("helm index data is nil, skipping cache")
	}
	record := &HelmIndexRecord{Body: body, ETag: etag, LastModified: modified, StoredAt: time.Now().UTC().Format(time.RFC3339Nano)}
	digest := sha256.Sum256(body)
	record.BodyDigest = hex.EncodeToString(digest[:])
	return c.SetHelmIndexRetainedRecord(repo, record)
}

// SetHelmIndexRetainedRecord writes a complete record (maintenance path).
func (c *Cache) SetHelmIndexRetainedRecord(repo string, record *HelmIndexRecord) error {
	if record == nil || len(record.Body) == 0 {
		return errors.New("helm index data is nil, skipping cache")
	}
	if record.BodyDigest == "" {
		digest := sha256.Sum256(record.Body)
		record.BodyDigest = hex.EncodeToString(digest[:])
	}
	// There is no legacy body mirror: pre-record replicas read an unscoped
	// "helm-index|<repo>" key, which must not carry credential-scoped bodies,
	// so they fetch from the origin on a miss exactly as before.
	// The record must OUTLIVE the freshness window: once a record is stale it
	// is revalidated by a conditional GET, which needs the stored body and
	// validators. TTL equal to freshness would delete exactly when
	// revalidation starts, turning every post-expiry check into a full 200.
	if err := c.cache.SetItem(helmIndexRecordKey(repo), record, &cacheutil.CacheActionOpts{Expiration: helmIndexRecordRetention(c.helmIndexCacheExpiration)}); err != nil {
		return err
	}
	// Publish the small metadata projection last. Readers that already hold a
	// verified body use it to skip the full-record transfer; a missing or
	// stale meta entry only costs the fallback read. Best-effort with one
	// retry: a failed meta write never invalidates the record just published,
	// but a persistent failure would leave readers on their current body for
	// up to one freshness window, so the write is attempted twice.
	meta := helmIndexMeta{BodyDigest: record.BodyDigest, ETag: record.ETag, LastModified: record.LastModified, URLIdentity: record.URLIdentity, Generation: record.Generation, StoredAt: record.StoredAt}
	metaOpts := &cacheutil.CacheActionOpts{Expiration: helmIndexRecordRetention(c.helmIndexCacheExpiration)}
	if err := c.cache.SetItem(helmIndexMetaKey(repo), meta, metaOpts); err != nil {
		// One immediate retry shrinks the stale-meta divergence window when
		// the first write hit a transient backend failure.
		if retryErr := c.cache.SetItem(helmIndexMetaKey(repo), meta, metaOpts); retryErr != nil {
			return err
		}
	}
	return nil
}

// helmIndexRecordRetention is how long a versioned record stays in Redis
// after publication: the freshness window plus one window of revalidation
// grace, floored at one hour so very short windows still get a usable
// revalidation period.
func helmIndexRecordRetention(freshness time.Duration) time.Duration {
	if freshness <= 0 {
		freshness = helm.DefaultIndexCacheExpiration
	}
	return max(2*freshness, time.Hour)
}

func (c *Cache) GetHelmIndexRetainedRecord(repo string) (*HelmIndexRecord, error) {
	return c.GetHelmIndexRetainedRecordContext(context.Background(), repo)
}

// GetHelmIndexMeta fetches the small metadata-only projection published by
// SetHelmIndexRetainedRecord. A miss returns ErrCacheMiss and callers must
// fall back to the full-record read. Satisfies helm.IndexMetaCache.
func (c *Cache) GetHelmIndexMeta(repo string) (*helm.HelmIndexMeta, error) {
	var meta helmIndexMeta
	if err := c.cache.GetItem(helmIndexMetaKey(repo), &meta); err != nil {
		return nil, err
	}
	if meta.BodyDigest == "" {
		return nil, cacheutil.ErrCacheMiss
	}
	return &meta, nil
}

// GetHelmIndexMetaContext is the deadline-aware variant: the read is
// abandoned once the caller's budget expires, keeping the request free to
// reach the origin. Satisfies helm.IndexMetaCacheContext.
func (c *Cache) GetHelmIndexMetaContext(ctx context.Context, repo string) (*helm.HelmIndexMeta, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	type result struct {
		meta *helm.HelmIndexMeta
		err  error
	}
	done := make(chan result, 1)
	go func() {
		meta, err := c.GetHelmIndexMeta(repo)
		done <- result{meta, err}
	}()
	select {
	case res := <-done:
		return res.meta, res.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// GetHelmIndexRetainedRecordContext honors the caller's context while the
// Redis read is in flight, so a slow backend cannot hold a per-key index
// lock past cancellation. The underlying read is abandoned, not cancelled —
// the shared CacheClient interface has no context-aware Get.
func (c *Cache) GetHelmIndexRetainedRecordContext(ctx context.Context, repo string) (*HelmIndexRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	type result struct {
		record *HelmIndexRecord
		err    error
	}
	done := make(chan result, 1)
	go func() {
		record, err := c.readHelmIndex(repo)
		done <- result{record, err}
	}()
	select {
	case res := <-done:
		return res.record, res.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// GetHelmIndexFreshRecordContext returns the index record in one shared read,
// but only while the record is within the freshness window. Freshness and
// request identity must come from the same record: deriving them from two
// reads lets an overlapping writer's renewed freshness authorize a body whose
// own validation time had already expired.
func (c *Cache) GetHelmIndexFreshRecordContext(ctx context.Context, repo string) (*HelmIndexRecord, error) {
	record, err := c.GetHelmIndexRetainedRecordContext(ctx, repo)
	if err != nil {
		return nil, err
	}
	if c.helmIndexCacheExpiration <= 0 || len(record.Body) == 0 || !record.Fresh(c.helmIndexCacheExpiration) {
		return nil, ErrCacheMiss
	}
	return record, nil
}

func (c *Cache) readHelmIndex(repo string) (*HelmIndexRecord, error) {
	var record HelmIndexRecord
	if err := c.cache.GetItem(helmIndexRecordKey(repo), &record); err != nil {
		return nil, err
	}
	if len(record.Body) == 0 {
		return nil, ErrCacheMiss
	}
	if record.BodyDigest != "" {
		digest := sha256.Sum256(record.Body)
		if hex.EncodeToString(digest[:]) != record.BodyDigest {
			return nil, errors.New("helm index body does not match its published digest")
		}
	}
	return &record, nil
}

func ociTagsKey(repo string) string {
	return "oci-tags|" + repo
}

// SetOCITags stores oci image tags to cache
func (c *Cache) SetOCITags(repo string, indexData []byte) error {
	if indexData == nil {
		// Logged as warning upstream
		return errors.New("oci index data is nil, skipping cache")
	}
	return c.cache.SetItem(
		ociTagsKey(repo),
		indexData,
		&cacheutil.CacheActionOpts{Expiration: c.revisionCacheExpiration})
}

// GetOCITags retrieves oci image tags from cache
func (c *Cache) GetOCITags(repo string, indexData *[]byte) error {
	return c.cache.GetItem(ociTagsKey(repo), indexData)
}

func gitRefsKey(repo string) string {
	return "git-refs|" + repo
}

// SetGitReferences saves resolved Git repository references to cache
func (c *Cache) SetGitReferences(repo string, references []*plumbing.Reference) error {
	var input [][2]string
	for i := range references {
		input = append(input, references[i].Strings())
	}
	return c.cache.SetItem(gitRefsKey(repo), input, &cacheutil.CacheActionOpts{Expiration: c.revisionCacheExpiration})
}

// Converts raw cache items to plumbing.Reference objects
func GitRefCacheItemToReferences(cacheItem [][2]string) *[]*plumbing.Reference {
	var res []*plumbing.Reference
	for i := range cacheItem {
		// Skip empty data
		if cacheItem[i][0] != "" || cacheItem[i][1] != "" {
			res = append(res, plumbing.NewReferenceFromStrings(cacheItem[i][0], cacheItem[i][1]))
		}
	}
	return &res
}

// TryLockGitRefCache attempts to lock the key for the Git repository references if the key doesn't exist, returns the value of
// GetGitReferences after calling the SET
func (c *Cache) TryLockGitRefCache(repo string, lockId string, references *[]*plumbing.Reference) (string, error) {
	// This try set with DisableOverwrite is important for making sure that only one process is able to claim ownership
	// A normal get + set, or just set would cause ownership to go to whoever the last writer was, and during race conditions
	// leads to duplicate requests
	err := c.cache.SetItem(gitRefsKey(repo), [][2]string{{cacheutil.CacheLockedValue, lockId}}, &cacheutil.CacheActionOpts{
		Expiration:       c.revisionCacheLockTimeout,
		DisableOverwrite: true,
	})
	if err != nil {
		// Log but ignore this error since we'll want to retry, failing to obtain the lock should not throw an error
		log.Errorf("Error attempting to acquire git references cache lock: %v", err)
	}
	return c.GetGitReferences(repo, references)
}

// Retrieves the cache item for git repo references. Returns foundLockId, error
func (c *Cache) GetGitReferences(repo string, references *[]*plumbing.Reference) (string, error) {
	var input [][2]string
	err := c.cache.GetItem(gitRefsKey(repo), &input)
	valueExists := len(input) > 0 && len(input[0]) > 0
	switch {
	// Unexpected Error
	case err != nil && !errors.Is(err, ErrCacheMiss):
		log.Errorf("Error attempting to retrieve git references from cache: %v", err)
		return "", err
	// Value is set
	case valueExists && input[0][0] != cacheutil.CacheLockedValue:
		*references = *GitRefCacheItemToReferences(input)
		return "", nil
	// Key is locked
	case valueExists:
		return input[0][1], nil
	// No key or empty key
	default:
		return "", nil
	}
}

// GetOrLockGitReferences retrieves the git references if they exist, otherwise creates a lock and returns so the caller can populate the cache
// Returns isLockOwner, localLockId, error
func (c *Cache) GetOrLockGitReferences(repo string, lockId string, references *[]*plumbing.Reference) (string, error) {
	// Value matches the ttl on the lock in TryLockGitRefCache
	waitUntil := time.Now().Add(c.revisionCacheLockTimeout)
	// Wait only the maximum amount of time configured for the lock
	// if the configured time is zero then the for loop will never run and instead act as the owner immediately
	for time.Now().Before(waitUntil) {
		// Get current cache state
		if foundLockId, err := c.GetGitReferences(repo, references); foundLockId == lockId || err != nil || (references != nil && len(*references) > 0) {
			return foundLockId, err
		}
		if foundLockId, err := c.TryLockGitRefCache(repo, lockId, references); foundLockId == lockId || err != nil || (references != nil && len(*references) > 0) {
			return foundLockId, err
		}
		time.Sleep(1 * time.Second)
	}
	// If configured time is 0 then this is expected
	if c.revisionCacheLockTimeout > 0 {
		log.Debug("Repository cache was unable to acquire lock or valid data within timeout")
	}
	// Timeout waiting for lock
	return lockId, nil
}

// UnlockGitReferences unlocks the key for the Git repository references if needed
func (c *Cache) UnlockGitReferences(repo string, lockId string) error {
	var input [][2]string
	var err error
	if err = c.cache.GetItem(gitRefsKey(repo), &input); err == nil &&
		input != nil &&
		len(input) > 0 &&
		len(input[0]) > 1 &&
		input[0][0] == cacheutil.CacheLockedValue &&
		input[0][1] == lockId {
		// We have the lock, so remove it
		return c.cache.SetItem(gitRefsKey(repo), input, &cacheutil.CacheActionOpts{Delete: true})
	}
	return err
}

// manifestKey carries all fields required to build a manifests cache key.
type manifestKey struct {
	Revision       string
	AppSource      *appv1.ApplicationSource
	RefSources     appv1.RefTargetRevisionMapping
	ClusterInfo    ClusterRuntimeInfo
	Namespace      string
	TrackingMethod string
	AppLabelKey    string
	AppName        string
	// RefSourceCommitSHAs is a list of resolved revisions for each ref source. This allows us to invalidate the cache
	// when someone pushes a commit to a source which is referenced from the main source (the one referred to by `revision`).
	RefSourceCommitSHAs ResolvedRevisions
	InstallationID      string
	SourceIntegrity     *appv1.SourceIntegrity
}

func NewManifestKey(
	revision string,
	appSource *appv1.ApplicationSource,
	refSources map[string]*appv1.RefTarget,
	namespace string,
	trackingMethod string,
	appLabelKey string,
	appName string,
	installationID string,
	sourceIntegrity *appv1.SourceIntegrity,
	clusterInfo ClusterRuntimeInfo,
	refSourceCommitSHAs ResolvedRevisions,
) manifestKey {
	return manifestKey{
		Revision:            revision,
		AppSource:           appSource,
		RefSources:          refSources,
		ClusterInfo:         clusterInfo,
		Namespace:           namespace,
		TrackingMethod:      trackingMethod,
		AppLabelKey:         appLabelKey,
		AppName:             appName,
		RefSourceCommitSHAs: refSourceCommitSHAs,
		InstallationID:      installationID,
		SourceIntegrity:     sourceIntegrity,
	}
}

func (d manifestKey) String() string {
	trackingKey := trackingKey(d.AppLabelKey, d.TrackingMethod)
	key := fmt.Sprintf("mfst|%s|%s|%s|%s|%d|%s", trackingKey, d.AppName, d.Revision, d.Namespace, appSourceKey(d.AppSource, d.RefSources, d.RefSourceCommitSHAs)+clusterRuntimeInfoKey(d.ClusterInfo), d.SourceIntegrity.CacheKey())
	if d.InstallationID != "" {
		key = fmt.Sprintf("%s|%s", key, d.InstallationID)
	}
	return key
}

func trackingKey(appLabelKey string, trackingMethod string) string {
	trackingKey := appLabelKey
	if text.FirstNonEmpty(trackingMethod, string(appv1.TrackingMethodLabel)) != string(appv1.TrackingMethodLabel) {
		trackingKey = trackingMethod + ":" + trackingKey
	}
	return trackingKey
}

func (c *Cache) SetNewRevisionManifests(oldKey, newKey manifestKey) error {
	return c.cache.RenameItem(oldKey.String(), newKey.String(), c.repoCacheExpiration)
}

func (c *Cache) GetManifests(manifestKey manifestKey, res *CachedManifestResponse) error {
	logCtx := log.WithField("cacheKey", manifestKey.String())
	err := c.cache.GetItem(manifestKey.String(), res)
	if err != nil {
		return err
	}

	hash, err := res.generateCacheEntryHash()
	if err != nil {
		return fmt.Errorf("unable to generate hash value: %w", err)
	}

	// If cached result does not have manifests or the expected hash of the cache entry does not match the actual hash value...
	if hash != res.CacheEntryHash || res.ManifestResponse == nil && res.MostRecentError == "" {
		logCtx.Warnf("Manifest hash did not match expected value or cached manifests response is empty, treating as a cache miss: %s", manifestKey.AppName)
		logCtx.Debug("deleting manifests cache: manifest hash did not match or cached response is empty")

		err = c.DeleteManifests(manifestKey)
		if err != nil {
			return fmt.Errorf("unable to delete manifest after hash mismatch: %w", err)
		}

		// Treat hash mismatches as cache misses, so that the underlying resource is reacquired
		return ErrCacheMiss
	}

	// The expected hash matches the actual hash, so remove the hash from the returned value
	res.CacheEntryHash = ""

	if res.ManifestResponse != nil {
		// cached manifest response might be reused across different revisions, so we need to assume that the revision is the one we are looking for
		res.ManifestResponse.Revision = manifestKey.Revision
	}

	return nil
}

func (c *Cache) SetManifests(manifestKey manifestKey, res *CachedManifestResponse) error {
	// Generate and apply the cache entry hash, before writing
	if res != nil {
		res = res.shallowCopy()
		hash, err := res.generateCacheEntryHash()
		if err != nil {
			return fmt.Errorf("unable to generate hash value: %w", err)
		}
		res.CacheEntryHash = hash
	}

	return c.cache.SetItem(
		manifestKey.String(),
		res,
		&cacheutil.CacheActionOpts{
			Expiration: c.repoCacheExpiration,
			Delete:     res == nil,
		})
}

func (c *Cache) DeleteManifests(manifestKey manifestKey) error {
	return c.cache.SetItem(
		manifestKey.String(),
		"",
		&cacheutil.CacheActionOpts{Delete: true})
}

func appDetailsCacheKey(revision string, appSrc *appv1.ApplicationSource, srcRefs appv1.RefTargetRevisionMapping, trackingMethod appv1.TrackingMethod, refSourceCommitSHAs ResolvedRevisions) string {
	if trackingMethod == "" {
		trackingMethod = appv1.TrackingMethodLabel
	}
	return fmt.Sprintf("appdetails|%s|%d|%s", revision, appSourceKey(appSrc, srcRefs, refSourceCommitSHAs), trackingMethod)
}

func (c *Cache) GetAppDetails(revision string, appSrc *appv1.ApplicationSource, srcRefs appv1.RefTargetRevisionMapping, res *apiclient.RepoAppDetailsResponse, trackingMethod appv1.TrackingMethod, refSourceCommitSHAs ResolvedRevisions) error {
	return c.cache.GetItem(appDetailsCacheKey(revision, appSrc, srcRefs, trackingMethod, refSourceCommitSHAs), res)
}

func (c *Cache) SetAppDetails(revision string, appSrc *appv1.ApplicationSource, srcRefs appv1.RefTargetRevisionMapping, res *apiclient.RepoAppDetailsResponse, trackingMethod appv1.TrackingMethod, refSourceCommitSHAs ResolvedRevisions) error {
	return c.cache.SetItem(
		appDetailsCacheKey(revision, appSrc, srcRefs, trackingMethod, refSourceCommitSHAs),
		res,
		&cacheutil.CacheActionOpts{
			Expiration: c.repoCacheExpiration,
			Delete:     res == nil,
		})
}

func revisionMetadataKey(repoURL, revision string) string {
	return fmt.Sprintf("revisionmetadata|%s|%s", repoURL, revision)
}

func (c *Cache) GetRevisionMetadata(repoURL, revision string) (*appv1.RevisionMetadata, error) {
	item := &appv1.RevisionMetadata{}
	return item, c.cache.GetItem(revisionMetadataKey(repoURL, revision), item)
}

func (c *Cache) SetRevisionMetadata(repoURL, revision string, item *appv1.RevisionMetadata) error {
	return c.cache.SetItem(
		revisionMetadataKey(repoURL, revision),
		item,
		&cacheutil.CacheActionOpts{Expiration: c.repoCacheExpiration})
}

func revisionChartDetailsKey(repoURL, chart, revision string) string {
	return fmt.Sprintf("chartdetails|%s|%s|%s", repoURL, chart, revision)
}

func (c *Cache) GetRevisionChartDetails(repoURL, chart, revision string) (*appv1.ChartDetails, error) {
	item := &appv1.ChartDetails{}
	return item, c.cache.GetItem(revisionChartDetailsKey(repoURL, chart, revision), item)
}

func (c *Cache) SetRevisionChartDetails(repoURL, chart, revision string, item *appv1.ChartDetails) error {
	return c.cache.SetItem(
		revisionChartDetailsKey(repoURL, chart, revision),
		item,
		&cacheutil.CacheActionOpts{Expiration: c.repoCacheExpiration})
}

func ociMetadataKey(repoURL, revision string) string {
	return fmt.Sprintf("ocimetadata|%s|%s", repoURL, revision)
}

func (c *Cache) GetOCIMetadata(repoURL, revision string) (*appv1.OCIMetadata, error) {
	item := &appv1.OCIMetadata{}
	return item, c.cache.GetItem(ociMetadataKey(repoURL, revision), item)
}

func (c *Cache) SetOCIMetadata(repoURL, revision string, item *appv1.OCIMetadata) error {
	return c.cache.SetItem(
		ociMetadataKey(repoURL, revision),
		item,
		&cacheutil.CacheActionOpts{Expiration: c.repoCacheExpiration})
}

func gitFilesKey(repoURL, revision, pattern string) string {
	return fmt.Sprintf("gitfiles|%s|%s|%s", repoURL, revision, pattern)
}

func (c *Cache) SetGitFiles(repoURL, revision, pattern string, files map[string][]byte) error {
	return c.cache.SetItem(
		gitFilesKey(repoURL, revision, pattern),
		&files,
		&cacheutil.CacheActionOpts{Expiration: c.repoCacheExpiration})
}

func (c *Cache) GetGitFiles(repoURL, revision, pattern string) (map[string][]byte, error) {
	var item map[string][]byte
	err := c.cache.GetItem(gitFilesKey(repoURL, revision, pattern), &item)
	return item, err
}

func gitDirectoriesKey(repoURL, revision string) string {
	return fmt.Sprintf("gitdirs|%s|%s", repoURL, revision)
}

func (c *Cache) SetGitDirectories(repoURL, revision string, directories []string) error {
	return c.cache.SetItem(
		gitDirectoriesKey(repoURL, revision),
		&directories,
		&cacheutil.CacheActionOpts{Expiration: c.repoCacheExpiration})
}

func (c *Cache) GetGitDirectories(repoURL, revision string) ([]string, error) {
	var item []string
	err := c.cache.GetItem(gitDirectoriesKey(repoURL, revision), &item)
	return item, err
}

func getGitFilesChangesKey(repoURL, revision, targetRevision string) string {
	return fmt.Sprintf("gitFilesChanges|%s|%s|%s", repoURL, revision, targetRevision)
}

func (c *Cache) SetGitFilesChanges(repoURL, revision, targetRevision string, files []string) error {
	return c.cache.SetItem(
		getGitFilesChangesKey(repoURL, revision, targetRevision),
		&files,
		&cacheutil.CacheActionOpts{Expiration: c.repoCacheExpiration})
}

func (c *Cache) GetGitFilesChanges(repoURL, revision, targetRevision string) ([]string, error) {
	var files []string
	err := c.cache.GetItem(getGitFilesChangesKey(repoURL, revision, targetRevision), &files)
	return files, err
}

func ociFilesKey(repoURL, revision, pattern string) string {
	return fmt.Sprintf("ocifiles|%s|%s|%s", repoURL, revision, pattern)
}

func (c *Cache) SetOciFiles(repoURL, revision, pattern string, files map[string][]byte) error {
	return c.cache.SetItem(
		ociFilesKey(repoURL, revision, pattern),
		&files,
		&cacheutil.CacheActionOpts{Expiration: c.repoCacheExpiration})
}

func (c *Cache) GetOciFiles(repoURL, revision, pattern string) (map[string][]byte, error) {
	var item map[string][]byte
	err := c.cache.GetItem(ociFilesKey(repoURL, revision, pattern), &item)
	return item, err
}

func ociDirectoriesKey(repoURL, revision string) string {
	return fmt.Sprintf("ocidirs|%s|%s", repoURL, revision)
}

func (c *Cache) SetOciDirectories(repoURL, revision string, directories []string) error {
	return c.cache.SetItem(
		ociDirectoriesKey(repoURL, revision),
		&directories,
		&cacheutil.CacheActionOpts{Expiration: c.repoCacheExpiration})
}

func (c *Cache) GetOciDirectories(repoURL, revision string) ([]string, error) {
	var item []string
	err := c.cache.GetItem(ociDirectoriesKey(repoURL, revision), &item)
	return item, err
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
	shallowCopy := cmr.shallowCopy()
	shallowCopy.CacheEntryHash = ""

	// Hash the JSON representation into a base-64-encoded FNV 64a (we don't need a cryptographic hash algorithm, since this is only for detecting data corruption)
	bytes, err := json.Marshal(shallowCopy)
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
