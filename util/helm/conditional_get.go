package helm

import (
	"errors"
	"net/http"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
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

// inMemoryIndexValidators is a per-process fallback for caches that do not
// implement indexCacheWithValidators. Losing the validators on restart merely
// means the next fetch is an unconditional GET. Entries never expire
// client-side: the server decides via 304/200, and any 200 refreshes them.
type inMemoryIndexValidators struct {
	mu sync.RWMutex
	m  map[string]helmIndexValidators
}

type helmIndexValidators struct {
	etag         string
	lastModified string
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

// getIndexValidators retrieves stored validators for a repo; empty strings
// when none are available (caller sends an unconditional GET).
func getIndexValidators(validators indexCacheWithValidators, repo string) (string, string) {
	etag, lastModified, err := validators.GetHelmIndexValidators(repo)
	if err != nil {
		return "", ""
	}
	return etag, lastModified
}

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

// storeResponseValidators persists the validators from a successful
// (200) response. Best-effort: failures are logged and never fail the fetch.
func storeResponseValidators(validators indexCacheWithValidators, repo string, resp *http.Response) {
	etag := resp.Header.Get("ETag")
	lastModified := resp.Header.Get("Last-Modified")
	// A 200 is authoritative: when it carries no validators, stale validators
	// from an earlier response must be dropped, otherwise a later conditional
	// GET could 304 against a validator that no longer describes the origin
	// state.
	if err := validators.SetHelmIndexValidators(repo, etag, lastModified); err != nil {
		log.WithError(err).Debugf("failed to store helm index validators for repo %s", repo)
	}
}

// indexCacheRecorder is implemented by caches that can store the index body
// and its validators as one atomic versioned record.
type indexCacheRecorder interface {
	SetHelmIndexRecord(repo string, indexData []byte, etag string, lastModified string) error
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
