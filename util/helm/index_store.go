package helm

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	yamlv3 "go.yaml.in/yaml/v3"

	"sigs.k8s.io/yaml"

	cacheutil "github.com/argoproj/argo-cd/v3/util/cache"
)

// IndexRecordCache stores raw, scoped records without changing validation time
// when a record is copied between Redis and the local directory.
type IndexRecordCache interface {
	GetHelmIndexRetainedRecord(string) (*HelmIndexRecord, error)
	SetHelmIndexRetainedRecord(string, *HelmIndexRecord) error
}

// IndexStore persists validated original Helm repository indexes for dependency
// builds. Body, validators and validation time live in one atomic JSON record.
// OS locks coordinate readers, publication, cleanup and ordinary single-flight
// across goroutines and processes. Helm receives an independent private copy.
// Freshness uses helm-index-cache-expiration; expired bodies remain available for
// conditional revalidation for another max(24h, freshness). Reads and copies
// never restart either deadline.
type IndexStore struct {
	// FilterEnabled reduces only private build snapshots. Configure before use;
	// false retains complete original indexes and disables snapshot retries.
	FilterEnabled bool
	cache         IndexRecordCache
	// dir is the persistent cache directory (empty disables the store).
	dir string
	// expiration is how long a cached index file may be served as fresh.
	expiration time.Duration
	// lock coordinates readers, publishers and cleanup across processes.
	lock func(context.Context, string, bool, bool) (func(), error)
	// indexCacheMaxSize bounds how much of a downloaded index.yaml is read
	// during Go-side revalidation (mirrors --helm-registry-max-index-size).
	indexCacheMaxSize int64
}

// retentionFloor is the minimum time a stale-but-valid entry is retained
// beyond freshness so it can be revalidated with conditional GET.
const retentionFloor = 24 * time.Hour

// DefaultIndexCacheExpiration is the freshness window for HTTP Helm indexes.
const DefaultIndexCacheExpiration = time.Minute

// DefaultIndexCacheDir returns the pod-local cache directory, respecting the
// platform's temporary-directory environment variables, including TMPDIR.
func DefaultIndexCacheDir() string {
	return filepath.Join(os.TempDir(), "helm-index-cache")
}

func NewIndexStore(dir string, expiration time.Duration, indexCacheMaxSize int64, caches ...IndexRecordCache) *IndexStore {
	if dir != "" {
		if absolute, err := filepath.Abs(dir); err == nil {
			dir = absolute
		}
	}
	store := &IndexStore{dir: dir, expiration: expiration, lock: acquireIndexLock, indexCacheMaxSize: indexCacheMaxSize}
	if len(caches) > 0 {
		store.cache = caches[0]
	}
	// Init-time sweep: evict entries orphaned by previous process lifetimes
	// (credential rotation, renamed repos) that no read will ever touch.
	store.Sweep()
	return store
}

// retention returns how long entries past freshness remain usable for
// revalidation: max(24h, expiration).
func (s *IndexStore) retention() time.Duration {
	if s.expiration > retentionFloor {
		return s.expiration
	}
	return retentionFloor
}

// indexRecord is the on-disk versioned record: index body and the validators
// the origin returned with it, stored together in one atomic write.
type indexRecord struct {
	persistedStoredAt string // original generation timestamp, before filesystem age adjustment
	digestVerified    bool
	APIVersion        string `json:"apiVersion"`
	Generation        string `json:"generation,omitempty"`
	ETag              string `json:"etag,omitempty"`
	LastModified      string `json:"lastModified,omitempty"`
	URLIdentity       string `json:"urlIdentity,omitempty"`
	StoredAt          string `json:"storedAt"`
	Body              string `json:"body"` // base64-encoded index.yaml bytes
	BodyDigest        string `json:"bodyDigest,omitempty"`
	RecordDigest      string `json:"recordDigest,omitempty"`
}

// Bind validators and freshness to the verified body digest as well as to
// each other. A body-only checksum cannot detect altered validator metadata.
func (r *indexRecord) checksum() string {
	metadata, _ := json.Marshal([]string{r.APIVersion, r.BodyDigest, r.Generation, r.ETag, r.LastModified, r.URLIdentity, r.StoredAt})
	digest := sha256.Sum256(metadata)
	return hex.EncodeToString(digest[:])
}

// parseIndexBody rejects bodies that are not plausibly valid Helm repo
// indexes, per the plan's "validate complete bounded responses and
// apiVersion: v1 before publication".
func parseIndexBody(data []byte) error {
	// The schema decoder accepts only the first YAML document. Check the
	// complete stream too, so trailing corruption cannot acquire validators.
	// Successful validation is memoized by body digest, including warm reads.
	decoder := yamlv3.NewDecoder(bytes.NewReader(data))
	var document yamlv3.Node
	if err := decoder.Decode(&document); err != nil {
		return fmt.Errorf("helm index is not valid YAML: %w", err)
	}
	if err := decoder.Decode(&document); !errors.Is(err, io.EOF) {
		return errors.New("helm index must contain exactly one YAML document")
	}
	var env struct {
		APIVersion string             `json:"apiVersion"`
		Entries    map[string]Entries `json:"entries"`
	}
	if err := yaml.Unmarshal(data, &env); err != nil {
		return fmt.Errorf("helm index is not valid YAML: %w", err)
	}
	// Only complete Helm v1 indexes can be published to shared storage.
	if env.APIVersion != "v1" {
		return fmt.Errorf("helm index has unsupported apiVersion %q", env.APIVersion)
	}
	return nil
}

// cacheIndexFileName returns the file name helm expects for a repository's
// cached index inside its own cache home: "<name>-index.yaml" (helm's
// helmpath.CacheIndexFile).
func cacheIndexFileName(name string) string {
	if name != "" {
		name += "-"
	}
	return name + "index.yaml"
}

// cacheFileName returns the persistent cache file path for a repository
// identity. The file name is a deterministic hash of the identity, so it is
// stable across processes and does not require URL characters to be escaped.
func (s *IndexStore) cacheFileName(repoURL string) string {
	sum := sha256.Sum256([]byte(repoURL))
	return filepath.Join(s.dir, hex.EncodeToString(sum[:])+".json")
}

// readRecord loads and decodes the stored record, treating corruption,
// emptiness and age per the caller's policy. expiredAsMiss=false returns
// retained (past-freshness) entries for revalidation.
func (s *IndexStore) readRecord(repoURL string, expiredAsMiss bool) (*indexRecord, error) {
	if s == nil || s.dir == "" {
		return nil, errors.New("index cache is disabled")
	}

	file := s.cacheFileName(repoURL)

	info, err := os.Stat(file)
	if err != nil {
		return nil, fmt.Errorf("no cached helm index for repo %s: %w", repoURL, err)
	}

	// Allow base64 expansion plus bounded metadata. Check before allocation,
	// and limit the read as well in case the file grows after Stat.
	const metadataAllowance int64 = 1 << 20
	encodedLimit := min(max(int64(0), s.indexCacheMaxSize), (math.MaxInt64-metadataAllowance-1)/2)*2 + metadataAllowance
	if info.Size() > encodedLimit {
		return nil, errors.New("encoded helm index record exceeds maximum size")
	}
	f, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("failed to open cached helm index: %w", err)
	}
	defer func() { _ = f.Close() }()
	raw, err := io.ReadAll(io.LimitReader(f, encodedLimit+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read cached helm index for repo %s: %w", repoURL, err)
	}
	if int64(len(raw)) > encodedLimit {
		return nil, errors.New("encoded helm index record exceeds maximum size")
	}
	var rec indexRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		// A corrupt record is as good as none.
		return nil, fmt.Errorf("cached helm index for repo %s is corrupt: %w", repoURL, err)
	}
	validatedAt := info.ModTime()
	if rec.StoredAt != "" {
		storedAt, err := time.Parse(time.RFC3339Nano, rec.StoredAt)
		if err != nil {
			return nil, fmt.Errorf("invalid helm index cache timestamp: %w", err)
		}
		if storedAt.Before(validatedAt) {
			validatedAt = storedAt
		}
	}
	age := time.Since(validatedAt)
	if s.expiration <= 0 || age > s.expiration {
		if expiredAsMiss {
			return nil, fmt.Errorf("cached helm index for repo %s is expired", repoURL)
		}
		if age > s.expiration+s.retention() {
			// Reads do not remove files. Sweep takes the publication lock and
			// rechecks expiry before deleting a record.
			return nil, fmt.Errorf("retained helm index for repo %s is past retention", repoURL)
		}
	}
	body, err := base64.StdEncoding.DecodeString(rec.Body)
	if err != nil || len(body) == 0 {
		return nil, fmt.Errorf("cached helm index for repo %s has an empty or invalid body", repoURL)
	}
	if rec.APIVersion != "v1" {
		return nil, errors.New("unsupported helm index cache record version")
	}
	if int64(len(body)) > s.indexCacheMaxSize {
		return nil, errors.New("cached helm index exceeds maximum size")
	}
	digest := sha256.Sum256(body)
	actualDigest := hex.EncodeToString(digest[:])
	if rec.BodyDigest != "" && rec.BodyDigest != actualDigest {
		return nil, errors.New("helm index disk body does not match its stored digest")
	}
	if rec.RecordDigest != "" && rec.RecordDigest != rec.checksum() {
		return nil, errors.New("helm index disk metadata does not match its stored digest")
	}
	rec.digestVerified = rec.BodyDigest != "" && rec.RecordDigest != ""
	rec.BodyDigest = actualDigest
	if err := validatedIndexBodies.validateBodyDigest(body, digest); err != nil {
		return nil, err
	}
	rec.Body = string(body)
	// Preserve the effective deadline, including legacy records and an older
	// mtime, when a validator-only update republishes this record.
	rec.persistedStoredAt = rec.StoredAt
	rec.StoredAt = validatedAt.UTC().Format(time.RFC3339Nano)
	return &rec, nil
}

// writeRecord publishes a record in one atomic write (temp file + rename)
// under the per-identity lock.
func (s *IndexStore) writeRecord(repoURL string, rec *indexRecord) error {
	file := s.cacheFileName(repoURL)
	unlock, err := s.lock(context.Background(), file, false, true)
	if err != nil {
		return err
	}
	defer unlock()
	rec.StoredAt = time.Now().UTC().Format(time.RFC3339Nano)
	return s.publishRecord(repoURL, rec)
}

// publishRecord writes the record file assuming the caller already holds the
// record lock. Revalidate already holds it for the entire fetch and publish.
func (s *IndexStore) publishRecord(repoURL string, rec *indexRecord) error {
	if rec.BodyDigest == "" {
		body, err := base64.StdEncoding.DecodeString(rec.Body)
		if err != nil {
			return fmt.Errorf("decode helm index for publication: %w", err)
		}
		digest := sha256.Sum256(body)
		rec.BodyDigest = hex.EncodeToString(digest[:])
	}
	if rec.StoredAt == "" {
		rec.StoredAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	rec.RecordDigest = rec.checksum()
	raw, err := json.Marshal(rec)
	if err != nil {
		return err
	}

	// First-use initialization is part of the contract: the store directory
	// may not exist yet when the very first record is published.
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return fmt.Errorf("failed to create helm index cache dir: %w", err)
	}

	file := s.cacheFileName(repoURL)

	tmp, err := os.CreateTemp(s.dir, filepath.Base(file)+".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file for helm index cache: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		_ = os.Remove(tmpName) // no-op after successful rename
	}()
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to write helm index cache: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to write helm index cache: %w", err)
	}
	validatedAt, err := time.Parse(time.RFC3339Nano, rec.StoredAt)
	if err != nil {
		return fmt.Errorf("invalid helm index validation time: %w", err)
	}
	if err := os.Chtimes(tmpName, validatedAt, validatedAt); err != nil {
		return err
	}
	if err := os.Rename(tmpName, file); err != nil {
		return fmt.Errorf("failed to install helm index cache file: %w", err)
	}
	return nil
}

// Get returns the FRESH cached index bytes for the given repository identity,
// or an error if the store is disabled, the entry is missing, or it is past
// freshness. Reads never restart freshness: the deadline is fixed when the
// entry is written. Past-freshness entries remain available via GetRetained.
func (s *IndexStore) Get(repoURL string) ([]byte, error) {
	if s == nil || s.dir == "" {
		return nil, errors.New("index cache is disabled")
	}
	file := s.cacheFileName(repoURL)
	unlock, err := s.lock(context.Background(), file, true, true)
	if err != nil {
		return nil, err
	}
	defer unlock()
	rec, err := s.readRecord(repoURL, true)
	if err != nil {
		return nil, err
	}
	return []byte(rec.Body), nil
}

// GetRetained returns the cached index bytes even when past freshness (but
// within retention), for conditional-GET revalidation of a stale entry.
func (s *IndexStore) GetRetained(repoURL string) ([]byte, error) {
	if s == nil || s.dir == "" {
		return nil, errors.New("index cache is disabled")
	}
	file := s.cacheFileName(repoURL)
	unlock, err := s.lock(context.Background(), file, true, true)
	if err != nil {
		return nil, err
	}
	defer unlock()
	rec, err := s.readRecord(repoURL, false)
	if err != nil {
		return nil, err
	}
	return []byte(rec.Body), nil
}

// Set stores index bytes for the given repository identity, replacing any
// validators previously associated with the identity (a 200 response is the
// authority for both body and validators). Writes are atomic.
func (s *IndexStore) Set(repoURL string, data []byte) error {
	return s.SetWithValidators(repoURL, data, "", "")
}

// SetWithValidators stores index bytes together with the validators the origin
// returned for them, as one atomic record.
func (s *IndexStore) SetWithValidators(repoURL string, data []byte, etag string, lastModified string) error {
	if s == nil || s.dir == "" {
		return errors.New("index cache is disabled")
	}
	if int64(len(data)) > s.indexCacheMaxSize {
		return errors.New("helm index exceeds maximum cache size")
	}
	if len(data) == 0 {
		return fmt.Errorf("refusing to cache empty helm index for repo %s", repoURL)
	}
	if err := validateIndexBody(data); err != nil {
		return fmt.Errorf("refusing to cache invalid helm index for repo %s: %w", repoURL, err)
	}
	rec := &indexRecord{
		APIVersion:   "v1",
		ETag:         etag,
		LastModified: lastModified,
		Body:         base64.StdEncoding.EncodeToString(data),
	}
	return s.writeRecord(repoURL, rec)
}

// UpdateValidators refreshes ONLY the validators of the existing record —
// used after a 304, where the origin re-asserts (or omits) validators for the
// body we already hold. Never touches the body or restarts freshness on its
// own (the caller decides freshness via Set).
func (s *IndexStore) UpdateValidators(repoURL string, etag string, lastModified string) error {
	if s == nil || s.dir == "" {
		return errors.New("index cache is disabled")
	}
	file := s.cacheFileName(repoURL)
	unlock, err := s.lock(context.Background(), file, false, true)
	if err != nil {
		return err
	}
	defer unlock()
	return s.updateValidators(repoURL, etag, lastModified)
}

// updateValidators requires the record-path lock to be held by the caller.
func (s *IndexStore) updateValidators(repoURL string, etag string, lastModified string) error {
	rec, err := s.readRecord(repoURL, false)
	if err != nil {
		return err
	}
	rec.ETag = etag
	rec.LastModified = lastModified
	rec.Body = base64.StdEncoding.EncodeToString([]byte(rec.Body))
	return s.publishRecord(repoURL, rec)
}

// CopyTo copies the given index bytes into destDir as an index file named
// cacheIndexFileName(name) — the exact path and name helm expects in its own
// cache home. The copy is written atomically under the store lock, so
// concurrent invocations copying the same repo never race on dest.
func (s *IndexStore) CopyTo(repoURL string, data []byte, destDir string, name string) error {
	return s.copyTo(repoURL, data, destDir, name, false)
}

func (s *IndexStore) copyTo(repoURL string, data []byte, destDir string, name string, replace bool) error {
	if s == nil || s.dir == "" {
		return errors.New("index cache is disabled")
	}

	if filepath.Base(name) != name || strings.ContainsAny(name, "/\\") || filepath.VolumeName(name) != "" {
		return errors.New("invalid helm repository cache name")
	}

	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return fmt.Errorf("failed to create helm cache dir: %w", err)
	}

	dest := filepath.Join(destDir, cacheIndexFileName(name))
	if !replace {
		if _, err := os.Stat(dest); err == nil {
			log.WithFields(log.Fields{"repo": repoURL, "file": dest}).Debug("Helm index snapshot already installed")
			return nil
		}
	}

	tmp, err := os.CreateTemp(destDir, ".tmp-helm-index-copy-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file for helm cache index: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		_ = os.Remove(tmpName) // no-op after successful rename
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to write helm cache index: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to write helm cache index: %w", err)
	}
	// If another invocation already installed an index file for this name,
	// keep theirs — both came from the same persistent store.
	if !replace {
		if _, err := os.Stat(dest); err == nil {
			return nil
		}
	}
	if err := os.Rename(tmpName, dest); err != nil {
		return fmt.Errorf("failed to install helm cache index file: %w", err)
	}
	return nil
}

// retainedIndexView gives one fetch an immutable body/validator generation.
// Publication belongs to the provider after validation, never the HTTP reader.
type retainedIndexView struct{ record *HelmIndexRecord }

func (v retainedIndexView) GetHelmIndex(_ string, body *[]byte) error {
	if v.record == nil {
		return errors.New("no retained helm index")
	}
	*body = v.record.Body
	return nil
}
func (v retainedIndexView) SetHelmIndex(string, []byte) error { return nil }
func (v retainedIndexView) GetHelmIndexRetainedRecord(string) (*HelmIndexRecord, error) {
	if v.record == nil {
		return nil, errors.New("no retained helm index")
	}
	return v.record, nil
}

func (v retainedIndexView) GetHelmIndexValidators(string) (string, string, error) {
	if v.record == nil {
		return "", "", errors.New("no retained helm index")
	}
	return v.record.ETag, v.record.LastModified, nil
}
func (v retainedIndexView) SetHelmIndexValidators(string, string, string) error { return nil }

func (s *IndexStore) retainedRecord(identity string) (*HelmIndexRecord, bool) {
	return s.observeRetainedRecord(context.Background(), identity, nil)
}

func (s *IndexStore) observeRetainedRecord(ctx context.Context, identity string, rejectedDigest *string) (*HelmIndexRecord, bool) {
	var newest *HelmIndexRecord
	var persistedStoredAt string
	var needsLocalDigest bool
	fromShared := false
	if local, err := s.readRecord(identity, false); err == nil && (rejectedDigest == nil || local.BodyDigest != *rejectedDigest) {
		persistedStoredAt = local.persistedStoredAt
		newest = &HelmIndexRecord{Body: []byte(local.Body), BodyDigest: local.BodyDigest, ETag: local.ETag, LastModified: local.LastModified, StoredAt: local.StoredAt, URLIdentity: local.URLIdentity, Generation: local.Generation}
		needsLocalDigest = !local.digestVerified
		// Older records had no timestamp. Their existing mtime is authoritative.
		if info, err := os.Stat(s.cacheFileName(identity)); err == nil {
			validatedAt, err := time.Parse(time.RFC3339Nano, newest.StoredAt)
			if err != nil || info.ModTime().Before(validatedAt) {
				newest.StoredAt = info.ModTime().UTC().Format(time.RFC3339Nano)
			}
		}
		if needsLocalDigest {
			// Upgrade legacy files through a verified shared record or a full
			// origin response before trusting their validators or freshness.
			newest.StoredAt, newest.ETag, newest.LastModified, newest.URLIdentity = "", "", "", ""
		}
	}
	if s.cache != nil {
		readContext, cancel := indexCacheContext(ctx, 5*time.Second)
		defer cancel()
		var sharedRecord *HelmIndexRecord
		if metadata, ok := s.cache.(interface {
			GetHelmIndexMetadataContext(context.Context, string) (*HelmIndexRecord, error)
		}); ok && newest != nil {
			record, err := metadata.GetHelmIndexMetadataContext(readContext, identity)
			if err == nil && record != nil {
				if record.BodyDigest != "" && record.BodyDigest == newest.BodyDigest {
					if !needsLocalDigest && record.Generation != "" && record.Generation == newest.Generation && newest.ETag == record.ETag && newest.LastModified == record.LastModified && newest.URLIdentity == record.URLIdentity && persistedStoredAt == record.StoredAt {
						return newest, false
					}
					// Another replica may have revalidated the same bytes. Its
					// confirmed metadata can reuse our validated local body even
					// when the large shared-body transfer is unavailable.
					cloned := *record
					cloned.Body = newest.Body
					sharedRecord = &cloned
					newest.StoredAt = ""
				} else {
					// Valid YAML is insufficient: keep neither altered bytes nor
					// their validators eligible for a 304 after an integrity miss.
					if rejectedDigest != nil {
						*rejectedDigest = newest.BodyDigest
					}
					newest = nil
				}
			}
			if err != nil && !errors.Is(err, cacheutil.ErrCacheMiss) {
				return newest, false
			}
		}
		if sharedRecord == nil {
			sharedRecord = observeIndexRecordWithBudget(readContext, s.cache, identity).record
		}
		if record := sharedRecord; record != nil {
			validatedAt, err := time.Parse(time.RFC3339Nano, record.StoredAt)
			if err == nil && time.Since(validatedAt) <= s.expiration+s.retention() && int64(len(record.Body)) <= s.indexCacheMaxSize {
				// Shared publication order is authoritative across pods; wall
				// clocks can differ. Identical locally validated bytes need no
				// second parse/hash, but different shared bytes must be validated.
				matchingBody := newest != nil && bytes.Equal(newest.Body, record.Body)
				if matchingBody || validateIndexBody(record.Body) == nil {
					fromShared = needsLocalDigest || !matchingBody || newest.ETag != record.ETag || newest.LastModified != record.LastModified || newest.URLIdentity != record.URLIdentity || newest.Generation != record.Generation || persistedStoredAt != record.StoredAt
					newest = record
				}
			}
		}
	}
	return newest, fromShared
}

func (s *IndexStore) installRecord(identity string, record *HelmIndexRecord) bool {
	err := s.publishRecord(identity, &indexRecord{APIVersion: "v1", Body: base64.StdEncoding.EncodeToString(record.Body), BodyDigest: record.BodyDigest, ETag: record.ETag, LastModified: record.LastModified, StoredAt: record.StoredAt, URLIdentity: record.URLIdentity, Generation: record.Generation})
	if err != nil {
		log.WithError(err).Debug("failed to persist helm index record")
	}
	return err == nil
}

// Only confirmed ownership-aware publication establishes a new disk deadline.
func (s *IndexStore) publishFetchedRecord(ctx context.Context, identity, owner string, record *HelmIndexRecord) bool {
	if s.cache != nil && !publishIndexFetch(ctx, s.cache, identity, owner, record) {
		return false
	}
	return s.installRecord(identity, record)
}

// Revalidate refreshes a stored entry using conditional GET (Part 3
// machinery) so an expired-but-retained entry costs one 304 revalidation
// instead of a full re-download by the helm binary. Read policy implemented
// here (plan §4):
//   - fresh entry present: return it, no origin contact;
//   - expired entry retained: conditional GET with stored validators — a 304
//     returns the retained body and restarts freshness; a 200 replaces body
//     and validators; an origin error is returned (never stale-on-error);
//   - no entry or past retention: full GET via the same client.
//
// The fetch runs through a cache view of THIS store so the conditional GET
// uses the retained body and its stored validators. Concurrency: the whole
// check–fetch–publish sequence runs under the per-identity OS lock, so
// concurrent local processes share one fetch and cannot overwrite
// newer state with older responses.
func (s *IndexStore) Revalidate(ctx context.Context, repoURL string, chartClient *nativeHelmChart) ([]byte, error) {
	if s == nil || s.dir == "" {
		return nil, errors.New("index cache is disabled")
	}

	// Single-flight per identity within this process: concurrent misses
	// serialize; later ones observe the refreshed entry and skip the fetch.
	file := s.cacheFileName(repoURL)
	unlock, err := s.lock(ctx, file, false, true)
	if err != nil {
		return nil, err
	}
	published := false
	defer func() {
		unlock()
		if published {
			s.Sweep()
		}
	}()

	var rejectedDigest string
	retireRejectedRecord := func() error {
		if rejectedDigest == "" {
			return nil
		}
		// Retire known superseded bytes under the record lock. An origin
		// failure followed by a shared-cache outage must not resurrect them
		// on this request or on the next dependency build.
		if err := os.Remove(file); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("retire superseded helm index: %w", err)
		}
		return nil
	}
	candidate, fromShared := s.observeRetainedRecord(ctx, repoURL, &rejectedDigest)
	if err := retireRejectedRecord(); err != nil {
		return nil, err
	}
	if candidate != nil && candidate.Fresh(s.expiration) {
		// Preserve timestamps when installing a newer Redis generation locally.
		if fromShared {
			published = s.installRecord(repoURL, candidate)
		}
		return candidate.Body, nil
	}
	owner := beginIndexFetch(ctx, s.cache, repoURL)
	fetchPublished := false
	defer func() {
		if !fetchPublished {
			releaseIndexFetch(ctx, s.cache, repoURL, owner)
		}
	}()
	if owner != "" {
		// Observe again after claiming: an intervening maintenance write
		// could not invalidate a claim that did not exist yet.
		candidate, fromShared = s.observeRetainedRecord(ctx, repoURL, &rejectedDigest)
		if err := retireRejectedRecord(); err != nil {
			return nil, err
		}
		if candidate != nil && candidate.Fresh(s.expiration) {
			if fromShared {
				published = s.installRecord(repoURL, candidate)
			}
			return candidate.Body, nil
		}
	}
	operation := *chartClient
	operation.indexCache = retainedIndexView{candidate}
	operation.requireCompleteIndex = true
	operation.lastValidators = nil
	data, err := operation.loadRepoIndex(ctx, s.indexCacheMaxSize)
	if err != nil {
		return nil, err
	}
	published = s.publishFetchedRecord(ctx, repoURL, owner, operation.fetchedIndexRecord(data))
	fetchPublished = published
	return data, nil
}

// GetHelmIndexValidators returns the validators stored with the record for
// the given identity (empty strings when none).
func (s *IndexStore) GetHelmIndexValidators(repoURL string) (string, string, error) {
	if s == nil || s.dir == "" {
		return "", "", errors.New("index cache is disabled")
	}
	file := s.cacheFileName(repoURL)
	unlock, err := s.lock(context.Background(), file, true, true)
	if err != nil {
		return "", "", err
	}
	defer unlock()
	rec, err := s.readRecord(repoURL, false)
	if err != nil {
		return "", "", err
	}
	return rec.ETag, rec.LastModified, nil
}

// SetHelmIndexValidators replaces the validators of the stored record
// (replace semantics: empty values clear them).
func (s *IndexStore) SetHelmIndexValidators(repoURL string, etag string, lastModified string) error {
	return s.UpdateValidators(repoURL, etag, lastModified)
}

// ForceRefresh unconditionally fetches the index from the origin (full GET,
// no conditional headers) and publishes it, returning the fresh bytes. Used
// by the hard-refresh policy: the origin is contacted exactly once per build,
// and the persistent store is updated so subsequent ordinary builds run warm.
func (s *IndexStore) ForceRefresh(ctx context.Context, repoURL string, chartClient *nativeHelmChart) ([]byte, error) {
	if s == nil || s.dir == "" {
		return nil, errors.New("index cache is disabled")
	}
	file := s.cacheFileName(repoURL)
	unlock, err := s.lock(ctx, file, false, true)
	if err != nil {
		return nil, err
	}
	published := false
	defer func() {
		unlock()
		if published {
			s.Sweep()
		}
	}()

	owner := beginIndexFetch(ctx, s.cache, repoURL)
	defer func() {
		if !published {
			releaseIndexFetch(ctx, s.cache, repoURL, owner)
		}
	}()
	operation := *chartClient
	operation.indexCache = nil
	operation.requireCompleteIndex = true
	operation.lastValidators = nil
	chartClient = &operation
	data, err := chartClient.loadRepoIndex(ctx, s.indexCacheMaxSize)
	if err != nil {
		return nil, err
	}
	if err := validateIndexBody(data); err != nil {
		return nil, fmt.Errorf("refreshed helm index for repo %s failed validation: %w", repoURL, err)
	}
	published = s.publishFetchedRecord(ctx, repoURL, owner, chartClient.fetchedIndexRecord(data))
	return data, nil
}

// Sweep evicts disk entries that are past their retention window even when
// their identity is never read again. Without the sweep, credential rotation
// (each resolved credential view mapping to its own identity file) or
// renamed repositories accumulate dead files forever, since readRecord only
// prunes the identity being read. The scan is O(entries) with one stat per
// file; it runs (a) once at store initialization and (b) opportunistically
// after Revalidate/ForceRefresh publications — never on the hot Get path.
func (s *IndexStore) Sweep() {
	if s == nil || s.dir == "" {
		return
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return // directory missing: nothing to sweep
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if record, suffix, ok := strings.Cut(entry.Name(), ".json.tmp-"); ok {
			if suffix == "" || !isIndexRecordName(record+".json") {
				continue
			}
			unlock, err := s.lock(context.Background(), filepath.Join(s.dir, record+".json"), false, false)
			if err == nil {
				_ = os.Remove(filepath.Join(s.dir, entry.Name()))
				unlock()
			}
			continue
		}
		if !isIndexRecordName(entry.Name()) {
			continue
		}
		s.evictExpiredFile(filepath.Join(s.dir, entry.Name()))
	}
}

func isIndexRecordName(name string) bool {
	digest, ok := strings.CutSuffix(name, ".json")
	if !ok || len(digest) != sha256.Size*2 {
		return false
	}
	for _, char := range digest {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

func (s *IndexStore) evictExpiredFile(file string) {
	unlock, err := s.lock(context.Background(), file, false, false)
	if err != nil {
		return
	}
	defer unlock()
	// Read the current file after acquiring the publisher's lock: a record
	// discovered by ReadDir may have been replaced with a fresh generation.
	info, err := os.Stat(file)
	if err != nil || time.Since(info.ModTime()) <= s.expiration+s.retention() {
		return
	}
	_ = os.Remove(file)
}
