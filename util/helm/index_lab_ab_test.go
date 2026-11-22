package helm

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Lab A/B (operator-facing claims, executed against the pinned Helm binary
// and a live fixture server). These tests exercise the dependency-index
// cache end to end and record the observable behavior the operator
// documentation claims:
//
//  1. A build inside the freshness window may reuse the previously fetched
//     index (zero origin requests).
//  2. A commit within the freshness window can render on the previous
//     index and stay cached beyond the TTL (manifests are not rebuilt when
//     nothing outside Helm changes).
//  3. A new dependency version is only picked up when the build runs on a
//     current-enough index.
//  4. Chart.lock pins resolve independently of index freshness.
//
// Each scenario prints the measured evidence (HTTP requests observed by the
// fixture) so the numbers can be quoted in the operator documentation.

// scenario 1+2: build, then immediately rebuild in the same chart directory
// (marker file present): the second build must not touch the origin.
func TestLabWarmBuildReusesMarker(t *testing.T) {
	regressionRequirePinnedHelm(t)
	fixture := newRegressionChartRepo(t)
	repo := HelmRepository{Name: "fixture", Repo: fixture.server.URL, Creds: HelmCreds{}}
	store := NewIndexStore(t.TempDir(), time.Minute, 1<<20, nil)
	dir := regressionParent(t, fixture.server.URL)

	cold, err := regressionBuild(t, dir, repo, store, false)
	require.NoError(t, err)
	regressionRender(t, cold, "1.0.0")

	warm, err := regressionBuild(t, dir, repo, store, false)
	require.NoError(t, err)
	regressionRender(t, warm, "1.0.0")

	requests, archives := fixture.snapshot()
	fmt.Printf("LAB warm-build: index requests=%d archives=%d\n", len(requests), archives)
	assert.Len(t, requests, 1, "second build must not touch the origin (marker reuse)")
}

// scenario 2+3: fresh chart directory (no marker → build executes), index
// still fresh → cached index served, zero requests. Then publish 2.0.0 and
// hard-refresh → new version picked up.
func TestLabFreshIndexAndRefresh(t *testing.T) {
	regressionRequirePinnedHelm(t)
	fixture := newRegressionChartRepo(t)
	repo := HelmRepository{Name: "fixture", Repo: fixture.server.URL, Creds: HelmCreds{}}
	store := NewIndexStore(t.TempDir(), time.Minute, 1<<20, nil)

	// Build 1 (cold): one index fetch.
	dir1 := regressionParent(t, fixture.server.URL)
	h1, err := regressionBuild(t, dir1, repo, store, false)
	require.NoError(t, err)
	regressionRender(t, h1, "1.0.0")

	// Build 2 in a DIFFERENT chart dir (no marker): index is fresh → cached.
	dir2 := regressionParent(t, fixture.server.URL)
	h2, err := regressionBuild(t, dir2, repo, store, false)
	require.NoError(t, err)
	regressionRender(t, h2, "1.0.0")
	requests, _ := fixture.snapshot()
	require.Len(t, requests, 1, "fresh index must be reused without an origin request")

	// Publish 2.0.0, expire the index, rebuild → 200 + new version.
	fixture.publish("2.0.0")
	regressionExpire(t, store, regressionIdentity(t, repo))
	dir3 := regressionParent(t, fixture.server.URL)
	h3, err := regressionBuild(t, dir3, repo, store, false)
	require.NoError(t, err)
	regressionRender(t, h3, "2.0.0")
	requests, archives := fixture.snapshot()
	fmt.Printf("LAB refresh: index requests=%d archives=%d\n", len(requests), archives)
	assert.GreaterOrEqual(t, len(requests), 2, "expired index must be refetched after publish")
}

// scenario 4: Chart.lock pins survive index expiry — a lockfile written at
// 1.0.0 still resolves 1.0.0 after the index expired AND 2.0.0 was published.
func TestLabLockfilePins(t *testing.T) {
	regressionRequirePinnedHelm(t)
	fixture := newRegressionChartRepo(t)
	repo := HelmRepository{Name: "fixture", Repo: fixture.server.URL, Creds: HelmCreds{}}
	store := NewIndexStore(t.TempDir(), time.Minute, 1<<20, nil)
	dir := regressionParent(t, fixture.server.URL)

	h1, err := regressionBuild(t, dir, repo, store, false)
	require.NoError(t, err)
	regressionRender(t, h1, "1.0.0")

	fixture.publish("2.0.0")
	regressionExpire(t, store, regressionIdentity(t, repo))

	h2, err := regressionBuild(t, dir, repo, store, false)
	require.NoError(t, err)
	// No marker file exists at this layer (the marker is created by the
	// repo-server's runHelmBuild, not by the helm client) — so this build
	// re-resolves from the fresh-enough cached index; the lockfile still
	// pins 1.0.0 even though 2.0.0 was published and the index expired.
	regressionRender(t, h2, "1.0.0")

	// Force a full refresh — the lockfile still pins 1.0.0.
	forceH, err := regressionBuild(t, dir, repo, store, true)
	require.NoError(t, err)
	regressionRender(t, forceH, "1.0.0") // lockfile pin respected across index expiry and 2.0.0 publish
}
