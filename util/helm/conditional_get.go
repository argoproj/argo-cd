package helm

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang/groupcache/lru"

	cryptoutil "github.com/argoproj/argo-cd/v3/util/crypto"
)

// DefaultIndexCacheExpiration is the freshness window for cached HTTP Helm
// indexes. Records older than this are revalidated with the origin through a
// conditional (If-None-Match / If-Modified-Since) request before reuse.
const DefaultIndexCacheExpiration = time.Minute

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
// means the next fetch is an unconditional GET. Bound retained credential
// scopes; eviction simply makes the next origin request unconditional.
type inMemoryIndexValidators struct {
	mu sync.Mutex
	m  *lru.Cache
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
	// Keep request provenance even when the origin has no validators, so a
	// fresh raw-cache hit can still establish the selected representation.
	v.m.Add(repo, helmIndexValidators{etag: etag, lastModified: modified, bodyDigest: digest, bound: true, urlIdentity: urlIdentity})
}

func (v *inMemoryIndexValidators) getForResponse(repo string, body []byte) (string, string, string) {
	digest := sha256.Sum256(body)
	v.mu.Lock()
	defer v.mu.Unlock()
	value, ok := v.m.Get(repo)
	if !ok {
		return "", "", ""
	}
	entry := value.(helmIndexValidators)
	if !entry.bound || entry.bodyDigest != digest {
		return "", "", ""
	}
	return entry.etag, entry.lastModified, entry.urlIdentity
}

func newInMemoryIndexValidators() *inMemoryIndexValidators {
	return &inMemoryIndexValidators{m: lru.New(256)}
}

// SetHelmIndexValidators REPLACES any validators previously stored for the
// repo (callers use it after a 200, which is authoritative for both body and
// validators — including the case where the new response carries no
// validators at all, in which case the old ones must not survive).
func (v *inMemoryIndexValidators) SetHelmIndexValidators(repo string, etag string, lastModified string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if etag == "" && lastModified == "" {
		v.m.Remove(repo)
		return nil
	}
	v.m.Add(repo, helmIndexValidators{etag: etag, lastModified: lastModified})
	return nil
}

func (v *inMemoryIndexValidators) GetHelmIndexValidators(repo string) (string, string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	value, ok := v.m.Get(repo)
	if !ok {
		return "", "", errors.New("no validators for repo")
	}
	vv := value.(helmIndexValidators)
	if vv.etag == "" && vv.lastModified == "" {
		return "", "", errors.New("no validators for repo")
	}
	return vv.etag, vv.lastModified, nil
}

// sharedInMemoryValidators backs clients whose cache cannot store validators.
var sharedInMemoryValidators = newInMemoryIndexValidators()

// applyConditionalHeaders sets If-None-Match / If-Modified-Since on the
// request. Per RFC 9110 §13.1.2, If-None-Match takes precedence when both
// are sent, so If-Modified-Since is only set when there is no ETag.
func applyConditionalHeaders(req *http.Request, etag string, lastModified string) {
	if validEntityTag(etag) {
		req.Header.Set("If-None-Match", etag)
	} else if _, err := http.ParseTime(lastModified); err == nil {
		req.Header.Set("If-Modified-Since", lastModified)
	}
}

// A response must contain one entity-tag, never the If-None-Match wildcard
// or a list. Otherwise a 304 could validate existence rather than these bytes.
func validEntityTag(value string) bool {
	value = strings.TrimPrefix(value, "W/")
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return false
	}
	for i := 1; i < len(value)-1; i++ {
		if b := value[i]; b < 0x21 || b == '"' || b == 0x7f {
			return false
		}
	}
	return true
}

// indexCacheRecorder is implemented by caches that can store the index body
// and its validators as one atomic versioned record.
type indexCacheRecorder interface {
	SetHelmIndexRecord(repo string, indexData []byte, etag string, lastModified string) error
}

// indexCacheRetainedWriter is implemented by caches that accept a complete
// record — body, validators, and the URL identity bound at fetch time.
// Publication through it keeps the request provenance a recorder-only write
// cannot express, so later revalidations can re-send conditional headers.
type indexCacheRetainedWriter interface {
	SetHelmIndexRetainedRecord(repo string, record *HelmIndexRecord) error
}

type indexRecordObservation struct {
	record *HelmIndexRecord
}

// Whole-body reads may take longer than metadata; callers bound them with a
// request deadline so one slow cache read cannot consume the whole budget.
// The PR1 record reader has no context-aware variant; the bounded read is
// enforced by the caller's context.
// indexCacheReadBudget bounds one retained-record read so a slow backend
// releases the per-key index lock instead of stalling the request past its
// usable budget. The budget reserves time for the origin request: a request
// with a deadline spends at most a quarter of its remaining time on the
// cache read; requests without a deadline keep the fixed 2-second cap.
const indexCacheReadBudget = 2 * time.Second

// observeIndexReadContext is the shared budget helper for every optional
// cache read (retained-record and raw-body alike): min(2s, remaining/4).
// It never extends the parent deadline; the child expires sooner at most.
func observeIndexReadContext(ctx context.Context) (context.Context, context.CancelFunc) {
	budget := indexCacheReadBudget
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline) / 4
		if remaining < budget {
			budget = remaining
		}
	}
	return context.WithTimeout(ctx, budget)
}

// observeFreshIndexRecord observes the record through a cache that enforces
// freshness centrally: the returned record is fresh at read time, so the
// caller may use its body, validators and identity as one consistent
// generation.
func observeFreshIndexRecord(ctx context.Context, reader indexCacheFreshRecordReader, repo string) indexRecordObservation {
	readCtx, cancel := observeIndexReadContext(ctx)
	defer cancel()

	record, err := reader.GetHelmIndexFreshRecordContext(readCtx, repo)
	if err != nil {
		return indexRecordObservation{}
	}
	return indexRecordObservation{record: record}
}

func observeIndexRecord(ctx context.Context, reader indexCacheRecordReader, repo string) indexRecordObservation {
	readCtx, cancel := observeIndexReadContext(ctx)
	defer cancel()

	if contextual, ok := reader.(indexCacheContextedReader); ok {
		record, err := contextual.GetHelmIndexRetainedRecordContext(readCtx, repo)
		if err != nil {
			return indexRecordObservation{}
		}
		return indexRecordObservation{record: record}
	}

	// Caches without a context-aware read still get the budget: abandon the
	// read when the budget expires (the underlying read finishes on its own;
	// its result is simply dropped).
	type outcome struct {
		record *HelmIndexRecord
	}
	done := make(chan outcome, 1)
	go func() {
		record, err := reader.GetHelmIndexRetainedRecord(repo)
		if err != nil {
			record = nil
		}
		done <- outcome{record: record}
	}()
	select {
	case res := <-done:
		return indexRecordObservation(res)
	case <-readCtx.Done():
		return indexRecordObservation{}
	}
}

// IndexURLIdentity binds the default request profile and effective URL without
// putting URL credentials or query secrets in disk or Redis metadata.
func IndexURLIdentity(rawURL string) string {
	identity, _ := indexURLIdentityContext(context.Background(), rawURL)
	return identity
}

func indexURLIdentityContext(ctx context.Context, rawURL string) (string, error) {
	return (&nativeHelmChart{}).indexURLIdentityContext(ctx, rawURL)
}

func (c *nativeHelmChart) indexRequestPrefix(ctx context.Context) (string, error) {
	profile, err := cryptoutil.CredentialScopeContext(ctx, "helm-index-request-agent-v1", c.getUserAgent())
	return "helm-index-request-v1|" + profile + "|", err
}

func (c *nativeHelmChart) indexURLIdentityContext(ctx context.Context, rawURL string) (string, error) {
	prefix, err := c.indexRequestPrefix(ctx)
	if err != nil {
		return "", err
	}
	identity, err := cryptoutil.CredentialScopeContext(ctx, "helm-index-response-url", rawURL)
	return prefix + identity, err
}

func (c *nativeHelmChart) matchesIndexRequest(ctx context.Context, identity string) bool {
	if !strings.HasPrefix(identity, "helm-index-request-v1|") {
		return false
	}
	prefix, err := c.indexRequestPrefix(ctx)
	return err == nil && strings.HasPrefix(identity, prefix)
}

// All mutable request headers must be represented in the cache view. Agent
// values are bound above; credentials and transport policy have scoped keys.
// The remaining supported fields are fixed by this GET implementation.
func (c *nativeHelmChart) supportsIndexVary(header http.Header) bool {
	for _, value := range header.Values("Vary") {
		for field := range strings.SplitSeq(value, ",") {
			switch strings.ToLower(strings.TrimSpace(field)) {
			case "", "user-agent", "accept", "accept-encoding", "cache-control", "origin", "access-control-request-method", "access-control-request-headers":
			case "authorization":
				if c.indexCacheScope == "" && !c.scopedByCreds {
					return false
				}
			default:
				return false
			}
		}
	}
	return true
}

// responseValidatorHeaders extracts validators from the headers of the last
// 200 response seen for this fetch (empty strings when none).
func responseValidatorHeaders(h http.Header) (string, string) {
	if h == nil {
		return "", ""
	}
	return singleResponseValidator(h, "ETag"), singleResponseValidator(h, "Last-Modified")
}

func singleResponseValidator(header http.Header, name string) string {
	if len(header.Values(name)) > 1 {
		// Keep malformed distinct from omitted, especially on a 304. This
		// sentinel fails the entity-tag and HTTP-date grammar checks.
		return "invalid-multiple-" + name
	}
	return header.Get(name)
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

// indexCacheContextedReader is implemented by caches whose retained-record
// read can honor the caller's context, so a slow backend releases the
// per-key index lock on cancellation.
type indexCacheContextedReader interface {
	GetHelmIndexRetainedRecordContext(ctx context.Context, repo string) (*HelmIndexRecord, error)
}

// indexCacheFreshRecordReader is implemented by caches that enforce the
// freshness window centrally. It returns the record in one shared read only
// while that same record is fresh, so freshness and request identity can
// never be attributed to different records.
type indexCacheFreshRecordReader interface {
	GetHelmIndexFreshRecordContext(ctx context.Context, repo string) (*HelmIndexRecord, error)
}

// RFC 9111 section 4.3.4: a new strong ETag cannot identify bytes cached
// under a different or weak validator. Weak tags must match their opaque
// value; a response omitting its ETag can still validate the sent condition.
func matchingNotModifiedETag(cached, received string) bool {
	if received == "" {
		return true
	}
	if !validEntityTag(received) {
		return false
	}
	if after, ok := strings.CutPrefix(received, "W/"); ok {
		return cached != "" && strings.TrimPrefix(cached, "W/") == after
	}
	return cached == received && !strings.HasPrefix(cached, "W/")
}
