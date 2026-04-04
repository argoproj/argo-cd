package application

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	"github.com/argoproj/argo-cd/v3/test"
)

func TestProcessApplicationListField_SyncOperation(t *testing.T) {
	list := v1alpha1.ApplicationList{
		Items: []v1alpha1.Application{{Operation: &v1alpha1.Operation{Sync: &v1alpha1.SyncOperation{
			Revision: "abc",
		}}}},
	}

	res, err := processApplicationListField(&list, map[string]any{"items.operation.sync": true}, false)
	require.NoError(t, err)
	resMap, ok := res.(map[string]any)
	require.True(t, ok)

	items, ok := resMap["items"].([]map[string]any)
	require.True(t, ok)
	item := test.ToMap(items[0])

	val, ok, err := unstructured.NestedString(item, "operation", "sync", "revision")
	require.NoError(t, err)
	require.True(t, ok)

	require.Equal(t, "abc", val)
}

func TestProcessApplicationListField_SyncOperationMissing(t *testing.T) {
	list := v1alpha1.ApplicationList{
		Items: []v1alpha1.Application{{Operation: nil}},
	}

	res, err := processApplicationListField(&list, map[string]any{"items.operation.sync": true}, false)
	require.NoError(t, err)
	resMap, ok := res.(map[string]any)
	require.True(t, ok)

	items, ok := resMap["items"].([]map[string]any)
	require.True(t, ok)
	item := test.ToMap(items[0])

	_, ok, err = unstructured.NestedString(item, "operation")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestStreamApplicationListJSON_WithFieldFilter(t *testing.T) {
	list := &v1alpha1.ApplicationList{
		ListMeta: metav1.ListMeta{ResourceVersion: "100"},
		Items: []v1alpha1.Application{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "app1", Namespace: "default"},
				Status:     v1alpha1.ApplicationStatus{Health: v1alpha1.AppHealthStatus{Status: "Healthy"}},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "app2", Namespace: "default"},
				Status:     v1alpha1.ApplicationStatus{Health: v1alpha1.AppHealthStatus{Status: "Degraded"}},
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
		ListMeta: metav1.ListMeta{ResourceVersion: "42"},
		Items: []v1alpha1.Application{
			{ObjectMeta: metav1.ObjectMeta{Name: "myapp", Namespace: "ns"}},
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
		ListMeta: metav1.ListMeta{ResourceVersion: "1"},
		Items:    []v1alpha1.Application{},
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

func TestStreamApplicationListJSON_MatchesProcessApplicationListField(t *testing.T) {
	list := &v1alpha1.ApplicationList{
		ListMeta: metav1.ListMeta{ResourceVersion: "99"},
		Items: []v1alpha1.Application{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "app1"},
				Operation:  &v1alpha1.Operation{Sync: &v1alpha1.SyncOperation{Revision: "abc"}},
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
		ListMeta: metav1.ListMeta{ResourceVersion: "200"},
		Items: []v1alpha1.Application{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "app1",
					Namespace: "ns1",
					Labels:    map[string]string{"env": "prod"},
				},
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
				ObjectMeta: metav1.ObjectMeta{Name: "app2", Namespace: "ns2"},
				Operation:  &v1alpha1.Operation{Sync: &v1alpha1.SyncOperation{Revision: "def456"}},
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
		TypeMeta: metav1.TypeMeta{Kind: "ApplicationList", APIVersion: "argoproj.io/v1alpha1"},
		ListMeta: metav1.ListMeta{ResourceVersion: "300"},
		Items: []v1alpha1.Application{
			{ObjectMeta: metav1.ObjectMeta{Name: "app1"}},
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
		ListMeta: metav1.ListMeta{ResourceVersion: "500"},
		Items: []v1alpha1.Application{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "full-app",
					Namespace:         "argocd",
					Annotations:       map[string]string{"note": "test"},
					Labels:            map[string]string{"team": "platform"},
					CreationTimestamp: metav1.Now(),
				},
				Spec: v1alpha1.ApplicationSpec{Project: "default"},
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

// TestStreamApplicationListJSON_MatchesFieldFilter_Exclude tests the exclude (negative) field selector.
func TestStreamApplicationListJSON_MatchesFieldFilter_Exclude(t *testing.T) {
	list := &v1alpha1.ApplicationList{
		ListMeta: metav1.ListMeta{ResourceVersion: "600"},
		Items: []v1alpha1.Application{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "app1", Namespace: "default"},
				Spec:       v1alpha1.ApplicationSpec{Project: "myproject"},
				Status: v1alpha1.ApplicationStatus{
					Health: v1alpha1.AppHealthStatus{Status: "Healthy"},
					Sync:   v1alpha1.SyncStatus{Status: "Synced"},
				},
			},
		},
	}

	// Exclude spec
	fields := map[string]any{"items.spec": true}

	batchResult, err := processApplicationListField(list, fields, true)
	require.NoError(t, err)
	batchJSON, err := json.Marshal(batchResult)
	require.NoError(t, err)

	var buf bytes.Buffer
	err = streamApplicationListJSON(&buf, list, fields, true)
	require.NoError(t, err)

	var batchParsed, streamParsed map[string]any
	require.NoError(t, json.Unmarshal(batchJSON, &batchParsed))
	require.NoError(t, json.Unmarshal(buf.Bytes(), &streamParsed))
	assert.Equal(t, batchParsed, streamParsed)
}

// TestForwarder_HeadersMatchForwardResponseMessage compares the HTTP headers produced by
// our streaming forwarder against runtime.ForwardResponseMessage (via UnaryForwarderWithFieldProcessor)
// to verify that gRPC metadata, trailers, Content-Type, and forward-response options are handled identically.
func TestForwarder_HeadersMatchForwardResponseMessage(t *testing.T) {
	list := &v1alpha1.ApplicationList{
		ListMeta: metav1.ListMeta{ResourceVersion: "700"},
		Items: []v1alpha1.Application{
			{ObjectMeta: metav1.ObjectMeta{Name: "app1"}},
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
