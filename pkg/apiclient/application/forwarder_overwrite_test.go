package application

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	//nolint:staticcheck
	"github.com/golang/protobuf/proto"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	argohttp "github.com/argoproj/pkg/v2/grpc/http"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestProcessApplicationListField_SyncOperation(t *testing.T) {
	t.Parallel()
	list := v1alpha1.ApplicationList{
		Items: []v1alpha1.Application{{Operation: &v1alpha1.Operation{Sync: &v1alpha1.SyncOperation{
			Revision: "abc",
		}}}},
	}

	res, err := processApplicationListField(&list, map[string]any{"items.operation.sync": true}, false)
	require.NoError(t, err)
	item := processedItem(t, res, 0)

	val, ok, err := unstructured.NestedString(item, "operation", "sync", "revision")
	require.NoError(t, err)
	require.True(t, ok)

	require.Equal(t, "abc", val)
}

func TestProcessApplicationListField_SyncOperationMissing(t *testing.T) {
	t.Parallel()
	list := v1alpha1.ApplicationList{
		Items: []v1alpha1.Application{{Operation: nil}},
	}

	res, err := processApplicationListField(&list, map[string]any{"items.operation.sync": true}, false)
	require.NoError(t, err)
	item := processedItem(t, res, 0)

	_, ok, err := unstructured.NestedString(item, "operation")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestProcessApplicationListField_OperationStateOperationSync(t *testing.T) {
	t.Parallel()
	list := v1alpha1.ApplicationList{
		Items: []v1alpha1.Application{{Status: v1alpha1.ApplicationStatus{
			OperationState: &v1alpha1.OperationState{
				Operation: v1alpha1.Operation{Sync: &v1alpha1.SyncOperation{
					Revision:  "abc",
					Resources: []v1alpha1.SyncOperationResource{{Group: "apps", Kind: "Deployment", Name: "web"}},
					Manifests: []string{"apiVersion: v1\nkind: ConfigMap"},
				}},
				SyncResult: &v1alpha1.SyncOperationResult{Revision: "def"},
			},
		}}},
	}

	res, err := processApplicationListField(&list, map[string]any{"items.status.operationState.operation.sync": true}, false)
	require.NoError(t, err)
	item := processedItem(t, res, 0)

	syncMap, ok, err := unstructured.NestedMap(item, "status", "operationState", "operation", "sync")
	require.NoError(t, err)
	require.True(t, ok)
	// The field is a pure presence marker: it must be an empty object, with none
	// of the operation's payload (revision, resources, manifests, sources).
	require.Empty(t, syncMap)
}

func TestProcessApplicationListField_OperationStateOperationSyncMultiSource(t *testing.T) {
	t.Parallel()
	list := v1alpha1.ApplicationList{
		Items: []v1alpha1.Application{{Status: v1alpha1.ApplicationStatus{
			OperationState: &v1alpha1.OperationState{
				Operation: v1alpha1.Operation{Sync: &v1alpha1.SyncOperation{
					Revisions: []string{"abc", "def"},
					Sources: v1alpha1.ApplicationSources{
						{RepoURL: "https://example.com/repo1.git"},
						{RepoURL: "https://example.com/repo2.git"},
					},
				}},
			},
		}}},
	}

	res, err := processApplicationListField(&list, map[string]any{"items.status.operationState.operation.sync": true}, false)
	require.NoError(t, err)
	item := processedItem(t, res, 0)

	syncMap, ok, err := unstructured.NestedMap(item, "status", "operationState", "operation", "sync")
	require.NoError(t, err)
	require.True(t, ok)
	// Multi-source operations get the same empty presence marker; their sources
	// and revisions must not leak into the list payload.
	require.Empty(t, syncMap)
}

func TestProcessApplicationListField_OperationStateOperationSyncMissing(t *testing.T) {
	t.Parallel()
	list := v1alpha1.ApplicationList{
		Items: []v1alpha1.Application{{Status: v1alpha1.ApplicationStatus{
			OperationState: &v1alpha1.OperationState{Operation: v1alpha1.Operation{Sync: nil}},
		}}},
	}

	res, err := processApplicationListField(&list, map[string]any{"items.status.operationState.operation.sync": true}, false)
	require.NoError(t, err)
	item := processedItem(t, res, 0)

	_, ok, err := unstructured.NestedMap(item, "status")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestStreamApplicationListJSON_WithFieldFilter(t *testing.T) {
	list := &v1alpha1.ApplicationList{
		ResourceVersion: "100",
		Items: []v1alpha1.Application{
			{
				Name: "app1", Namespace: "default",
				Status: v1alpha1.ApplicationStatus{Health: v1alpha1.AppHealthStatus{Status: "Healthy"}},
			},
			{
				Name: "app2", Namespace: "default",
				Status: v1alpha1.ApplicationStatus{Health: v1alpha1.AppHealthStatus{Status: "Degraded"}},
			},
		},
	}

	fields := map[string]any{"items.metadata.name": true, "items.status.health": true}
	var buf bytes.Buffer
	err := streamApplicationListJSON(&buf, list, fields, false)
	require.NoError(t, err)

	var result map[string]any
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)

	items, ok := result["items"].([]any)
	require.True(t, ok)
	assert.Len(t, items, 2)

	item0 := items[0].(map[string]any)
	metadata0 := item0["metadata"].(map[string]any)
	assert.Equal(t, "app1", metadata0["name"])

	item1 := items[1].(map[string]any)
	metadata1 := item1["metadata"].(map[string]any)
	assert.Equal(t, "app2", metadata1["name"])

	meta := result["metadata"].(map[string]any)
	assert.Equal(t, "100", meta["resourceVersion"])
}

func TestStreamApplicationListJSON_NoFieldFilter(t *testing.T) {
	list := &v1alpha1.ApplicationList{
		ResourceVersion: "42",
		Items: []v1alpha1.Application{
			{Name: "myapp", Namespace: "ns"},
		},
	}

	var buf bytes.Buffer
	err := streamApplicationListJSON(&buf, list, nil, false)
	require.NoError(t, err)

	var result map[string]any
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)

	items, ok := result["items"].([]any)
	require.True(t, ok)
	assert.Len(t, items, 1)

	item0 := items[0].(map[string]any)
	metadata0 := item0["metadata"].(map[string]any)
	assert.Equal(t, "myapp", metadata0["name"])
	assert.Equal(t, "ns", metadata0["namespace"])
}

func TestStreamApplicationListJSON_EmptyList(t *testing.T) {
	list := &v1alpha1.ApplicationList{
		ResourceVersion: "1",
		Items:           []v1alpha1.Application{},
	}

	var buf bytes.Buffer
	err := streamApplicationListJSON(&buf, list, nil, false)
	require.NoError(t, err)

	var result map[string]any
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)

	items, ok := result["items"].([]any)
	require.True(t, ok)
	assert.Empty(t, items)
}

func TestStreamApplicationListJSON_NilItems(t *testing.T) {
	list := &v1alpha1.ApplicationList{
		ResourceVersion: "1",
		Items:           nil,
	}

	var buf bytes.Buffer
	err := streamApplicationListJSON(&buf, list, nil, false)
	require.NoError(t, err)

	var result map[string]any
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)

	// nil items should produce "items":null, not "items":[]
	assert.Nil(t, result["items"])
}

// TestStreamApplicationListJSON_NilVsEmptyMatchesJSONMarshal verifies that
// the streaming output for nil and empty items matches json.Marshal behavior.
func TestStreamApplicationListJSON_NilVsEmptyMatchesJSONMarshal(t *testing.T) {
	tests := []struct {
		name  string
		items []v1alpha1.Application
	}{
		{"nil items", nil},
		{"empty items", []v1alpha1.Application{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list := &v1alpha1.ApplicationList{
				ResourceVersion: "1",
				Items:           tt.items,
			}

			oldJSON, err := json.Marshal(list)
			require.NoError(t, err)

			var buf bytes.Buffer
			err = streamApplicationListJSON(&buf, list, nil, false)
			require.NoError(t, err)

			var oldParsed, newParsed map[string]any
			require.NoError(t, json.Unmarshal(oldJSON, &oldParsed))
			require.NoError(t, json.Unmarshal(buf.Bytes(), &newParsed))
			assert.Equal(t, oldParsed, newParsed)
		})
	}
}

func TestStreamApplicationListJSON_MatchesProcessApplicationListField(t *testing.T) {
	list := &v1alpha1.ApplicationList{
		ResourceVersion: "99",
		Items: []v1alpha1.Application{
			{
				Name:      "app1",
				Operation: &v1alpha1.Operation{Sync: &v1alpha1.SyncOperation{Revision: "abc"}},
				Status: v1alpha1.ApplicationStatus{
					Health: v1alpha1.AppHealthStatus{Status: "Healthy"},
					Sync:   v1alpha1.SyncStatus{Status: "Synced"},
				},
			},
		},
	}
	fields := map[string]any{
		"items.metadata.name":      true,
		"items.operation.sync":     true,
		"items.status.health":      true,
		"items.status.sync.status": true,
	}

	// Get result from the batch processApplicationListField
	batchResult, err := processApplicationListField(list, fields, false)
	require.NoError(t, err)
	batchJSON, err := json.Marshal(batchResult)
	require.NoError(t, err)

	// Get result from streaming
	var buf bytes.Buffer
	err = streamApplicationListJSON(&buf, list, fields, false)
	require.NoError(t, err)

	// Both should parse to equivalent structures
	var batchParsed, streamParsed map[string]any
	require.NoError(t, json.Unmarshal(batchJSON, &batchParsed))
	require.NoError(t, json.Unmarshal(buf.Bytes(), &streamParsed))
	assert.Equal(t, batchParsed, streamParsed)
}

// TestStreamApplicationListJSON_MatchesJSONMarshal verifies that the unfiltered streaming
// path produces the same parsed JSON as json.Marshal on the full ApplicationList struct.
func TestStreamApplicationListJSON_MatchesJSONMarshal(t *testing.T) {
	list := &v1alpha1.ApplicationList{
		ResourceVersion: "200",
		Items: []v1alpha1.Application{
			{
				Name:      "app1",
				Namespace: "ns1",
				Labels:    map[string]string{"env": "prod"},
				Spec: v1alpha1.ApplicationSpec{
					Project: "default",
					Sources: v1alpha1.ApplicationSources{{RepoURL: "https://github.com/example/repo"}},
				},
				Status: v1alpha1.ApplicationStatus{
					Health: v1alpha1.AppHealthStatus{Status: "Healthy"},
					Sync:   v1alpha1.SyncStatus{Status: "Synced"},
					Resources: []v1alpha1.ResourceStatus{
						{Group: "apps", Kind: "Deployment", Name: "nginx"},
					},
				},
			},
			{
				Name: "app2", Namespace: "ns2",
				Operation: &v1alpha1.Operation{Sync: &v1alpha1.SyncOperation{Revision: "def456"}},
			},
		},
	}

	// Old path: json.Marshal the full struct
	oldJSON, err := json.Marshal(list)
	require.NoError(t, err)

	// New path: streaming
	var buf bytes.Buffer
	err = streamApplicationListJSON(&buf, list, nil, false)
	require.NoError(t, err)

	var oldParsed, newParsed map[string]any
	require.NoError(t, json.Unmarshal(oldJSON, &oldParsed))
	require.NoError(t, json.Unmarshal(buf.Bytes(), &newParsed))
	assert.Equal(t, oldParsed, newParsed)
}

// TestStreamApplicationListJSON_MatchesJSONMarshal_WithTypeMeta verifies TypeMeta fields
// are preserved in the unfiltered streaming path.
func TestStreamApplicationListJSON_MatchesJSONMarshal_WithTypeMeta(t *testing.T) {
	list := &v1alpha1.ApplicationList{
		Kind: "ApplicationList", APIVersion: "argoproj.io/v1alpha1",
		ResourceVersion: "300",
		Items: []v1alpha1.Application{
			{Name: "app1"},
		},
	}

	oldJSON, err := json.Marshal(list)
	require.NoError(t, err)

	var buf bytes.Buffer
	err = streamApplicationListJSON(&buf, list, nil, false)
	require.NoError(t, err)

	var oldParsed, newParsed map[string]any
	require.NoError(t, json.Unmarshal(oldJSON, &oldParsed))
	require.NoError(t, json.Unmarshal(buf.Bytes(), &newParsed))
	assert.Equal(t, oldParsed, newParsed)
}

// TestStreamApplicationListJSON_MatchesFieldFilter_AllFields exercises every entry
// in the appFields map to ensure streaming and batch produce the same result.
func TestStreamApplicationListJSON_MatchesFieldFilter_AllFields(t *testing.T) {
	startedAt := metav1.Now()
	finishedAt := metav1.Now()
	list := &v1alpha1.ApplicationList{
		ResourceVersion: "500",
		Items: []v1alpha1.Application{
			{
				Name:              "full-app",
				Namespace:         "argocd",
				Annotations:       map[string]string{"note": "test"},
				Labels:            map[string]string{"team": "platform"},
				CreationTimestamp: metav1.Now(),
				Spec:              v1alpha1.ApplicationSpec{Project: "default"},
				Operation: &v1alpha1.Operation{
					Sync: &v1alpha1.SyncOperation{Revision: "HEAD"},
				},
				Status: v1alpha1.ApplicationStatus{
					Health:  v1alpha1.AppHealthStatus{Status: "Healthy"},
					Sync:    v1alpha1.SyncStatus{Status: "Synced"},
					Summary: v1alpha1.ApplicationSummary{Images: []string{"nginx:latest"}},
					Resources: []v1alpha1.ResourceStatus{
						{Group: "apps", Kind: "Deployment", Name: "web"},
					},
					OperationState: &v1alpha1.OperationState{
						Phase:      "Succeeded",
						StartedAt:  startedAt,
						FinishedAt: &finishedAt,
						SyncResult: &v1alpha1.SyncOperationResult{Revision: "abc123"},
					},
				},
			},
		},
	}

	// Build fields map with every known appField
	fields := make(map[string]any)
	for field := range appFields {
		fields["items."+field] = true
	}

	// Batch (old path)
	batchResult, err := processApplicationListField(list, fields, false)
	require.NoError(t, err)
	batchJSON, err := json.Marshal(batchResult)
	require.NoError(t, err)

	// Streaming (new path)
	var buf bytes.Buffer
	err = streamApplicationListJSON(&buf, list, fields, false)
	require.NoError(t, err)

	var batchParsed, streamParsed map[string]any
	require.NoError(t, json.Unmarshal(batchJSON, &batchParsed))
	require.NoError(t, json.Unmarshal(buf.Bytes(), &streamParsed))
	assert.Equal(t, batchParsed, streamParsed)
}

// TestSelectAppFields_OverlappingFields ensures that selecting both a field and
// its descendants (e.g. "spec" and "spec.destination", as appFields contains on
// master) keeps only the ancestor, so "spec" is written once, deterministically.
func TestSelectAppFields_OverlappingFields(t *testing.T) {
	// Inject descendant entries unless already present (master defines them).
	overlapping := map[string]func(app *v1alpha1.Application) any{
		"spec.destination": func(app *v1alpha1.Application) any { return app.Spec.Destination },
		"spec.project":     func(app *v1alpha1.Application) any { return app.Spec.Project },
	}
	var injected []string
	for field, fn := range overlapping {
		if _, exists := appFields[field]; !exists {
			appFields[field] = fn
			injected = append(injected, field)
		}
	}
	t.Cleanup(func() {
		for _, field := range injected {
			delete(appFields, field)
		}
	})

	fields := make(map[string]any)
	for field := range appFields {
		fields["items."+field] = true
	}

	app := &v1alpha1.Application{
		Spec: v1alpha1.ApplicationSpec{
			Project:     "default",
			Destination: v1alpha1.ApplicationDestination{Namespace: "prod"},
		},
	}

	// Map iteration order is randomized per range loop, so repeat to cover
	// both "spec"-first and descendant-first orderings.
	var firstJSON []byte
	for range 50 {
		specs := selectAppFields(fields)
		for _, spec := range specs {
			assert.NotEqual(t, "spec.destination", spec.field)
			assert.NotEqual(t, "spec.project", spec.field)
		}
		convertedJSON := encodeApp(t, app, specs)
		if firstJSON == nil {
			firstJSON = convertedJSON
		} else {
			assert.JSONEq(t, string(firstJSON), string(convertedJSON))
		}
	}

	// The full spec must be present, covering the dropped descendants.
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(firstJSON, &parsed))
	spec, ok := parsed["spec"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "default", spec["project"])
}

// Exclude mode must match the generic argoproj/pkg filter: the full object
// minus the excluded paths and everything under them (#31204).
func TestListForwarder_ExcludeMatchesGenericFilter(t *testing.T) {
	list := benchList(2)
	list.TypeMeta = metav1.TypeMeta{Kind: "ApplicationList", APIVersion: "argoproj.io/v1alpha1"}
	list.Items[0].Status.Conditions = []v1alpha1.ApplicationCondition{{Type: v1alpha1.ApplicationConditionComparisonError, Message: "boom"}}
	list.Items[0].Annotations = map[string]string{"a": "1"}

	tests := []struct {
		fields  string
		absent  []string
		present []string
	}{
		{"-items.spec", []string{"spec"}, []string{"metadata.name", "status.conditions", "status.resources"}},
		{"-items.spec.project", []string{"spec.project"}, []string{"spec.destination", "spec.source"}},
		{"-items.status.conditions", []string{"status.conditions"}, []string{"spec.project", "status.health"}},
		{"-items.spec,items.status.resources,items.metadata.annotations", []string{"spec", "status.resources", "metadata.annotations"}, []string{"metadata.name", "status.sync"}},
		// Paths through arrays don't add a segment.
		{"-items.status.resources.name", []string{"status.resources.name"}, []string{"status.resources.kind"}},
		{"-items.unknown", nil, []string{"spec", "status.conditions"}},
		{"-metadata,kind", nil, nil},
		{"-items", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.fields, func(t *testing.T) {
			url := "/api/v1/applications?fields=" + tt.fields
			generic := httptest.NewRecorder()
			argohttp.UnaryForwarder(t.Context(), runtime.NewServeMux(), nil, generic,
				httptest.NewRequestWithContext(t.Context(), http.MethodGet, url, http.NoBody), list)

			for _, accept := range []string{"application/json", "text/event-stream"} {
				t.Run(accept, func(t *testing.T) {
					req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, url, http.NoBody)
					req.Header.Set("Accept", accept)
					rec := httptest.NewRecorder()
					forward_ApplicationService_List_0(t.Context(), runtime.NewServeMux(), nil, rec, req, list)
					body := strings.TrimSpace(strings.TrimPrefix(rec.Body.String(), "data: "))
					assert.JSONEq(t, generic.Body.String(), body)

					var got struct {
						Items []map[string]any `json:"items"`
					}
					require.NoError(t, json.Unmarshal([]byte(body), &got))
					for _, path := range tt.absent {
						assert.False(t, hasPath(got.Items[0], path), "%s should be excluded", path)
					}
					for _, path := range tt.present {
						assert.True(t, hasPath(got.Items[0], path), "%s should be kept", path)
					}
				})
			}
		})
	}
}

// hasPath reports whether the dotted path exists in v, descending into the
// first element of arrays.
func hasPath(v any, path string) bool {
	for part := range strings.SplitSeq(path, ".") {
		if arr, ok := v.([]any); ok {
			if len(arr) == 0 {
				return false
			}
			v = arr[0]
		}
		m, ok := v.(map[string]any)
		if !ok {
			return false
		}
		if v, ok = m[part]; !ok {
			return false
		}
	}
	return true
}

// TestForwarder_HeadersMatchForwardResponseMessage compares the HTTP headers produced by
// our streaming forwarder against runtime.ForwardResponseMessage (via UnaryForwarderWithFieldProcessor)
// to verify that gRPC metadata, trailers, Content-Type, and forward-response options are handled identically.
func TestForwarder_HeadersMatchForwardResponseMessage(t *testing.T) {
	list := &v1alpha1.ApplicationList{
		ResourceVersion: "700",
		Items: []v1alpha1.Application{
			{Name: "app1"},
		},
	}

	// Build context with gRPC server metadata
	grpcMD := runtime.ServerMetadata{
		HeaderMD: metadata.MD{
			"x-request-id": []string{"req-123"},
			"x-ratelimit":  []string{"100", "200"},
		},
		TrailerMD: metadata.MD{
			"x-stream-id": []string{"stream-abc"},
		},
	}
	ctx := runtime.NewServerMetadataContext(context.Background(), grpcMD)
	mux := runtime.NewServeMux()

	// Track whether forward-response opts are called
	optCalled := false
	testOpt := func(_ context.Context, w http.ResponseWriter, _ proto.Message) error {
		w.Header().Set("X-Test-Opt", "applied")
		optCalled = true
		return nil
	}

	// --- Old path: UnaryForwarderWithFieldProcessor via ForwardResponseMessage ---
	oldForwarder := argohttp.UnaryForwarderWithFieldProcessor(processApplicationListField)
	oldRec := httptest.NewRecorder()
	oldReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/applications?fields=items.metadata.name", http.NoBody)
	oldForwarder(ctx, mux, nil, oldRec, oldReq, list, testOpt)
	require.True(t, optCalled, "old path should call forward-response option")

	// --- New path: streaming forwarder ---
	optCalled = false
	newRec := httptest.NewRecorder()
	newReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/applications?fields=items.metadata.name", http.NoBody)
	forward_ApplicationService_List_0(ctx, mux, nil, newRec, newReq, list, testOpt)
	require.True(t, optCalled, "new path should call forward-response option")

	// Compare headers
	assert.Equal(t, oldRec.Header().Get("Content-Type"), newRec.Header().Get("Content-Type"),
		"Content-Type should match")
	assert.Equal(t, "application/json", newRec.Header().Get("Content-Type"))

	// gRPC metadata headers (Grpc-Metadata-* prefix)
	assert.Equal(t, oldRec.Header().Values("Grpc-Metadata-X-Request-Id"), newRec.Header().Values("Grpc-Metadata-X-Request-Id"),
		"gRPC metadata header x-request-id should match")
	assert.Equal(t, oldRec.Header().Values("Grpc-Metadata-X-Ratelimit"), newRec.Header().Values("Grpc-Metadata-X-Ratelimit"),
		"gRPC metadata header x-ratelimit should match")

	// Trailer announcement headers
	assert.Equal(t, oldRec.Header().Values("Trailer"), newRec.Header().Values("Trailer"),
		"Trailer announcement headers should match")

	// Trailer values
	assert.Equal(t, oldRec.Header().Values("Grpc-Trailer-X-Stream-Id"), newRec.Header().Values("Grpc-Trailer-X-Stream-Id"),
		"trailer values should match")

	// Forward-response option header
	assert.Equal(t, "applied", oldRec.Header().Get("X-Test-Opt"))
	assert.Equal(t, "applied", newRec.Header().Get("X-Test-Opt"))

	// Body should be equivalent JSON
	var oldParsed, newParsed map[string]any
	require.NoError(t, json.Unmarshal(oldRec.Body.Bytes(), &oldParsed))
	require.NoError(t, json.Unmarshal(newRec.Body.Bytes(), &newParsed))
	assert.Equal(t, oldParsed, newParsed)
}

// failAfterNWriter wraps a writer and returns an error after n bytes have been written.
type failAfterNWriter struct {
	w       io.Writer
	limit   int
	written int
}

func (f *failAfterNWriter) Write(p []byte) (int, error) {
	if f.written+len(p) > f.limit {
		return 0, errors.New("simulated write failure")
	}
	n, err := f.w.Write(p)
	f.written += n
	return n, err
}

func TestForwarder_StreamErrorPanicsWithErrAbortHandler(t *testing.T) {
	list := &v1alpha1.ApplicationList{
		ResourceVersion: "1",
		Items: []v1alpha1.Application{
			{Name: "app1"},
		},
	}

	ctx := runtime.NewServerMetadataContext(context.Background(), runtime.ServerMetadata{})
	mux := runtime.NewServeMux()

	// Use an httptest.Server so the panic(http.ErrAbortHandler) is handled
	// the same way a real net/http server handles it: the connection is aborted.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Wrap the ResponseWriter so writes fail mid-stream after headers are sent.
		failing := &failAfterNWriter{w: w, limit: 1}
		fw := &failingResponseWriter{ResponseWriter: w, Writer: failing}
		forward_ApplicationService_List_0(ctx, mux, nil, fw, r, list)
	}))
	defer ts.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, ts.URL+"/api/v1/applications", http.NoBody)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		resp.Body.Close()
	}
	// The aborted connection surfaces as io.EOF or a connection reset
	// depending on the platform and timing; either way the request must
	// not complete cleanly.
	require.Error(t, err, "aborted response must not complete cleanly")
}

// failingResponseWriter delegates Header/WriteHeader to the real ResponseWriter
// but routes Write through a separate writer that can simulate failures.
type failingResponseWriter struct {
	http.ResponseWriter
	Writer io.Writer
}

func (f *failingResponseWriter) Write(p []byte) (int, error) {
	return f.Writer.Write(p)
}

func TestLogsForwarder_DownloadFileName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{"pod", "namespace=default&podName=guestbook-ui-5d7f9c8b4-x2k9p&container=guestbook-ui", "default-guestbook-ui-5d7f9c8b4-x2k9p-guestbook-ui.log"},
		{"deployment", "namespace=default&group=apps&kind=Deployment&resourceName=guestbook-ui&container=guestbook-ui", "default-guestbook-ui-guestbook-ui.log"},
		{"replicaset", "namespace=default&group=apps&kind=ReplicaSet&resourceName=guestbook-ui-5d7f9c8b4&container=guestbook-ui", "default-guestbook-ui-5d7f9c8b4-guestbook-ui.log"},
		{"invalid part", "namespace=default&podName=..%2Fetc&container=guestbook-ui", "log.log"},
		{"empty", "", "log.log"},
		{"dotted deployment", "namespace=default&group=apps&kind=Deployment&resourceName=guestbook.ui.v2&container=guestbook-ui", "default-guestbook.ui.v2-guestbook-ui.log"},
		{"long deployment", "namespace=default&group=apps&kind=Deployment&resourceName=" + strings.Repeat("a", 100) + "&container=guestbook-ui", "default-" + strings.Repeat("a", 100) + "-guestbook-ui.log"},
		{"253 char deployment", "namespace=default&group=apps&kind=Deployment&resourceName=" + strings.Repeat("a", 253) + "&container=guestbook-ui", "default-" + strings.Repeat("a", 253) + "-guestbook-ui.log"},
		{"254 char deployment", "namespace=default&group=apps&kind=Deployment&resourceName=" + strings.Repeat("a", 254) + "&container=guestbook-ui", "log.log"},
		{"deployment traversal", "namespace=default&group=apps&kind=Deployment&resourceName=..%2Fetc&container=guestbook-ui", "log.log"},
		{"deployment slash", "namespace=default&group=apps&kind=Deployment&resourceName=guestbook%2Fui&container=guestbook-ui", "log.log"},
		{"dotted pod", "namespace=default&podName=guestbook.ui&container=guestbook-ui", "log.log"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/applications/guestbook/logs?download=true&"+tt.query, http.NoBody)
			recv := func() (proto.Message, error) {
				return &LogEntry{Content: proto.String(""), Last: proto.Bool(true)}, nil
			}
			forward_ApplicationService_PodLogs_1(t.Context(), runtime.NewServeMux(), nil, rec, req, recv)
			assert.Equal(t, `attachment;filename="`+tt.expected+`"`, rec.Header().Get("Content-Disposition"))
		})
	}
}

// processedItem decodes item i of a processor result as argoproj/pkg serves it.
func processedItem(t *testing.T, res any, i int) map[string]any {
	t.Helper()
	b, err := json.Marshal(res)
	require.NoError(t, err)
	var decoded struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(b, &decoded))
	require.Greater(t, len(decoded.Items), i)
	return decoded.Items[i]
}

func encodeApp(t *testing.T, app *v1alpha1.Application, specs []appFieldSpec) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc := jsontext.NewEncoder(&buf, json.DefaultOptionsV1())
	require.NoError(t, encodeSelectedFields(enc, app, specs))
	return buf.Bytes()
}
