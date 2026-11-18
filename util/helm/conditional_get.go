package helm

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	cryptoutil "github.com/argoproj/argo-cd/v3/util/crypto"
)

// indexCacheWithValidators is an optional extension of the indexCache
// interface for caches that can store HTTP validator headers (ETag,
// Last-Modified) alongside the cached index body. Implementations that do not
// support validators simply implement the base indexCache interface; the
// conditional-GET logic degrades gracefully to unconditional GETs.
type indexCacheWithValidators interface {
	SetHelmIndexValidators(repo string, etag string, lastModified string) error
	GetHelmIndexValidators(repo string) (etag string, lastModified string, err error)
}

// inMemoryIndexValidators is a body-bound fallback for caches without atomic
// record reads. Losing the validators on restart merely
// means the next fetch is an unconditional GET. Entries never expire
// client-side: the server decides via 304/200, and any 200 refreshes them.
type inMemoryIndexValidators struct {
	mu sync.RWMutex
	m  map[string]helmIndexValidators
}

type helmIndexValidators struct {
	etag         string
	lastModified string
	bodyDigest   [sha256.Size]byte
	bound        bool
	urlIdentity  string
}

// Bind fallback validators to the exact body. An external cache writer must
// never make a different body eligible for a 304 under these validators.
func (v *inMemoryIndexValidators) setForResponse(repo string, body []byte, etag, modified, urlIdentity string) {
	digest := sha256.Sum256(body)
	v.mu.Lock()
	defer v.mu.Unlock()
	if etag == "" && modified == "" {
		delete(v.m, repo)
		return
	}
	if v.m == nil {
		v.m = map[string]helmIndexValidators{}
	}
	v.m[repo] = helmIndexValidators{etag: etag, lastModified: modified, bodyDigest: digest, bound: true, urlIdentity: urlIdentity}
}

func (v *inMemoryIndexValidators) getForBody(repo string, body []byte) (string, string) {
	etag, modified, _ := v.getForResponse(repo, body)
	return etag, modified
}

func (v *inMemoryIndexValidators) getForResponse(repo string, body []byte) (string, string, string) {
	digest := sha256.Sum256(body)
	v.mu.RLock()
	defer v.mu.RUnlock()
	entry, ok := v.m[repo]
	if !ok || !entry.bound || entry.bodyDigest != digest {
		return "", "", ""
	}
	return entry.etag, entry.lastModified, entry.urlIdentity
}

func newInMemoryIndexValidators() *inMemoryIndexValidators {
	return &inMemoryIndexValidators{m: map[string]helmIndexValidators{}}
}

// SetHelmIndexValidators REPLACES any validators previously stored for the
// repo (callers use it after a 200, which is authoritative for both body and
// validators — including the case where the new response carries no
// validators at all, in which case the old ones must not survive).
func (v *inMemoryIndexValidators) SetHelmIndexValidators(repo string, etag string, lastModified string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.m == nil {
		v.m = map[string]helmIndexValidators{}
	}
	if etag == "" && lastModified == "" {
		delete(v.m, repo)
		return nil
	}
	v.m[repo] = helmIndexValidators{etag: etag, lastModified: lastModified}
	return nil
}

func (v *inMemoryIndexValidators) GetHelmIndexValidators(repo string) (string, string, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	vv, ok := v.m[repo]
	if !ok {
		return "", "", errors.New("no validators for repo")
	}
	return vv.etag, vv.lastModified, nil
}

// validatorsFor returns the validator store for an indexCache: the cache
// itself when it implements indexCacheWithValidators, otherwise the shared
// in-process fallback.
func validatorsFor(indexCacheObj indexCache) indexCacheWithValidators {
	if vc, ok := indexCacheObj.(indexCacheWithValidators); ok {
		return vc
	}
	return sharedInMemoryValidators
}

// sharedInMemoryValidators backs clients whose cache cannot store validators.
var sharedInMemoryValidators = newInMemoryIndexValidators()

// applyConditionalHeaders sets If-None-Match / If-Modified-Since on the
// request. Per RFC 9110 §13.1.2, If-None-Match takes precedence when both
// are sent, so If-Modified-Since is only set when there is no ETag.
func applyConditionalHeaders(req *http.Request, etag string, lastModified string) {
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	} else if lastModified != "" {
		req.Header.Set("If-Modified-Since", lastModified)
	}
}

// indexCacheRecorder is implemented by caches that can store the index body
// and its validators as one atomic versioned record.
type indexCacheRecorder interface {
	SetHelmIndexRecord(repo string, indexData []byte, etag string, lastModified string) error
}

// Ownership is acquired before the origin request. Losing the token, even to
// eviction, removes publication permission; a cache miss is never permission.
type indexCachePublisher interface {
	BeginHelmIndexFetch(context.Context, string) (string, error)
	PublishHelmIndexRecord(context.Context, string, string, *HelmIndexRecord) (bool, error)
}

func indexCacheContext(ctx context.Context, maximum time.Duration) (context.Context, context.CancelFunc) {
	budget := maximum
	if deadline, ok := ctx.Deadline(); ok {
		budget = min(budget, time.Until(deadline)/4)
	}
	return context.WithTimeout(ctx, budget)
}

func beginIndexFetch(ctx context.Context, cache any, repo string) string {
	publisher, ok := cache.(indexCachePublisher)
	if !ok {
		return ""
	}
	operation, cancel := indexCacheContext(ctx, time.Second)
	defer cancel()
	owner, err := publisher.BeginHelmIndexFetch(operation, repo)
	if err != nil {
		log.WithError(err).Debug("helm index fetch ownership unavailable")
		return ""
	}
	return owner
}

func publishIndexFetch(ctx context.Context, cache any, repo, owner string, record *HelmIndexRecord) bool {
	publisher, ok := cache.(indexCachePublisher)
	if !ok || owner == "" {
		return false
	}
	operation, cancel := indexCacheContext(ctx, 5*time.Second)
	defer cancel()
	record.Generation = owner
	published, err := publisher.PublishHelmIndexRecord(operation, repo, owner, record)
	if err != nil {
		log.WithError(err).Debug("helm index publication unconfirmed")
		return false
	}
	return published
}

type indexRecordObservation struct {
	record *HelmIndexRecord
}

// Whole-body reads may take longer than metadata. One bounded read budget
// still leaves most of the parent deadline for origin validation and rendering.
func observeIndexRecord(ctx context.Context, reader indexCacheRecordReader, repo string) indexRecordObservation {
	operation, cancel := indexCacheContext(ctx, 5*time.Second)
	defer cancel()
	return observeIndexRecordWithBudget(operation, reader, repo)
}

func observeIndexRecordWithBudget(ctx context.Context, reader indexCacheRecordReader, repo string) indexRecordObservation {
	var record *HelmIndexRecord
	var err error
	if bounded, ok := reader.(interface {
		GetHelmIndexRetainedRecordContext(context.Context, string) (*HelmIndexRecord, error)
	}); ok {
		record, err = bounded.GetHelmIndexRetainedRecordContext(ctx, repo)
	} else {
		record, err = reader.GetHelmIndexRetainedRecord(repo)
	}
	if err != nil {
		return indexRecordObservation{}
	}
	return indexRecordObservation{record: record}
}

// IndexURLIdentity binds validators to the effective response URL without
// putting URL credentials or query secrets in disk or Redis metadata.
func IndexURLIdentity(rawURL string) string {
	return cryptoutil.CredentialScope("helm-index-response-url", rawURL)
}

// responseValidatorHeaders extracts validators from the headers of the last
// 200 response seen for this fetch (empty strings when none).
func responseValidatorHeaders(h http.Header) (string, string) {
	if h == nil {
		return "", ""
	}
	return h.Get("ETag"), h.Get("Last-Modified")
}

// HelmIndexRecord is the versioned cache record for a helm repository index:
// the body and the validators the origin returned with it, published and read
// as one atomic unit so they can never describe different index generations.
type HelmIndexRecord struct {
	BodyDigest   string `json:"bodyDigest,omitempty"`
	Generation   string `json:"generation,omitempty"`
	Body         []byte `json:"body"`
	ETag         string `json:"etag,omitempty"`
	LastModified string `json:"lastModified,omitempty"`
	URLIdentity  string `json:"urlIdentity,omitempty"`
	// StoredAt is the publication time; freshness is measured from it while
	// the record is retained longer for 304 revalidation.
	StoredAt string `json:"storedAt,omitempty"`
}

// Fresh reports whether a usable validation timestamp falls within freshness.
// Missing or corrupt timestamps cannot establish a freshness deadline.
func (r *HelmIndexRecord) Fresh(freshness time.Duration) bool {
	if freshness <= 0 {
		return false
	}
	storedAt, err := time.Parse(time.RFC3339Nano, r.StoredAt)
	if err != nil {
		return false
	}
	age := time.Since(storedAt)
	return age >= 0 && age <= freshness
}

// indexCacheRecordReader is implemented by caches that expose the complete
// versioned record (body + validators) as one atomic unit, regardless of
// freshness — the revalidation path reads it to send conditional headers.
type indexCacheRecordReader interface {
	GetHelmIndexRetainedRecord(repo string) (*HelmIndexRecord, error)
}

// RFC 9111 section 4.3.4: a new strong ETag cannot identify bytes cached
// under a different or weak validator. Weak tags must match their opaque
// value; a response omitting its ETag can still validate the sent condition.
func matchingNotModifiedETag(cached, received string) bool {
	if received == "" {
		return true
	}
	if after, ok := strings.CutPrefix(received, "W/"); ok {
		return cached != "" && strings.TrimPrefix(cached, "W/") == after
	}
	return cached == received && !strings.HasPrefix(cached, "W/")
}
