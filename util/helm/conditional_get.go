package helm

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"sync"
	"time"
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
}

// Bind fallback validators to the exact body. An external cache writer must
// never make a different body eligible for a 304 under these validators.
func (v *inMemoryIndexValidators) setForBody(repo string, body []byte, etag, modified string) {
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
	v.m[repo] = helmIndexValidators{etag: etag, lastModified: modified, bodyDigest: digest, bound: true}
}

func (v *inMemoryIndexValidators) getForBody(repo string, body []byte) (string, string) {
	digest := sha256.Sum256(body)
	v.mu.RLock()
	defer v.mu.RUnlock()
	entry, ok := v.m[repo]
	if !ok || !entry.bound || entry.bodyDigest != digest {
		return "", ""
	}
	return entry.etag, entry.lastModified
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

// A 304 can renew shared freshness only if the observed generation still exists.
type indexCacheRecordRevalidator interface {
	RevalidateHelmIndexRecord(context.Context, string, *HelmIndexRecord, *HelmIndexRecord) (bool, error)
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
	Body         []byte `json:"body"`
	ETag         string `json:"etag,omitempty"`
	LastModified string `json:"lastModified,omitempty"`
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
