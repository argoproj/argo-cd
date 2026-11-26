package application

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/health"
	synccommon "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/common"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// uiListFields mirrors the UI's APP_LIST_FIELDS.
var uiListFields = func() map[string]any {
	fields := map[string]any{"metadata.resourceVersion": true}
	for _, f := range []string{
		"metadata.name", "metadata.namespace", "metadata.annotations", "metadata.labels",
		"metadata.creationTimestamp", "metadata.deletionTimestamp", "spec.destination",
		"spec.project", "spec.source", "spec.sources", "spec.sourceHydrator", "spec.syncPolicy",
		"operation.sync", "status.sourceHydrator", "status.summary", "status.sync.status",
		"status.sync.revision", "status.health", "status.operationState.phase",
		"status.operationState.operation.sync", "status.operationState.startedAt",
		"status.operationState.finishedAt",
	} {
		fields["items."+f] = true
	}
	return fields
}()

// benchApp has a realistically sized status, which dominates list payloads.
func benchApp(i int) v1alpha1.Application {
	now := metav1.NewTime(time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC))
	const resourceCount = 40
	resources := make([]v1alpha1.ResourceStatus, resourceCount)
	syncResults := make([]*v1alpha1.ResourceResult, resourceCount)
	for r := range resourceCount {
		resources[r] = v1alpha1.ResourceStatus{
			Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "ns",
			Name:   fmt.Sprintf("app-%d-res-%d", i, r),
			Status: v1alpha1.SyncStatusCodeSynced,
			Health: &v1alpha1.HealthStatus{Status: health.HealthStatusHealthy},
		}
		syncResults[r] = &v1alpha1.ResourceResult{
			Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "ns",
			Name: resources[r].Name, Status: synccommon.ResultCodeSynced,
			Message: "deployment.apps/x configured", HookPhase: synccommon.OperationRunning,
		}
	}
	history := make(v1alpha1.RevisionHistories, 10)
	for h := range history {
		history[h] = v1alpha1.RevisionHistory{
			ID: int64(h), Revision: fmt.Sprintf("%040d", h), DeployedAt: now, DeployStartedAt: &now,
			Source: v1alpha1.ApplicationSource{RepoURL: "https://github.com/example/repo", Path: "apps", TargetRevision: "HEAD"},
		}
	}
	source := &v1alpha1.ApplicationSource{
		RepoURL: "https://github.com/example/repo", Path: fmt.Sprintf("apps/app-%d", i), TargetRevision: "HEAD",
		Helm: &v1alpha1.ApplicationSourceHelm{ValueFiles: []string{"values.yaml"}, Values: "replicas: 2\nimage: <tag>&x\n"},
	}
	return v1alpha1.Application{
		Name: fmt.Sprintf("app-%d", i), Namespace: "argocd", ResourceVersion: strconv.Itoa(1000 + i),
		CreationTimestamp: now,
		Labels:            map[string]string{"team": "platform", "env": "prod", "tier": "backend", "app": fmt.Sprintf("app-%d", i)},
		Annotations:       map[string]string{"notifications.argoproj.io/subscribe": "slack:<chan>&x"},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default", Source: source,
			Destination: v1alpha1.ApplicationDestination{Server: "https://kubernetes.default.svc", Namespace: "ns"},
			SyncPolicy:  &v1alpha1.SyncPolicy{Automated: &v1alpha1.SyncPolicyAutomated{}, SyncOptions: []string{"CreateNamespace=true"}},
		},
		Operation: &v1alpha1.Operation{Sync: &v1alpha1.SyncOperation{Revision: "abc"}},
		Status: v1alpha1.ApplicationStatus{
			Resources:    resources,
			Sync:         v1alpha1.SyncStatus{Status: v1alpha1.SyncStatusCodeSynced, Revision: "abc", ComparedTo: v1alpha1.ComparedTo{Source: *source}},
			Health:       v1alpha1.AppHealthStatus{Status: health.HealthStatusHealthy},
			History:      history,
			ReconciledAt: &now,
			OperationState: &v1alpha1.OperationState{
				Operation: v1alpha1.Operation{Sync: &v1alpha1.SyncOperation{Revision: "abc"}},
				Phase:     synccommon.OperationSucceeded, Message: "successfully synced", StartedAt: now, FinishedAt: &now,
				SyncResult: &v1alpha1.SyncOperationResult{Resources: syncResults, Revision: "abc", Source: *source},
			},
			Summary: v1alpha1.ApplicationSummary{Images: []string{"nginx:1.27", "redis:7", "busybox"}},
		},
	}
}

func benchList(n int) *v1alpha1.ApplicationList {
	items := make([]v1alpha1.Application, n)
	for i := range items {
		items[i] = benchApp(i)
	}
	return &v1alpha1.ApplicationList{ResourceVersion: "42", Items: items}
}

// partialList varies which parents have values across items.
func partialList() *v1alpha1.ApplicationList {
	list := benchList(4)
	list.Items[1].Status.OperationState = nil
	list.Items[1].Operation = nil
	list.Items[2].Spec.SyncPolicy = nil
	list.Items[2].Status.OperationState.FinishedAt = nil
	list.Items[3].Spec.Source = nil
	list.Items[3].Spec.Sources = v1alpha1.ApplicationSources{{RepoURL: "https://example.com/a"}, {RepoURL: "https://example.com/b"}}
	return list
}

// compactJSON strips whitespace so outputs compare byte for byte.
func compactJSON(t *testing.T, write func(io.Writer) error) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, write(&buf))
	var out bytes.Buffer
	require.NoError(t, json.Compact(&out, buf.Bytes()))
	return out.String()
}

// legacyListJSON is the previous map-based implementation, kept as a reference.
func legacyListJSON(t *testing.T, list *v1alpha1.ApplicationList, fields map[string]any, exclude bool) string {
	t.Helper()
	if len(fields) == 0 {
		b, err := json.Marshal(list)
		require.NoError(t, err)
		return string(b)
	}
	specs := selectAppFields(fields, exclude)
	var items []map[string]any
	if list.Items != nil {
		items = make([]map[string]any, 0, len(list.Items))
	}
	for i := range list.Items {
		converted := map[string]any{}
		for _, spec := range specs {
			value := spec.fn(&list.Items[i])
			if value == nil {
				continue
			}
			item := converted
			for _, part := range spec.parts[:len(spec.parts)-1] {
				if _, ok := item[part]; !ok {
					item[part] = map[string]any{}
				}
				item = item[part].(map[string]any)
			}
			item[spec.parts[len(spec.parts)-1]] = value
		}
		items = append(items, converted)
	}
	b, err := json.Marshal(map[string]any{"metadata": list.ListMeta, "items": items})
	require.NoError(t, err)
	return string(b)
}

// assertMatchesLegacy compares the list semantically (top-level key order
// changed) and each item byte for byte.
func assertMatchesLegacy(t *testing.T, want, got string) {
	t.Helper()
	assert.JSONEq(t, want, got)
	var wantItems, gotItems struct {
		Items []json.RawMessage `json:"items"`
	}
	require.NoError(t, json.Unmarshal([]byte(want), &wantItems))
	require.NoError(t, json.Unmarshal([]byte(got), &gotItems))
	require.Len(t, gotItems.Items, len(wantItems.Items))
	for i := range wantItems.Items {
		var w, g bytes.Buffer
		require.NoError(t, json.Compact(&w, wantItems.Items[i]))
		require.NoError(t, json.Compact(&g, gotItems.Items[i]))
		assert.Equal(t, w.String(), g.String(), "item %d", i)
	}
}

func TestStreamApplicationListJSON_MatchesLegacy(t *testing.T) {
	withTypeMeta := benchList(3)
	withTypeMeta.TypeMeta = metav1.TypeMeta{Kind: "ApplicationList", APIVersion: "argoproj.io/v1alpha1"}
	tests := []struct {
		name    string
		list    *v1alpha1.ApplicationList
		fields  map[string]any
		exclude bool
	}{
		{"unfiltered", benchList(3), nil, false},
		{"unfiltered with type meta", withTypeMeta, nil, false},
		{"ui field filter", benchList(3), uiListFields, false},
		{"ui field filter drops type meta", withTypeMeta, uiListFields, false},
		{"exclude filter", benchList(3), map[string]any{"items.status.resources": true, "items.spec": true}, true},
		{"nil items", &v1alpha1.ApplicationList{ResourceVersion: "1"}, nil, false},
		{"nil items filtered", &v1alpha1.ApplicationList{ResourceVersion: "1"}, uiListFields, false},
		{"empty items", &v1alpha1.ApplicationList{Items: []v1alpha1.Application{}}, nil, false},
		{"empty items filtered", &v1alpha1.ApplicationList{Items: []v1alpha1.Application{}}, uiListFields, false},
		{"zero-value app", &v1alpha1.ApplicationList{Items: []v1alpha1.Application{{}}}, nil, false},
		// Parents whose children are all nil are omitted.
		{"zero-value app filtered", &v1alpha1.ApplicationList{Items: []v1alpha1.Application{{}}}, uiListFields, false},
		{"partial app filtered", partialList(), uiListFields, false},
		// Dotted paths that string-sort differently from per-level keys.
		{"sibling key order", benchList(2), map[string]any{
			"items.spec.sources": true, "items.spec.source": true, "items.spec.sourceHydrator": true,
			"items.status.operationState.startedAt": true, "items.status.operationState.operation.sync": true,
			"items.status.operationState.finishedAt": true, "items.status.operationState.phase": true,
		}, false},
		// Every appFields entry; "spec" covers its descendants.
		{"all fields", benchList(2), map[string]any{"items.unknown": true}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			streamed := compactJSON(t, func(w io.Writer) error { return streamApplicationListJSON(w, tt.list, tt.fields, tt.exclude) })
			assertMatchesLegacy(t, legacyListJSON(t, tt.list, tt.fields, tt.exclude), streamed)
			if len(tt.fields) == 0 {
				return // the event-stream branch only runs the processor when fields are selected
			}
			// Both List branches produce identical bytes.
			assert.Equal(t, streamed, marshalProcessed(t, tt.list, tt.fields, tt.exclude))
		})
	}
}

func marshalProcessed(t *testing.T, list *v1alpha1.ApplicationList, fields map[string]any, exclude bool) string {
	t.Helper()
	res, err := processApplicationListField(list, fields, exclude)
	require.NoError(t, err)
	// As argoproj/pkg serves it.
	b, err := json.Marshal(res)
	require.NoError(t, err)
	var out bytes.Buffer
	require.NoError(t, json.Compact(&out, b))
	return out.String()
}

// An empty list is [] in both branches (the event-stream branch used to write null).
func TestProcessApplicationListField_EmptyItems(t *testing.T) {
	list := &v1alpha1.ApplicationList{ResourceVersion: "1", Items: []v1alpha1.Application{}}
	assert.JSONEq(t, `{"metadata":{"resourceVersion":"1"},"items":[]}`, marshalProcessed(t, list, uiListFields, false))
}

func TestProcessApplicationListField_RejectsOtherTypes(t *testing.T) {
	_, err := processApplicationListField(&v1alpha1.Application{}, uiListFields, false)
	require.Error(t, err)
}

// chunkWriter records the largest Write, showing the response is streamed.
type chunkWriter struct{ max, total int }

func (c *chunkWriter) Write(p []byte) (int, error) {
	c.max = max(c.max, len(p))
	c.total += len(p)
	return len(p), nil
}

func BenchmarkStreamApplicationListJSON(b *testing.B) {
	for _, n := range []int{100, 1000, 5000} {
		list := benchList(n)
		for _, mode := range []struct {
			name   string
			fields map[string]any
		}{{"full", nil}, {"ui-fields", uiListFields}} {
			b.Run(fmt.Sprintf("apps=%d/mode=%s", n, mode.name), func(b *testing.B) {
				b.ReportAllocs()
				var cw chunkWriter
				for b.Loop() {
					cw = chunkWriter{}
					if err := streamApplicationListJSON(&cw, list, mode.fields, false); err != nil {
						b.Fatal(err)
					}
				}
				b.ReportMetric(float64(cw.max), "max-write-B")
				b.ReportMetric(float64(cw.total), "resp-B")
			})
		}
	}
}
