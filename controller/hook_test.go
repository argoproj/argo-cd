package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestIsHookOfType(t *testing.T) {
	tests := []struct {
		name     string
		hookType HookType
		annot    map[string]string
		expected bool
	}{
		{
			name:     "ArgoCD PreDelete hook",
			hookType: PreDeleteHookType,
			annot:    map[string]string{"argocd.argoproj.io/hook": "PreDelete"},
			expected: true,
		},
		{
			name:     "Helm PreDelete hook",
			hookType: PreDeleteHookType,
			annot:    map[string]string{"helm.sh/hook": "pre-delete"},
			expected: true,
		},
		{
			name:     "ArgoCD PostDelete hook",
			hookType: PostDeleteHookType,
			annot:    map[string]string{"argocd.argoproj.io/hook": "PostDelete"},
			expected: true,
		},
		{
			name:     "Helm PostDelete hook",
			hookType: PostDeleteHookType,
			annot:    map[string]string{"helm.sh/hook": "post-delete"},
			expected: true,
		},
		{
			name:     "Not a hook",
			hookType: PreDeleteHookType,
			annot:    map[string]string{"some-other": "annotation"},
			expected: false,
		},
		{
			name:     "Wrong hook type",
			hookType: PreDeleteHookType,
			annot:    map[string]string{"argocd.argoproj.io/hook": "PostDelete"},
			expected: false,
		},
		{
			name:     "Nil annotations",
			hookType: PreDeleteHookType,
			annot:    nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := &unstructured.Unstructured{}
			obj.SetAnnotations(tt.annot)
			result := isHookOfType(obj, tt.hookType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsHook(t *testing.T) {
	tests := []struct {
		name     string
		annot    map[string]string
		expected bool
	}{
		{
			name:     "ArgoCD PreDelete hook",
			annot:    map[string]string{"argocd.argoproj.io/hook": "PreDelete"},
			expected: true,
		},
		{
			name:     "ArgoCD PostDelete hook",
			annot:    map[string]string{"argocd.argoproj.io/hook": "PostDelete"},
			expected: true,
		},
		{
			name:     "ArgoCD PreSync hook",
			annot:    map[string]string{"argocd.argoproj.io/hook": "PreSync"},
			expected: true,
		},
		{
			name:     "Not a hook",
			annot:    map[string]string{"some-other": "annotation"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := &unstructured.Unstructured{}
			obj.SetAnnotations(tt.annot)
			result := isHook(obj)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsPreDeleteHook(t *testing.T) {
	tests := []struct {
		name     string
		annot    map[string]string
		expected bool
	}{
		{
			name:     "ArgoCD PreDelete hook",
			annot:    map[string]string{"argocd.argoproj.io/hook": "PreDelete"},
			expected: true,
		},
		{
			name:     "Helm PreDelete hook",
			annot:    map[string]string{"helm.sh/hook": "pre-delete"},
			expected: true,
		},
		{
			name:     "ArgoCD PostDelete hook",
			annot:    map[string]string{"argocd.argoproj.io/hook": "PostDelete"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := &unstructured.Unstructured{}
			obj.SetAnnotations(tt.annot)
			result := isPreDeleteHook(obj)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsPostDeleteHook(t *testing.T) {
	tests := []struct {
		name     string
		annot    map[string]string
		expected bool
	}{
		{
			name:     "ArgoCD PostDelete hook",
			annot:    map[string]string{"argocd.argoproj.io/hook": "PostDelete"},
			expected: true,
		},
		{
			name:     "Helm PostDelete hook",
			annot:    map[string]string{"helm.sh/hook": "post-delete"},
			expected: true,
		},
		{
			name:     "ArgoCD PreDelete hook",
			annot:    map[string]string{"argocd.argoproj.io/hook": "PreDelete"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := &unstructured.Unstructured{}
			obj.SetAnnotations(tt.annot)
			result := isPostDeleteHook(obj)
			assert.Equal(t, tt.expected, result)
		})
	}
}
