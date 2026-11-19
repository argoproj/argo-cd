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
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	cryptoutil "github.com/argoproj/argo-cd/v3/util/crypto"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	cacheutil "github.com/argoproj/argo-cd/v3/util/cache"
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

// GetHelmIndexCacheExpiration returns HTTP Helm index freshness, independently of Git revisions.
func (c *Cache) GetHelmIndexCacheExpiration() time.Duration {
	return c.helmIndexCacheExpiration
}

// SetHelmIndexCacheExpiration configures HTTP Helm index freshness before the cache is used.
func (c *Cache) SetHelmIndexCacheExpiration(expiration time.Duration) {
	c.helmIndexCacheExpiration = expiration
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
	return &Cache{
		cache: cache, repoCacheExpiration: repoCacheExpiration, revisionCacheExpiration: revisionCacheExpiration,
		revisionCacheLockTimeout: revisionCacheLockTimeout, helmIndexCacheExpiration: helm.DefaultIndexCacheExpiration,
	}
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

func helmIndexRefsKey(repo string) string {
	return "helm-index|" + repo
}

func ociTagsKey(repo string) string {
	return "oci-tags|" + repo
}

// The v3 namespace isolates ownership-aware writers from older replicas.
// All publication keys share a Redis hash tag for atomic multi-key updates.
func helmIndexKey(repo, part string) string {
	prefix, _ := helmIndexKeyPrefix(context.Background(), repo)
	return prefix + part
}

func helmIndexKeyPrefix(ctx context.Context, repo string) (string, error) {
	scope, err := cryptoutil.CredentialScopeContext(ctx, "helm-index-record-v3", repo)
	if err != nil {
		return "", err
	}
	return "helm-index-v3|{" + scope + "}|", nil
}
func helmIndexRecordKey(repo string) string        { return helmIndexKey(repo, "record") }
func helmIndexBodyReferenceKey(repo string) string { return helmIndexKey(repo, "body-reference") }
func helmIndexBodyKey(repo string) string          { return helmIndexKey(repo, "body") }

type HelmIndexRecord = helm.HelmIndexRecord

func (c *Cache) SetHelmIndex(repo string, indexData []byte) error {
	if indexData == nil {
		return errors.New("helm index data is nil, skipping cache")
	}
	if err := c.SetHelmIndexRecord(repo, indexData, "", ""); err != nil {
		return err
	}
	return c.cache.SetItem(helmIndexRefsKey(repo), indexData, &cacheutil.CacheActionOpts{Expiration: c.helmIndexCacheExpiration})
}

// GetHelmIndex keeps the original context-free API for legacy integrations.
// Request paths use GetHelmIndexSnapshotContext instead.
func (c *Cache) GetHelmIndex(repo string, indexData *[]byte) error {
	if c.helmIndexCacheExpiration <= 0 {
		return ErrCacheMiss
	}
	record, err := c.GetHelmIndexRecord(repo)
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

// Fresh and retained lookups, including old keys, share the caller's budget.
func (c *Cache) GetHelmIndexSnapshotContext(ctx context.Context, repo string) (*HelmIndexRecord, bool, error) {
	record, err := c.readHelmIndex(ctx, repo, true)
	if record != nil {
		if err != nil {
			return record, false, err
		}
		fresh := record.Fresh(c.helmIndexCacheExpiration)
		return record, fresh, nil
	}
	if !errors.Is(err, ErrCacheMiss) {
		return nil, false, err
	}
	var old HelmIndexRecord
	if err := c.cache.GetItemContext(ctx, "helm-index-v2|"+repo, &old); err == nil && len(old.Body) > 0 {
		return &old, old.Fresh(c.helmIndexCacheExpiration), nil
	} else if err != nil && !errors.Is(err, ErrCacheMiss) {
		return nil, false, err
	}
	var body []byte
	if err := c.cache.GetItemContext(ctx, helmIndexRefsKey(repo), &body); err != nil {
		return nil, false, err
	}
	return &HelmIndexRecord{Body: body}, false, nil
}

func (c *Cache) SetHelmIndexRecord(repo string, body []byte, etag, modified string) error {
	if body == nil {
		return errors.New("helm index data is nil, skipping cache")
	}
	return c.SetHelmIndexRetainedRecord(repo, &HelmIndexRecord{Body: body, ETag: etag, LastModified: modified, StoredAt: time.Now().UTC().Format(time.RFC3339Nano)})
}

// Maintenance/seeding writes invalidate every outstanding fetch owner.
// HTTP response publication must use the owner acquired before that request.
func (c *Cache) SetHelmIndexRetainedRecord(repo string, record *HelmIndexRecord) error {
	token, err := uuid.NewRandom()
	if err != nil {
		return err
	}
	_, err = c.storeHelmIndex(context.Background(), repo, token.String(), record, false)
	return err
}

func (c *Cache) BeginHelmIndexFetch(ctx context.Context, repo string) (string, error) {
	prefix, err := helmIndexKeyPrefix(ctx, repo)
	if err != nil {
		return "", err
	}
	token, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	claimContext := ctx
	cancel := func() {}
	if deadline, ok := ctx.Deadline(); ok {
		claimContext, cancel = context.WithTimeout(ctx, time.Until(deadline)*3/4)
	}
	// Carry the already-scoped key in the opaque claim so cancellation cleanup
	// never has to repeat an evicted credential-scope derivation.
	owner := prefix + "owner|fetch|" + token.String()
	ok, err := c.cache.BeginUpdate(claimContext, prefix+"owner", owner, helm.IndexFetchLease)
	cancel()
	if err != nil {
		// Use the reserved portion of this phase rather than adding another
		// timeout after acquisition has already spent the caller's budget.
		cleanup, stop := context.WithTimeout(ctx, 250*time.Millisecond)
		if ctx.Err() != nil {
			stop()
			cleanup, stop = context.WithTimeout(context.WithoutCancel(ctx), 25*time.Millisecond)
		}
		_, _ = c.cache.ReleaseUpdate(cleanup, prefix+"owner", owner)
		stop()
		return "", err
	}
	if !ok {
		return "", nil // Another fetch holds the claim until publication or release.
	}
	return owner, nil
}

func (c *Cache) ReleaseHelmIndexFetch(ctx context.Context, _ string, owner string) error {
	key, err := helmIndexClaimKey(owner)
	if err != nil {
		return err
	}
	_, err = c.cache.ReleaseUpdate(ctx, key, owner)
	return err
}

func (c *Cache) RenewHelmIndexFetch(ctx context.Context, _ string, owner string) (bool, error) {
	key, err := helmIndexClaimKey(owner)
	if err != nil {
		return false, err
	}
	return c.cache.RenewUpdate(ctx, key, owner, helm.IndexFetchLease)
}

func helmIndexClaimKey(owner string) (string, error) {
	key, nonce, ok := strings.Cut(owner, "|fetch|")
	if !ok || !strings.HasPrefix(key, "helm-index-v3|{argon2id-v1:") || !strings.HasSuffix(key, "}|owner") {
		return "", errors.New("invalid helm index fetch claim")
	}
	if _, err := uuid.Parse(nonce); err != nil {
		return "", errors.New("invalid helm index fetch claim nonce")
	}
	return key, nil
}

func (c *Cache) PublishHelmIndexRecord(ctx context.Context, repo, owner string, record *HelmIndexRecord) (bool, error) {
	if owner == "" {
		return false, errors.New("missing helm index fetch owner")
	}
	return c.storeHelmIndex(ctx, repo, owner, record, true)
}

func (c *Cache) storeHelmIndex(ctx context.Context, repo, owner string, record *HelmIndexRecord, fenced bool) (bool, error) {
	if record == nil || len(record.Body) == 0 {
		return false, errors.New("cannot publish an empty helm index")
	}
	validatedAt, err := time.Parse(time.RFC3339Nano, record.StoredAt)
	if err != nil {
		return false, fmt.Errorf("invalid helm index validation time: %w", err)
	}
	remaining := time.Until(validatedAt.Add(c.helmIndexCacheExpiration + max(24*time.Hour, c.helmIndexCacheExpiration)))
	if remaining <= 0 {
		return false, nil
	}
	prefix, err := helmIndexKeyPrefix(ctx, repo)
	if err != nil {
		return false, err
	}
	digest := sha256.Sum256(record.Body)
	metadata := *record
	if !fenced && metadata.URLIdentity == "" {
		// Explicit maintenance seeding supplies a default request view. The
		// unbound URL cannot authorize conditional headers for an origin.
		metadata.URLIdentity = helm.IndexURLIdentity("")
	}
	metadata.Body = nil
	metadata.Generation = owner
	metadata.BodyDigest = hex.EncodeToString(digest[:])
	bodyKey := prefix + "body"
	confirmCurrent := func() bool {
		var current HelmIndexRecord
		confirmation, stop := context.WithTimeout(ctx, time.Second)
		defer stop()
		readErr := c.cache.GetItemContext(confirmation, prefix+"record", &current)
		return readErr == nil && current.Generation == metadata.Generation && current.BodyDigest == metadata.BodyDigest && current.StoredAt == metadata.StoredAt && current.ETag == metadata.ETag && current.LastModified == metadata.LastModified && current.URLIdentity == metadata.URLIdentity
	}
	// Reserve time to confirm an ambiguous reply and share the body. A script
	// can commit and consume the owner before its reply is lost or retried.
	writeContext := ctx
	cancel := func() {}
	if deadline, ok := ctx.Deadline(); ok {
		writeContext, cancel = context.WithTimeout(ctx, time.Until(deadline)*3/4)
	}
	updated, err := c.cache.StoreVersionedItem(writeContext, cacheutil.VersionedItem{Key: prefix + "record", ReferenceKey: prefix + "body-reference", ReferenceValue: metadata.BodyDigest, BodyKey: bodyKey, OwnerKey: prefix + "owner", Token: owner, Object: &metadata, Expiration: remaining, RequireOwner: fenced})
	cancel()
	if !fenced && errors.Is(err, cacheutil.ErrAtomicPublicationUnsupported) {
		// Inline storage preserves legacy in-memory integrations, which have no
		// authority to publish origin responses through the distributed protocol.
		metadata.Body = record.Body
		return false, c.cache.SetItem(prefix+"record", &metadata, &cacheutil.CacheActionOpts{Expiration: remaining})
	}
	if (err != nil || !updated) && ctx.Err() == nil {
		if confirmCurrent() {
			updated, err = true, nil
		}
	}
	if err != nil || !updated {
		return updated, err
	}
	bodyContext := ctx
	stopBody := func() {}
	if deadline, ok := ctx.Deadline(); ok {
		bodyContext, stopBody = context.WithTimeout(ctx, time.Until(deadline)*3/4)
	}
	bodyStored, bodyErr := c.cache.StoreVersionedBody(bodyContext, prefix+"body-reference", bodyKey, metadata.BodyDigest, record.Body, remaining)
	stopBody()
	if bodyErr == nil && !bodyStored {
		return false, helm.ErrHelmIndexBodyReferenceRejected
	}
	if bodyErr != nil {
		if !fenced {
			return false, bodyErr
		}
		if !confirmCurrent() {
			return false, helm.ErrHelmIndexPublicationUnconfirmed
		}
		log.WithError(bodyErr).Debug("helm index version confirmed; shared body unavailable")
	}
	return true, nil
}

func (c *Cache) GetHelmIndexGenerationContext(ctx context.Context, repo string) (string, error) {
	metadata, err := c.GetHelmIndexMetadataContext(ctx, repo)
	if err != nil {
		return "", err
	}
	return metadata.Generation, nil
}

func (c *Cache) GetHelmIndexMetadataContext(ctx context.Context, repo string) (*HelmIndexRecord, error) {
	prefix, err := helmIndexKeyPrefix(ctx, repo)
	if err != nil {
		return nil, err
	}
	var metadata HelmIndexRecord
	if err := c.cache.GetItemContext(ctx, prefix+"record", &metadata); err != nil {
		return nil, err
	}
	if metadata.Generation == "" {
		return nil, ErrCacheMiss
	}
	metadata.Body = nil
	return &metadata, nil
}

func (c *Cache) readHelmIndex(ctx context.Context, repo string, contextual bool) (*HelmIndexRecord, error) {
	prefix, err := helmIndexKeyPrefix(ctx, repo)
	if err != nil {
		return nil, err
	}
	get := func(key string, obj any) error {
		if contextual {
			return c.cache.GetItemContext(ctx, key, obj)
		}
		return c.cache.GetItem(key, obj)
	}
	var metadata HelmIndexRecord
	if err := get(prefix+"record", &metadata); err != nil {
		return nil, err
	}
	if len(metadata.Body) > 0 {
		return &metadata, nil
	} // Legacy in-memory adapter.
	var body []byte
	if err := get(prefix+"body", &body); err != nil {
		return &metadata, err
	}
	digest := sha256.Sum256(body)
	if hex.EncodeToString(digest[:]) != metadata.BodyDigest {
		return &metadata, errors.New("helm index body does not match its published digest")
	}
	metadata.Body = body
	return &metadata, nil
}

func (c *Cache) GetHelmIndexRetainedRecordContext(ctx context.Context, repo string) (*HelmIndexRecord, error) {
	return c.readHelmIndex(ctx, repo, true)
}

func (c *Cache) GetHelmIndexRetainedRecord(repo string) (*HelmIndexRecord, error) {
	return c.GetHelmIndexRecord(repo)
}

func (c *Cache) GetHelmIndexRecord(repo string) (*HelmIndexRecord, error) {
	return c.readHelmIndex(context.Background(), repo, false)
}

// helmIndexValidatorsKey returns the cache key for a repository's HTTP
// validator headers (ETag / Last-Modified) used for conditional GETs.
func helmIndexValidatorsKey(repo string) string {
	return "helm-index-validators|" + repo
}

// SetHelmIndexValidators preserves the legacy split-key contract. It cannot
// change the authoritative versioned record or invalidate a fetch owner.
func (c *Cache) SetHelmIndexValidators(repo, etag, modified string) error {
	return c.cache.SetItem(helmIndexValidatorsKey(repo), &HelmIndexValidators{ETag: etag, LastModified: modified}, &cacheutil.CacheActionOpts{Expiration: c.helmIndexCacheExpiration})
}

// GetHelmIndexValidators retrieves the HTTP validators for a repository's
// index.yaml from the legacy split key. Body-bound readers use the record API.
func (c *Cache) GetHelmIndexValidators(repo string) (string, string, error) {
	var validators HelmIndexValidators
	if err := c.cache.GetItem(helmIndexValidatorsKey(repo), &validators); err != nil {
		return "", "", err
	}
	return validators.ETag, validators.LastModified, nil
}

// HelmIndexValidators holds the HTTP conditional-GET validators for a cached
// helm repository index.
type HelmIndexValidators struct {
	ETag         string `json:"etag"`
	LastModified string `json:"lastModified"`
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
