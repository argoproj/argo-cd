package helm

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidEntityTag(t *testing.T) {
	t.Run("Strong", func(t *testing.T) {
		assert.True(t, validEntityTag(`"abc"`))
	})
	t.Run("Weak", func(t *testing.T) {
		assert.True(t, validEntityTag(`W/"abc"`))
	})
	t.Run("Empty", func(t *testing.T) {
		assert.False(t, validEntityTag(""))
	})
	t.Run("Unquoted", func(t *testing.T) {
		assert.False(t, validEntityTag("abc"))
	})
	t.Run("Wildcard", func(t *testing.T) {
		assert.False(t, validEntityTag("*"))
	})
	t.Run("InternalQuote", func(t *testing.T) {
		assert.False(t, validEntityTag(`"ab"c"`))
	})
	t.Run("ControlChars", func(t *testing.T) {
		assert.False(t, validEntityTag("\"a\x01b\""))
	})
	t.Run("Space", func(t *testing.T) {
		assert.False(t, validEntityTag(`"a b"`))
	})
	t.Run("MissingTrailingQuote", func(t *testing.T) {
		assert.False(t, validEntityTag(`"abc`))
	})
}

func TestMatchingNotModifiedETag(t *testing.T) {
	t.Run("OmittedETagInResponse", func(t *testing.T) {
		assert.True(t, matchingNotModifiedETag(`"abc"`, ""))
	})
	t.Run("StrongMatch", func(t *testing.T) {
		assert.True(t, matchingNotModifiedETag(`"abc"`, `"abc"`))
	})
	t.Run("StrongMismatch", func(t *testing.T) {
		assert.False(t, matchingNotModifiedETag(`"abc"`, `"abd"`))
	})
	t.Run("WeakMatch", func(t *testing.T) {
		assert.True(t, matchingNotModifiedETag(`W/"abc"`, `W/"abc"`))
		assert.True(t, matchingNotModifiedETag(`"abc"`, `W/"abc"`))
	})
	t.Run("WeakCachedCannotBeValidatedByStrong", func(t *testing.T) {
		assert.False(t, matchingNotModifiedETag(`W/"abc"`, `"abc"`))
	})
	t.Run("MalformedResponseTag", func(t *testing.T) {
		assert.False(t, matchingNotModifiedETag(`"abc"`, "*"))
		assert.False(t, matchingNotModifiedETag(`"abc"`, "abc"))
	})
}

func TestApplyConditionalHeaders(t *testing.T) {
	t.Run("ETagTakesPrecedence", func(t *testing.T) {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/index.yaml", http.NoBody)
		require.NoError(t, err)
		applyConditionalHeaders(req, `"etag"`, "Mon, 02 Jan 2006 15:04:05 GMT")
		assert.Equal(t, `"etag"`, req.Header.Get("If-None-Match"))
		assert.Empty(t, req.Header.Get("If-Modified-Since"))
	})
	t.Run("FallbackToLastModified", func(t *testing.T) {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/index.yaml", http.NoBody)
		require.NoError(t, err)
		applyConditionalHeaders(req, "", "Mon, 02 Jan 2006 15:04:05 GMT")
		assert.Empty(t, req.Header.Get("If-None-Match"))
		assert.Equal(t, "Mon, 02 Jan 2006 15:04:05 GMT", req.Header.Get("If-Modified-Since"))
	})
	t.Run("InvalidValidatorsNotSent", func(t *testing.T) {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com/index.yaml", http.NoBody)
		require.NoError(t, err)
		applyConditionalHeaders(req, "*", "not-a-date")
		assert.Empty(t, req.Header.Get("If-None-Match"))
		assert.Empty(t, req.Header.Get("If-Modified-Since"))
	})
}

func TestSingleResponseValidator(t *testing.T) {
	header := http.Header{}
	header.Set("ETag", `"abc"`)
	assert.Equal(t, `"abc"`, singleResponseValidator(header, "ETag"))

	header.Add("ETag", `"a"`)
	header.Add("ETag", `"b"`)
	assert.Equal(t, "invalid-multiple-ETag", singleResponseValidator(header, "ETag"))

	// Map-style assignment under a canonically-cased key must also be
	// detected: Go only exposes it through the canonical key.
	header = http.Header{}
	header[http.CanonicalHeaderKey("ETag")] = []string{`"a"`, `"b"`}
	assert.Equal(t, "invalid-multiple-ETag", singleResponseValidator(header, "ETag"))

	header = http.Header{}
	header[http.CanonicalHeaderKey("ETag")] = []string{`"a"`}
	assert.Equal(t, `"a"`, singleResponseValidator(header, "ETag"))
}

func TestSupportsIndexVary(t *testing.T) {
	t.Run("EmptyHeader", func(t *testing.T) {
		client := &nativeHelmChart{}
		assert.True(t, client.supportsIndexVary(http.Header{}))
	})
	t.Run("AllowlistedFields", func(t *testing.T) {
		client := &nativeHelmChart{}
		header := http.Header{}
		header.Set("Vary", "Accept-Encoding, User-Agent")
		assert.True(t, client.supportsIndexVary(header))
	})
	t.Run("UnknownField", func(t *testing.T) {
		client := &nativeHelmChart{}
		header := http.Header{}
		header.Set("Vary", "X-Custom-Header")
		assert.False(t, client.supportsIndexVary(header))
	})
	t.Run("AuthorizationAnonymousIsUnsafe", func(t *testing.T) {
		client := &nativeHelmChart{}
		header := http.Header{}
		header.Set("Vary", "Authorization")
		assert.False(t, client.supportsIndexVary(header))
	})
	t.Run("AuthorizationScopedIsSafe", func(t *testing.T) {
		client := &nativeHelmChart{scopedByCreds: true}
		header := http.Header{}
		header.Set("Vary", "Authorization")
		assert.True(t, client.supportsIndexVary(header))
	})
}

func TestHelmIndexRecordFresh(t *testing.T) {
	t.Run("FreshRecord", func(t *testing.T) {
		record := &HelmIndexRecord{StoredAt: time.Now().UTC().Format(time.RFC3339Nano)}
		assert.True(t, record.Fresh(time.Minute))
	})
	t.Run("StaleRecord", func(t *testing.T) {
		record := &HelmIndexRecord{StoredAt: time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339Nano)}
		assert.False(t, record.Fresh(time.Minute))
	})
	t.Run("ZeroFreshnessNeverFresh", func(t *testing.T) {
		record := &HelmIndexRecord{StoredAt: time.Now().UTC().Format(time.RFC3339Nano)}
		assert.False(t, record.Fresh(0))
	})
	t.Run("MissingTimestamp", func(t *testing.T) {
		record := &HelmIndexRecord{}
		assert.False(t, record.Fresh(time.Minute))
	})
	t.Run("FutureTimestampIsNotFresh", func(t *testing.T) {
		record := &HelmIndexRecord{StoredAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano)}
		assert.False(t, record.Fresh(time.Minute))
	})
}

func TestIndexURLIdentityBindsUserAgentAndURL(t *testing.T) {
	client := &nativeHelmChart{}
	identityA, err := client.indexURLIdentityContext(t.Context(), "https://example.com/index.yaml")
	require.NoError(t, err)
	identityB, err := client.indexURLIdentityContext(t.Context(), "https://example.com/index.yaml")
	require.NoError(t, err)
	assert.Equal(t, identityA, identityB)

	other, err := client.indexURLIdentityContext(t.Context(), "https://example.org/index.yaml")
	require.NoError(t, err)
	assert.NotEqual(t, identityA, other)

	// Identity must not contain plaintext secrets from URL userinfo.
	withCreds, err := (&nativeHelmChart{}).indexURLIdentityContext(t.Context(), "https://user:secret@example.com/index.yaml")
	require.NoError(t, err)
	assert.NotContains(t, withCreds, "secret")
}

func TestMatchesIndexRequest(t *testing.T) {
	client := &nativeHelmChart{}
	identity, err := client.indexURLIdentityContext(t.Context(), "https://example.com/index.yaml")
	require.NoError(t, err)
	assert.True(t, client.matchesIndexRequest(t.Context(), identity))
	assert.False(t, client.matchesIndexRequest(t.Context(), "bogus"))
}

func TestResponseValidatorHeaders(t *testing.T) {
	header := http.Header{}
	header.Set("ETag", `"v1"`)
	header.Set("Last-Modified", "Mon, 02 Jan 2006 15:04:05 GMT")
	etag, modified := responseValidatorHeaders(header)
	assert.Equal(t, `"v1"`, etag)
	assert.Equal(t, "Mon, 02 Jan 2006 15:04:05 GMT", modified)

	etag, modified = responseValidatorHeaders(nil)
	assert.Empty(t, etag)
	assert.Empty(t, modified)
}

func TestInMemoryIndexValidatorsBodyBinding(t *testing.T) {
	store := newInMemoryIndexValidators()
	bodyA := []byte("index-a")
	bodyB := []byte("index-b")

	store.setForResponse("repo", bodyA, `"etag-a"`, "Mon, 02 Jan 2006 15:04:05 GMT", "identity-a")

	etag, modified, identity := store.getForResponse("repo", bodyA)
	assert.Equal(t, `"etag-a"`, etag)
	assert.Equal(t, "Mon, 02 Jan 2006 15:04:05 GMT", modified)
	assert.Equal(t, "identity-a", identity)

	// A different body must not reuse validators bound to bodyA.
	etag, _, _ = store.getForResponse("repo", bodyB)
	assert.Empty(t, etag)

	// A different repo must not reuse validators bound to "repo".
	etag, _, _ = store.getForResponse("other", bodyA)
	assert.Empty(t, etag)

	// A validator that was never bound (e.g. restored from an external
	// writer) must not be eligible for a 304 against different bytes.
	require.NoError(t, store.SetHelmIndexValidators("repo", `"etag-b"`, ""))
	etag, _, _ = store.getForResponse("repo", bodyA)
	assert.Empty(t, etag, "unbound validators must not match a body")

	// Setting both validators empty removes the entry entirely.
	store.setForResponse("repo", bodyA, `"etag-a"`, "", "identity-a")
	require.NoError(t, store.SetHelmIndexValidators("repo", "", ""))
	_, _, identity = store.getForResponse("repo", bodyA)
	assert.Empty(t, identity)
}

func TestDefaultIndexCacheExpirationValue(t *testing.T) {
	assert.Equal(t, time.Minute, DefaultIndexCacheExpiration)
}
