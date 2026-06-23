package controller

import (
	"testing"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
		{
			name:     "Helm PreDelete & PreDelete hook",
			annot:    map[string]string{"helm.sh/hook": "pre-delete,post-delete"},
			expected: true,
		},
		{
			name:     "ArgoCD PostDelete & PreDelete hook",
			annot:    map[string]string{"argocd.argoproj.io/hook": "PostDelete,PreDelete"},
			expected: true,
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
		{
			name:     "ArgoCD PostDelete & PreDelete hook",
			annot:    map[string]string{"argocd.argoproj.io/hook": "PostDelete,PreDelete"},
			expected: true,
		},
		{
			name:     "Helm PostDelete & PreDelete hook",
			annot:    map[string]string{"helm.sh/hook": "post-delete,pre-delete"},
			expected: true,
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

// TestPartitionTargetObjsForSync covers partitionTargetObjsForSync in state.go.
func TestPartitionTargetObjsForSync(t *testing.T) {
	newObj := func(name string, annot map[string]string) *unstructured.Unstructured {
		u := &unstructured.Unstructured{}
		u.SetName(name)
		u.SetAnnotations(annot)
		return u
	}

	tests := []struct {
		name           string
		in             []*unstructured.Unstructured
		wantNames      []string
		wantPreDelete  bool
		wantPostDelete bool
	}{
		{
			name: "PostSync with PreDelete and PostDelete in same annotation stays in sync set",
			in: []*unstructured.Unstructured{
				newObj("combined", map[string]string{"argocd.argoproj.io/hook": "PostSync,PreDelete,PostDelete"}),
			},
			wantNames:      []string{"combined"},
			wantPreDelete:  true,
			wantPostDelete: true,
		},
		{
			name: "PreDelete-only manifest excluded from sync",
			in: []*unstructured.Unstructured{
				newObj("pre-del", map[string]string{"argocd.argoproj.io/hook": "PreDelete"}),
			},
			wantNames:      nil,
			wantPreDelete:  true,
			wantPostDelete: false,
		},
		{
			name: "PostDelete-only manifest excluded from sync",
			in: []*unstructured.Unstructured{
				newObj("post-del", map[string]string{"argocd.argoproj.io/hook": "PostDelete"}),
			},
			wantNames:      nil,
			wantPreDelete:  false,
			wantPostDelete: true,
		},
		{
			name: "Helm pre-delete only excluded from sync",
			in: []*unstructured.Unstructured{
				newObj("helm-pre-del", map[string]string{"helm.sh/hook": "pre-delete"}),
			},
			wantNames:      nil,
			wantPreDelete:  true,
			wantPostDelete: false,
		},
		{
			name: "Helm pre-install with pre-delete stays in sync (sync-phase hook wins)",
			in: []*unstructured.Unstructured{
				newObj("helm-mixed", map[string]string{"helm.sh/hook": "pre-install,pre-delete"}),
			},
			wantNames:      []string{"helm-mixed"},
			wantPreDelete:  true,
			wantPostDelete: false,
		},
		{
			name: "Non-hook resource unchanged",
			in: []*unstructured.Unstructured{
				newObj("pod", map[string]string{"app": "x"}),
			},
			wantNames:      []string{"pod"},
			wantPreDelete:  false,
			wantPostDelete: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, hasPre, hasPost := partitionTargetObjsForSync(tt.in)
			var names []string
			for _, o := range got {
				names = append(names, o.GetName())
			}
			assert.Equal(t, tt.wantNames, names)
			assert.Equal(t, tt.wantPreDelete, hasPre, "hasPreDeleteHooks")
			assert.Equal(t, tt.wantPostDelete, hasPost, "hasPostDeleteHooks")
		})
	}
}

func TestMultiHookOfType(t *testing.T) {
	tests := []struct {
		name     string
		hookType []HookType
		annot    map[string]string
		expected bool
	}{
		{
			name:     "helm PreDelete &  PostDelete hook",
			hookType: []HookType{PreDeleteHookType, PostDeleteHookType},
			annot:    map[string]string{"helm.sh/hook": "pre-delete,post-delete"},
			expected: true,
		},

		{
			name:     "ArgoCD PreDelete &  PostDelete hook",
			hookType: []HookType{PreDeleteHookType, PostDeleteHookType},
			annot:    map[string]string{"argocd.argoproj.io/hook": "PreDelete,PostDelete"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := &unstructured.Unstructured{}
			obj.SetAnnotations(tt.annot)

			for _, hookType := range tt.hookType {
				result := isHookOfType(obj, hookType)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestExecuteHooksAlreadyExistsLogic(t *testing.T) {
	newObj := func(name string, annot map[string]string) *unstructured.Unstructured {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"})
		obj.SetName(name)
		obj.SetNamespace("default")
		obj.SetAnnotations(annot)
		return obj
	}

	tests := []struct {
		name          string
		hookType      []HookType
		targetAnnot   map[string]string
		liveAnnot     map[string]string // nil -> object doesn't exist in cluster
		expectCreated bool
	}{
		//  PRE DELETE TESTS
		{
			name:          "PreDelete (argocd): Not in cluster - should be created",
			hookType:      []HookType{PreDeleteHookType},
			targetAnnot:   map[string]string{"argocd.argoproj.io/hook": "PreDelete"},
			liveAnnot:     nil,
			expectCreated: true,
		},
		{
			name:          "PreDelete (helm): Not in cluster - should be created",
			hookType:      []HookType{PreDeleteHookType},
			targetAnnot:   map[string]string{"helm.sh/hook": "pre-delete"},
			liveAnnot:     nil,
			expectCreated: true,
		},
		{
			name:          "PreDelete (argocd): Already exists - should be skipped",
			hookType:      []HookType{PreDeleteHookType},
			targetAnnot:   map[string]string{"argocd.argoproj.io/hook": "PreDelete"},
			liveAnnot:     map[string]string{"argocd.argoproj.io/hook": "PreDelete"},
			expectCreated: false,
		},
		{
			name:          "PreDelete (argocd): Already exists - should be skipped",
			hookType:      []HookType{PreDeleteHookType},
			targetAnnot:   map[string]string{"helm.sh/hook": "pre-delete"},
			liveAnnot:     map[string]string{"helm.sh/hook": "pre-delete"},
			expectCreated: false,
		},
		{
			name:          "PreDelete (helm+argocd): One of two already exists - should be skipped",
			hookType:      []HookType{PreDeleteHookType},
			targetAnnot:   map[string]string{"helm.sh/hook": "pre-delete", "argocd.argoproj.io/hook": "PreDelete"},
			liveAnnot:     map[string]string{"helm.sh/hook": "pre-delete"},
			expectCreated: false,
		},
		{
			name:          "PreDelete (helm+argocd): One of two already exists - should be skipped",
			hookType:      []HookType{PreDeleteHookType},
			targetAnnot:   map[string]string{"helm.sh/hook": "pre-delete", "argocd.argoproj.io/hook": "PreDelete"},
			liveAnnot:     map[string]string{"argocd.argoproj.io/hook": "PreDelete"},
			expectCreated: false,
		},
		//  POST DELETE TESTS
		{
			name:          "PostDelete (argocd): Not in cluster - should be created",
			hookType:      []HookType{PostDeleteHookType},
			targetAnnot:   map[string]string{"argocd.argoproj.io/hook": "PostDelete"},
			liveAnnot:     nil,
			expectCreated: true,
		},
		{
			name:          "PostDelete (helm): Not in cluster - should be created",
			hookType:      []HookType{PostDeleteHookType},
			targetAnnot:   map[string]string{"helm.sh/hook": "post-delete"},
			liveAnnot:     nil,
			expectCreated: true,
		},
		{
			name:          "PostDelete (argocd): Already exists - should be skipped",
			hookType:      []HookType{PostDeleteHookType},
			targetAnnot:   map[string]string{"argocd.argoproj.io/hook": "PostDelete"},
			liveAnnot:     map[string]string{"argocd.argoproj.io/hook": "PostDelete"},
			expectCreated: false,
		},
		{
			name:          "PostDelete (helm): Already exists - should be skipped",
			hookType:      []HookType{PostDeleteHookType},
			targetAnnot:   map[string]string{"helm.sh/hook": "post-delete"},
			liveAnnot:     map[string]string{"helm.sh/hook": "post-delete"},
			expectCreated: false,
		},
		{
			name:          "PostDelete (helm+argocd): Already exists - should be skipped",
			hookType:      []HookType{PostDeleteHookType},
			targetAnnot:   map[string]string{"helm.sh/hook": "post-delete", "argocd.argoproj.io/hook": "PostDelete"},
			liveAnnot:     map[string]string{"helm.sh/hook": "post-delete", "argocd.argoproj.io/hook": "PostDelete"},
			expectCreated: false,
		},
		{
			name:          "PostDelete (helm+argocd): One of two already exists - should be skipped",
			hookType:      []HookType{PostDeleteHookType},
			targetAnnot:   map[string]string{"helm.sh/hook": "post-delete", "argocd.argoproj.io/hook": "PostDelete"},
			liveAnnot:     map[string]string{"helm.sh/hook": "post-delete"},
			expectCreated: false,
		},
		{
			name:          "PostDelete (helm+argocd): One of two already exists - should be skipped",
			hookType:      []HookType{PostDeleteHookType},
			targetAnnot:   map[string]string{"helm.sh/hook": "post-delete", "argocd.argoproj.io/hook": "PostDelete"},
			liveAnnot:     map[string]string{"argocd.argoproj.io/hook": "PostDelete"},
			expectCreated: false,
		},
		//  MULTI HOOK TESTS - SKIP LOGIC
		{
			name:          "Multi-hook (argocd): Target is (Pre,Post), Cluster has (Pre,Post) - should be skipped",
			hookType:      []HookType{PreDeleteHookType, PostDeleteHookType},
			targetAnnot:   map[string]string{"argocd.argoproj.io/hook": "PreDelete,PostDelete"},
			liveAnnot:     map[string]string{"argocd.argoproj.io/hook": "PreDelete,PostDelete"},
			expectCreated: false,
		},
		{
			name:          "Multi-hook (helm): Target is (Pre,Post), Cluster has (Pre,Post) - should be skipped",
			hookType:      []HookType{PreDeleteHookType, PostDeleteHookType},
			targetAnnot:   map[string]string{"helm.sh/hook": "post-delete,pre-delete"},
			liveAnnot:     map[string]string{"helm.sh/hook": "post-delete,pre-delete"},
			expectCreated: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetObj := newObj("my-hook", tt.targetAnnot)
			targetKey := kube.GetResourceKey(targetObj)

			liveObjs := make(map[kube.ResourceKey]*unstructured.Unstructured)
			if tt.liveAnnot != nil {
				liveObjs[targetKey] = newObj("my-hook", tt.liveAnnot)
			}

			runningHooks := map[kube.ResourceKey]*unstructured.Unstructured{}
			for key, obj := range liveObjs {
				for _, hookType := range tt.hookType {
					if isHookOfType(obj, hookType) {
						runningHooks[key] = obj
					}
				}
			}

			expectedHooksToCreate := map[kube.ResourceKey]*unstructured.Unstructured{}
			targets := []*unstructured.Unstructured{targetObj}

			for _, obj := range targets {
				for _, hookType := range tt.hookType {
					if !isHookOfType(obj, hookType) {
						continue
					}
				}

				objKey := kube.GetResourceKey(obj)
				if _, alreadyExists := runningHooks[objKey]; !alreadyExists {
					expectedHooksToCreate[objKey] = obj
				}
			}

			if tt.expectCreated {
				assert.NotEmpty(t, expectedHooksToCreate, "Expected hook to be marked for creation")
			} else {
				assert.Empty(t, expectedHooksToCreate, "Expected hook to be skipped (already exists)")
			}
		})
	}
}
