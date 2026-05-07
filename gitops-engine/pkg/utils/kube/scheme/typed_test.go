package scheme

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/structured-merge-diff/v6/typed"
)

// mockGVKParser is a GVKParser that delegates to a function for testing.
type mockGVKParser struct {
	typeFn func(gvk schema.GroupVersionKind) (*typed.ParseableType, error)
}

func (m *mockGVKParser) Type(gvk schema.GroupVersionKind) (*typed.ParseableType, error) {
	if m.typeFn != nil {
		return m.typeFn(gvk)
	}
	return nil, nil
}

func TestBuildSchemeModelNames(t *testing.T) {
	names := buildSchemeModelNames()

	// Verify well-known built-in types have correct model names
	tests := []struct {
		gvk      schema.GroupVersionKind
		expected string
	}{
		{
			gvk:      schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			expected: "io.k8s.api.apps.v1.Deployment",
		},
		{
			gvk:      schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			expected: "io.k8s.api.core.v1.Pod",
		},
		{
			gvk:      schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Service"},
			expected: "io.k8s.api.core.v1.Service",
		},
		{
			gvk:      schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"},
			expected: "io.k8s.api.batch.v1.Job",
		},
		{
			gvk:      schema.GroupVersionKind{Group: "networking.k8s.io", Version: "v1", Kind: "Ingress"},
			expected: "io.k8s.api.networking.v1.Ingress",
		},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			name, ok := names[tt.gvk]
			require.True(t, ok, "GVK %s should be in the scheme model names", tt.gvk)
			assert.Equal(t, tt.expected, name)
		})
	}
}

func TestResolveParseableType_WithNonConcreteParser(t *testing.T) {
	// Verify that built-in types resolve from the static parser
	// even when a non-concrete GVKParser is used (e.g. lazy v3 parser).
	fallbackCalled := false
	mock := &mockGVKParser{
		typeFn: func(gvk schema.GroupVersionKind) (*typed.ParseableType, error) {
			fallbackCalled = true
			return nil, nil
		},
	}

	gvk := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	pt, err := ResolveParseableType(gvk, mock)

	// Should resolve from static parser, not fall through to mock
	require.NoError(t, err)
	assert.False(t, fallbackCalled, "should not fall through to dynamic parser for built-in types")
	assert.NotNil(t, pt, "should resolve Deployment from static parser")
	assert.NotEqual(t, &typed.DeducedParseableType, pt, "should not return DeducedParseableType")
}

func TestResolveParseableType_CRDFallsThrough(t *testing.T) {
	// Verify that CRD types (not in the scheme) fall through to the dynamic parser.
	fallbackCalled := false
	deduced := typed.DeducedParseableType
	mock := &mockGVKParser{
		typeFn: func(gvk schema.GroupVersionKind) (*typed.ParseableType, error) {
			fallbackCalled = true
			return &deduced, nil
		},
	}

	gvk := schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"}
	pt, err := ResolveParseableType(gvk, mock)

	require.NoError(t, err)
	assert.True(t, fallbackCalled, "should fall through to dynamic parser for CRD types")
	assert.Equal(t, &deduced, pt)
}

func TestResolveParseableType_ErrorPropagated(t *testing.T) {
	// Verify that errors from the parser are propagated to callers.
	expectedErr := fmt.Errorf("failed to get schema: connection refused")
	mock := &mockGVKParser{
		typeFn: func(gvk schema.GroupVersionKind) (*typed.ParseableType, error) {
			return nil, expectedErr
		},
	}

	gvk := schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"}
	pt, err := ResolveParseableType(gvk, mock)

	assert.Nil(t, pt)
	assert.ErrorIs(t, err, expectedErr)
}
