package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestIsHookOfType(t *testing.T) {
	tests := []struct {
		name     string
		hookType string
		annot    map[string]string
		expected bool
	}{
		{
			name:     "ArgoCD PreDelete hook",
			hookType: preDeleteHook,
			annot:    map[string]string{"argocd.argoproj.io/hook": preDeleteHook},
			expected: true,
		},
		{
			name:     "Helm PreDelete hook",
			hookType: preDeleteHook,
			annot:    map[string]string{"helm.sh/hook": "pre-delete"},
			expected: true,
		},
		{
			name:     "ArgoCD PostDelete hook",
			hookType: postDeleteHook,
			annot:    map[string]string{"argocd.argoproj.io/hook": postDeleteHook},
			expected: true,
		},
		{
			name:     "Helm PostDelete hook",
			hookType: postDeleteHook,
			annot:    map[string]string{"helm.sh/hook": "post-delete"},
			expected: true,
		},
		{
			name:     "Not a hook",
			hookType: preDeleteHook,
			annot:    map[string]string{"some-other": "annotation"},
			expected: false,
		},
		{
			name:     "Wrong hook type",
			hookType: preDeleteHook,
			annot:    map[string]string{"argocd.argoproj.io/hook": postDeleteHook},
			expected: false,
		},
		{
			name:     "Nil annotations",
			hookType: preDeleteHook,
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
			name:     "PreDelete hook",
			annot:    map[string]string{"argocd.argoproj.io/hook": preDeleteHook},
			expected: true,
		},
		{
			name:     "PostDelete hook",
			annot:    map[string]string{"argocd.argoproj.io/hook": postDeleteHook},
			expected: true,
		},
		{
			name:     "PreSync hook",
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
			annot:    map[string]string{"argocd.argoproj.io/hook": preDeleteHook},
			expected: true,
		},
		{
			name:     "Helm PreDelete hook",
			annot:    map[string]string{"helm.sh/hook": "pre-delete"},
			expected: true,
		},
		{
			name:     "Not a PreDelete hook",
			annot:    map[string]string{"argocd.argoproj.io/hook": postDeleteHook},
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
			annot:    map[string]string{"argocd.argoproj.io/hook": postDeleteHook},
			expected: true,
		},
		{
			name:     "Helm PostDelete hook",
			annot:    map[string]string{"helm.sh/hook": "post-delete"},
			expected: true,
		},
		{
			name:     "Not a PostDelete hook",
			annot:    map[string]string{"argocd.argoproj.io/hook": preDeleteHook},
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
