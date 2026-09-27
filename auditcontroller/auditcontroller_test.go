package auditcontroller

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/util/audit"
)

func decodeLines(t *testing.T, buf *bytes.Buffer) []audit.Record {
	t.Helper()
	var records []audit.Record
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var r audit.Record
		require.NoError(t, json.Unmarshal([]byte(line), &r))
		records = append(records, r)
	}
	return records
}

func TestRecorder_WritesJSONLinesAndNormalizes(t *testing.T) {
	var out bytes.Buffer
	rec := NewRecorder(10, NewMetrics(), &out)
	fixed := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	rec.now = func() time.Time { return fixed }

	require.NoError(t, rec.Record(audit.Record{Action: "application.sync", Actor: audit.Actor{Username: "admin"}}))

	lines := decodeLines(t, &out)
	require.Len(t, lines, 1)
	r := lines[0]
	assert.Equal(t, audit.RecordKind, r.Kind)
	assert.NotEmpty(t, r.ID)
	assert.Equal(t, fixed, r.Timestamp)
	require.NotNil(t, r.ReceivedAt)
	assert.Equal(t, fixed, *r.ReceivedAt)
	assert.Equal(t, audit.ResultSuccess, r.Result.Status)
	assert.Equal(t, "admin", r.Actor.Username)
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestRecorder_ReportsWriteErrors(t *testing.T) {
	var out bytes.Buffer
	rec := NewRecorder(10, NewMetrics(), failingWriter{}, &out)
	require.Error(t, rec.Record(audit.Record{Action: "application.sync"}))
	// The other outputs still get the record.
	assert.Len(t, decodeLines(t, &out), 1)
}

func TestRecorder_RecentIsBoundedAndFiltered(t *testing.T) {
	rec := NewRecorder(3, nil)
	base := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	for i, u := range []string{"alice", "bob", "alice", "carol", "alice"} {
		require.NoError(t, rec.Record(audit.Record{
			Timestamp: base.Add(time.Duration(i) * time.Minute),
			Actor:     audit.Actor{Username: u},
			Action:    "application.sync",
			Resource:  audit.Resource{Type: "application", Name: u + "-app"},
		}))
	}
	all := rec.Recent(Filter{}, 0)
	require.Len(t, all, 3, "only the last 3 records are buffered")
	assert.Equal(t, []string{"alice", "carol", "alice"}, []string{all[0].Actor.Username, all[1].Actor.Username, all[2].Actor.Username})

	alice := rec.Recent(Filter{User: "ALICE"}, 0)
	require.Len(t, alice, 2)
	assert.True(t, alice[0].Timestamp.After(alice[1].Timestamp), "newest first")

	assert.Len(t, rec.Recent(Filter{}, 1), 1)
	assert.Len(t, rec.Recent(Filter{Since: base.Add(4 * time.Minute)}, 0), 1)
	assert.Empty(t, rec.Recent(Filter{ResourceName: "bob-app"}, 0))
}

func TestServer_Ingest(t *testing.T) {
	var out bytes.Buffer
	metrics := NewMetrics()
	rec := NewRecorder(100, metrics, &out)
	srv := httptest.NewServer(NewServer(rec, "s3cret", metrics).Handler())
	defer srv.Close()

	post := func(token, body string) *http.Response {
		req, err := http.NewRequest(http.MethodPost, srv.URL+audit.IngestPath, strings.NewReader(body))
		require.NoError(t, err)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		resp.Body.Close()
		return resp
	}

	assert.Equal(t, http.StatusUnauthorized, post("", `[]`).StatusCode)
	assert.Equal(t, http.StatusUnauthorized, post("wrong", `[]`).StatusCode)
	assert.Equal(t, http.StatusBadRequest, post("s3cret", `{not json`).StatusCode)
	assert.Empty(t, out.String())

	body := `[{"action":"application.sync","actor":{"username":"alice"},"resource":{"type":"application","name":"guestbook"},"result":{"status":"success","code":"OK"}},
	          {"action":"cluster.delete","actor":{"username":"bob"},"resource":{"type":"cluster","name":"https://kubernetes.default.svc"},"result":{"status":"failure","code":"PermissionDenied"}}]`
	assert.Equal(t, http.StatusAccepted, post("s3cret", body).StatusCode)

	lines := decodeLines(t, &out)
	require.Len(t, lines, 2)
	assert.Equal(t, "alice", lines[0].Actor.Username)
	assert.Equal(t, audit.SourceAPIServer, lines[0].Source, "source defaults to the API server")
	assert.Equal(t, "failure", lines[1].Result.Status)
}

func TestServer_Query(t *testing.T) {
	rec := NewRecorder(100, nil)
	for _, u := range []string{"alice", "bob", "alice"} {
		require.NoError(t, rec.Record(audit.Record{Actor: audit.Actor{Username: u}, Action: "application.sync"}))
	}
	srv := httptest.NewServer(NewServer(rec, "s3cret", nil).Handler())
	defer srv.Close()

	get := func(query, token string) (*http.Response, []audit.Record) {
		req, err := http.NewRequest(http.MethodGet, srv.URL+audit.QueryPath+query, http.NoBody)
		require.NoError(t, err)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		var records []audit.Record
		if resp.StatusCode == http.StatusOK {
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&records))
		}
		return resp, records
	}

	resp, _ := get("", "")
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "reading the audit trail requires the token")

	resp, records := get("?user=alice", "s3cret")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Len(t, records, 2)

	_, records = get("?limit=1", "s3cret")
	assert.Len(t, records, 1)

	_, records = get("?since=1h", "s3cret")
	assert.Len(t, records, 3)

	resp, _ = get("?since=yesterday", "s3cret")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp, _ = get("?limit=-1", "s3cret")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestServer_Healthz(t *testing.T) {
	srv := httptest.NewServer(NewServer(NewRecorder(1, nil), "s3cret", nil).Handler())
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/healthz")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func appEvent(reason, eventType, message, user string, ts time.Time) *corev1.Event {
	e := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "guestbook.1",
			Namespace: "argocd",
			UID:       "uid-1",
			Labels:    map[string]string{"project": "default"},
			Annotations: map[string]string{
				"dest-server":    "https://kubernetes.default.svc",
				"dest-namespace": "default",
			},
		},
		InvolvedObject: corev1.ObjectReference{
			APIVersion: "argoproj.io/v1alpha1",
			Kind:       "Application",
			Name:       "guestbook",
			Namespace:  "argocd",
		},
		Reason:        reason,
		Type:          eventType,
		Message:       message,
		Source:        corev1.EventSource{Component: common.CommandApplicationController},
		LastTimestamp: metav1.NewTime(ts),
		Count:         1,
	}
	if user != "" {
		e.Annotations["user"] = user
	}
	return e
}

func newTestWatcher(t *testing.T) (*EventWatcher, *bytes.Buffer) {
	t.Helper()
	var out bytes.Buffer
	metrics := NewMetrics()
	w := NewEventWatcher(fake.NewClientset(), NewRecorder(100, metrics, &out), metrics, EventWatcherOptions{
		Reasons:        DefaultEventReasons,
		IgnoredSources: DefaultIgnoredEventSources,
	})
	w.startedAt = time.Date(2026, 9, 27, 9, 0, 0, 0, time.UTC)
	return w, &out
}

func TestEventWatcher_AutomatedSync(t *testing.T) {
	w, out := newTestWatcher(t)
	ts := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	w.handle(appEvent("OperationStarted", corev1.EventTypeNormal, "Initiated automated sync to 'abc123'", "", ts))

	lines := decodeLines(t, out)
	require.Len(t, lines, 1)
	r := lines[0]
	assert.Equal(t, audit.SourceKubernetesEvent, r.Source)
	assert.Equal(t, "system:"+common.CommandApplicationController, r.Actor.Username)
	assert.Equal(t, "application.operation-started", r.Action)
	assert.Equal(t, audit.Resource{Type: "application", Name: "guestbook", Namespace: "argocd", Project: "default"}, r.Resource)
	assert.Equal(t, "true", r.Details["automated"])
	assert.Equal(t, "https://kubernetes.default.svc", r.Details["dest-server"])
	assert.Equal(t, ts, r.Timestamp)
	assert.Equal(t, audit.ResultSuccess, r.Result.Status)
}

func TestEventWatcher_SyncCompletedByUser(t *testing.T) {
	w, out := newTestWatcher(t)
	w.handle(appEvent("OperationCompleted", corev1.EventTypeWarning, "Sync operation to abc123 failed: one or more objects failed to apply", "alice@example.com", time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)))

	lines := decodeLines(t, out)
	require.Len(t, lines, 1)
	assert.Equal(t, "alice@example.com", lines[0].Actor.Username)
	assert.Equal(t, "application.operation-completed", lines[0].Action)
	assert.Equal(t, audit.ResultFailure, lines[0].Result.Status)
	assert.Equal(t, "OperationCompleted", lines[0].Result.Code)
}

func TestEventWatcher_Skips(t *testing.T) {
	w, out := newTestWatcher(t)
	ts := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)

	// Events of the API server are ignored: it ships richer records itself.
	e := appEvent("OperationStarted", corev1.EventTypeNormal, "Initiated sync", "admin", ts)
	e.Source.Component = common.CommandServer
	rec, reason := w.toRecord(e)
	assert.Nil(t, rec)
	assert.Equal(t, "ignored_source", reason)

	// Status changes describe the cluster, not what somebody did.
	rec, reason = w.toRecord(appEvent("ResourceUpdated", corev1.EventTypeNormal, "Updated sync status: Synced -> OutOfSync", "", ts))
	assert.Nil(t, rec)
	assert.Equal(t, "ignored_reason", reason)

	// Events from before the controller started are not replayed.
	rec, reason = w.toRecord(appEvent("OperationStarted", corev1.EventTypeNormal, "Initiated automated sync", "", ts.Add(-24*time.Hour)))
	assert.Nil(t, rec)
	assert.Equal(t, "before_start", reason)

	// The same event (same UID and count) is only recorded once, e.g. after a watch re-list.
	e = appEvent("OperationStarted", corev1.EventTypeNormal, "Initiated automated sync", "", ts)
	w.handle(e)
	w.handle(e)
	// Kubernetes aggregates a repeated event by bumping its count: that is a new occurrence.
	e2 := e.DeepCopy()
	e2.Count = 2
	w.handle(e2)
	assert.Len(t, decodeLines(t, out), 2)
}

func TestEventWatcher_ProjectEvents(t *testing.T) {
	w, out := newTestWatcher(t)
	e := appEvent("ResourceCreated", corev1.EventTypeNormal, "created project", "admin", time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC))
	e.Source.Component = "argocd-applicationset-controller"
	e.InvolvedObject.Kind = "AppProject"
	e.InvolvedObject.Name = "team-a"
	w.handle(e)
	lines := decodeLines(t, out)
	require.Len(t, lines, 1)
	assert.Equal(t, "project.create", lines[0].Action)
	assert.Equal(t, "team-a", lines[0].Resource.Project)
}

func TestSeenCacheForgetsOldest(t *testing.T) {
	c := newSeenCache(2)
	assert.True(t, c.add("a"))
	assert.True(t, c.add("b"))
	assert.False(t, c.add("a"))
	assert.True(t, c.add("c")) // evicts "a"
	assert.True(t, c.add("a"))
	assert.False(t, c.add("c"))
}

func TestParseSince(t *testing.T) {
	now := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	got, err := parseSince("15m", now)
	require.NoError(t, err)
	assert.Equal(t, now.Add(-15*time.Minute), got)
	got, err = parseSince("2026-09-27T08:00:00Z", now)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 9, 27, 8, 0, 0, 0, time.UTC), got)
	_, err = parseSince("yesterday", now)
	assert.Error(t, err)
}
