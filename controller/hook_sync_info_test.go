package controller

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	cdcommon "github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func newSyncInfoHook(apiVersion, kind, inject string) *unstructured.Unstructured {
	hook := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata":   map[string]any{"name": "verify"},
	}}
	if inject != "" {
		hook.SetAnnotations(map[string]string{cdcommon.AnnotationInjectSyncInfo: inject})
	}
	return hook
}

func podTemplateAnnotations(t *testing.T, job *unstructured.Unstructured) map[string]string {
	t.Helper()
	annotations, _, err := unstructured.NestedStringMap(job.Object, "spec", "template", "metadata", "annotations")
	require.NoError(t, err)
	return annotations
}

func TestInjectSyncInfoIntoHooks_Job(t *testing.T) {
	job := newSyncInfoHook("batch/v1", "Job", "revision")
	require.NoError(t, unstructured.SetNestedStringMap(job.Object, map[string]string{"existing": "kept"}, "spec", "template", "metadata", "annotations"))

	hooks, err := injectSyncInfoIntoHooks([]*unstructured.Unstructured{job}, "sha-A", nil, nil, false)
	require.NoError(t, err)
	assert.Equal(t, "sha-A", hooks[0].GetAnnotations()[cdcommon.AnnotationSyncRevision])
	podAnnotations := podTemplateAnnotations(t, hooks[0])
	assert.Equal(t, "sha-A", podAnnotations[cdcommon.AnnotationSyncRevision])
	assert.Equal(t, "kept", podAnnotations["existing"])
	assert.NotContains(t, job.GetAnnotations(), cdcommon.AnnotationSyncRevision, "desired hook must not be mutated")
	assert.NotContains(t, podTemplateAnnotations(t, job), cdcommon.AnnotationSyncRevision)

	// A later operation re-evaluating the same desired hook gets its own revision.
	later, err := injectSyncInfoIntoHooks([]*unstructured.Unstructured{job}, "sha-B", nil, nil, false)
	require.NoError(t, err)
	assert.Equal(t, "sha-B", podTemplateAnnotations(t, later[0])[cdcommon.AnnotationSyncRevision])
	assert.Equal(t, "sha-A", podAnnotations[cdcommon.AnnotationSyncRevision])
}

func TestInjectSyncInfoIntoHooks_AnyKind(t *testing.T) {
	pod := newSyncInfoHook("v1", "Pod", "revision")
	workflow := newSyncInfoHook("argoproj.io/v1alpha1", "Workflow", " revision ,")

	hooks, err := injectSyncInfoIntoHooks([]*unstructured.Unstructured{pod, workflow}, "sha-A", nil, nil, false)
	require.NoError(t, err)
	assert.Equal(t, "sha-A", hooks[0].GetAnnotations()[cdcommon.AnnotationSyncRevision])
	assert.Equal(t, "sha-A", hooks[1].GetAnnotations()[cdcommon.AnnotationSyncRevision])
	_, found, _ := unstructured.NestedMap(hooks[1].Object, "spec")
	assert.False(t, found, "only Jobs get a Pod template annotation")
}

func TestInjectSyncInfoIntoHooks_MultiSource(t *testing.T) {
	job := newSyncInfoHook("batch/v1", "Job", "revision")
	sources := []v1alpha1.ApplicationSource{{Name: "app"}, {}, {Name: "values"}}

	hooks, err := injectSyncInfoIntoHooks([]*unstructured.Unstructured{job}, "", []string{"sha-1", "sha-2", "sha-3"}, sources, true)
	require.NoError(t, err)
	podAnnotations := podTemplateAnnotations(t, hooks[0])
	assert.Equal(t, map[string]string{
		cdcommon.AnnotationSyncRevision + "-1":      "sha-1",
		cdcommon.AnnotationSyncRevision + "-2":      "sha-2",
		cdcommon.AnnotationSyncRevision + "-3":      "sha-3",
		cdcommon.AnnotationSyncRevision + "-app":    "sha-1",
		cdcommon.AnnotationSyncRevision + "-values": "sha-3",
	}, podAnnotations)
	assert.NotContains(t, podAnnotations, cdcommon.AnnotationSyncRevision, "no single value for multiple sources")
}

// Named keys follow the source when spec.sources is reordered; positional keys do not.
func TestInjectSyncInfoIntoHooks_MultiSourceReordered(t *testing.T) {
	job := newSyncInfoHook("batch/v1", "Job", "revision")
	sources := []v1alpha1.ApplicationSource{{Name: "values"}, {Name: "app"}}

	hooks, err := injectSyncInfoIntoHooks([]*unstructured.Unstructured{job}, "", []string{"sha-values", "sha-app"}, sources, true)
	require.NoError(t, err)
	annotations := hooks[0].GetAnnotations()
	assert.Equal(t, "sha-app", annotations[cdcommon.AnnotationSyncRevision+"-app"])
	assert.Equal(t, "sha-values", annotations[cdcommon.AnnotationSyncRevision+"-values"])
	assert.Equal(t, "sha-values", annotations[cdcommon.AnnotationSyncRevision+"-1"])
}

func TestInjectSyncInfoIntoHooks_MultiSourceSkipsUnusableNames(t *testing.T) {
	job := newSyncInfoHook("batch/v1", "Job", "revision")
	sources := []v1alpha1.ApplicationSource{
		{Name: "dup"},
		{Name: "dup"},
		{Name: "has space"},
		{Name: strings.Repeat("a", 50)},
		{Name: "1"},
	}

	hooks, err := injectSyncInfoIntoHooks([]*unstructured.Unstructured{job}, "", []string{"a", "b", "c", "d", "e"}, sources, true)
	require.NoError(t, err)
	annotations := hooks[0].GetAnnotations()
	for i, want := range []string{"a", "b", "c", "d", "e"} {
		assert.Equal(t, want, annotations[cdcommon.AnnotationSyncRevision+"-"+strconv.Itoa(i+1)])
	}
	assert.Len(t, annotations, 6, "only the opt-in and positional keys; unusable names are skipped")
}

func TestInjectSyncInfoIntoHooks_NotOptedIn(t *testing.T) {
	plain := newSyncInfoHook("batch/v1", "Job", "")
	empty := newSyncInfoHook("batch/v1", "Job", " ")

	hooks, err := injectSyncInfoIntoHooks([]*unstructured.Unstructured{plain, empty}, "", nil, nil, false)
	require.NoError(t, err)
	assert.Same(t, plain, hooks[0])
	assert.Same(t, empty, hooks[1])
}

func TestInjectSyncInfoIntoHooks_Errors(t *testing.T) {
	job := newSyncInfoHook("batch/v1", "Job", "revision")

	_, err := injectSyncInfoIntoHooks([]*unstructured.Unstructured{job}, "", nil, nil, false)
	require.ErrorContains(t, err, "no resolved revision")

	_, err = injectSyncInfoIntoHooks([]*unstructured.Unstructured{job}, "", []string{"sha-0", ""}, nil, true)
	require.ErrorContains(t, err, "no resolved revision for source 2")

	_, err = injectSyncInfoIntoHooks([]*unstructured.Unstructured{newSyncInfoHook("batch/v1", "Job", "revision,true")}, "sha-A", nil, nil, false)
	require.ErrorContains(t, err, `unsupported argocd.argoproj.io/inject-sync-info field "true"`)
}

func TestInjectSyncInfoIntoHooks_HookKinds(t *testing.T) {
	deployment := newSyncInfoHook("apps/v1", "Deployment", "revision")
	require.NoError(t, unstructured.SetNestedField(deployment.Object, map[string]any{"metadata": map[string]any{}}, "spec", "template"))
	tests := []struct {
		name         string
		hook         *unstructured.Unstructured
		podTemplated bool
	}{
		{"Job", newSyncInfoHook("batch/v1", "Job", "revision"), true},
		{"Pod", newSyncInfoHook("v1", "Pod", "revision"), false},
		{"Workflow", newSyncInfoHook("argoproj.io/v1alpha1", "Workflow", "revision"), false},
		{"Deployment", deployment, false},
		{"Job from another group", newSyncInfoHook("example.com/v1", "Job", "revision"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hooks, err := injectSyncInfoIntoHooks([]*unstructured.Unstructured{tt.hook}, "sha-A", nil, nil, false)
			require.NoError(t, err)
			assert.Equal(t, "sha-A", hooks[0].GetAnnotations()[cdcommon.AnnotationSyncRevision])
			podAnnotations, _, err := unstructured.NestedStringMap(hooks[0].Object, "spec", "template", "metadata", "annotations")
			require.NoError(t, err)
			if tt.podTemplated {
				assert.Equal(t, "sha-A", podAnnotations[cdcommon.AnnotationSyncRevision])
			} else {
				assert.NotContains(t, podAnnotations, cdcommon.AnnotationSyncRevision)
			}
		})
	}
}

func TestInjectSyncInfoIntoHooks_OptInValues(t *testing.T) {
	tests := []struct {
		value    string
		injected bool
		err      string
	}{
		{value: "", injected: false},
		{value: " ", injected: false},
		{value: ",", injected: false},
		{value: "revision", injected: true},
		{value: " revision ,", injected: true},
		{value: "revision,revision", injected: true},
		{value: "foo", err: `unsupported argocd.argoproj.io/inject-sync-info field "foo"`},
		{value: "revision,foo", err: `unsupported argocd.argoproj.io/inject-sync-info field "foo"`},
		{value: "Revision", err: `unsupported argocd.argoproj.io/inject-sync-info field "Revision"`},
		{value: "true", err: `unsupported argocd.argoproj.io/inject-sync-info field "true"`},
	}
	for _, tt := range tests {
		t.Run(strconv.Quote(tt.value), func(t *testing.T) {
			hook := newSyncInfoHook("v1", "Pod", "")
			hook.SetAnnotations(map[string]string{cdcommon.AnnotationInjectSyncInfo: tt.value})
			hooks, err := injectSyncInfoIntoHooks([]*unstructured.Unstructured{hook}, "sha-A", nil, nil, false)
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			if tt.injected {
				assert.Equal(t, "sha-A", hooks[0].GetAnnotations()[cdcommon.AnnotationSyncRevision])
			} else {
				assert.Same(t, hook, hooks[0])
			}
		})
	}
}

func TestSyncRevisionAnnotations_SourceNames(t *testing.T) {
	key := func(suffix string) string { return cdcommon.AnnotationSyncRevision + "-" + suffix }
	tests := []struct {
		name      string
		sources   []v1alpha1.ApplicationSource
		revisions []string
		want      map[string]string
		err       string
	}{
		{
			name:      "one source",
			sources:   []v1alpha1.ApplicationSource{{Name: "app"}},
			revisions: []string{"a"},
			want:      map[string]string{key("1"): "a", key("app"): "a"},
		},
		{
			name:      "all unnamed",
			sources:   []v1alpha1.ApplicationSource{{}, {}, {}},
			revisions: []string{"a", "b", "c"},
			want:      map[string]string{key("1"): "a", key("2"): "b", key("3"): "c"},
		},
		{
			name:      "all named",
			sources:   []v1alpha1.ApplicationSource{{Name: "app"}, {Name: "values"}},
			revisions: []string{"a", "b"},
			want:      map[string]string{key("1"): "a", key("2"): "b", key("app"): "a", key("values"): "b"},
		},
		{
			name:      "duplicate names skipped, unique kept",
			sources:   []v1alpha1.ApplicationSource{{Name: "x"}, {Name: "x"}, {Name: "y"}},
			revisions: []string{"a", "b", "c"},
			want:      map[string]string{key("1"): "a", key("2"): "b", key("3"): "c", key("y"): "c"},
		},
		{
			name:      "numeric names skipped",
			sources:   []v1alpha1.ApplicationSource{{Name: "2"}, {Name: "01"}},
			revisions: []string{"a", "b"},
			want:      map[string]string{key("1"): "a", key("2"): "b"},
		},
		{
			name:      "invalid key characters skipped",
			sources:   []v1alpha1.ApplicationSource{{Name: "has space"}, {Name: "a/b"}, {Name: "a@b"}, {Name: "trailing-"}},
			revisions: []string{"a", "b", "c", "d"},
			want:      map[string]string{key("1"): "a", key("2"): "b", key("3"): "c", key("4"): "d"},
		},
		{
			name:      "dotted and underscored names kept",
			sources:   []v1alpha1.ApplicationSource{{Name: "helm.values"}, {Name: "base_app"}},
			revisions: []string{"a", "b"},
			want:      map[string]string{key("1"): "a", key("2"): "b", key("helm.values"): "a", key("base_app"): "b"},
		},
		{
			name:      "name length boundary",
			sources:   []v1alpha1.ApplicationSource{{Name: strings.Repeat("a", 49)}, {Name: strings.Repeat("b", 50)}},
			revisions: []string{"a", "b"},
			want:      map[string]string{key("1"): "a", key("2"): "b", key(strings.Repeat("a", 49)): "a"},
		},
		{
			name:      "more revisions than sources",
			sources:   []v1alpha1.ApplicationSource{{Name: "app"}},
			revisions: []string{"a", "b"},
			want:      map[string]string{key("1"): "a", key("2"): "b", key("app"): "a"},
		},
		{
			name:      "case-only differences are distinct keys",
			sources:   []v1alpha1.ApplicationSource{{Name: "App"}, {Name: "app"}},
			revisions: []string{"a", "b"},
			want:      map[string]string{key("1"): "a", key("2"): "b", key("App"): "a", key("app"): "b"},
		},
		{
			name: "long names differing only after the limit are both skipped, never truncated into one key",
			sources: []v1alpha1.ApplicationSource{
				{Name: strings.Repeat("x", 49) + "-staging"},
				{Name: strings.Repeat("x", 49) + "-production"},
			},
			revisions: []string{"a", "b"},
			want:      map[string]string{key("1"): "a", key("2"): "b"},
		},
		{
			name:      "names differing by an invalid character are not sanitized into one key",
			sources:   []v1alpha1.ApplicationSource{{Name: "my app"}, {Name: "my-app"}, {Name: "my_app"}},
			revisions: []string{"a", "b", "c"},
			want:      map[string]string{key("1"): "a", key("2"): "b", key("3"): "c", key("my-app"): "b", key("my_app"): "c"},
		},
		{
			name:      "non-ASCII and control characters skipped",
			sources:   []v1alpha1.ApplicationSource{{Name: "café"}, {Name: "app🚀"}, {Name: "a\tb"}, {Name: "a\nb"}},
			revisions: []string{"a", "b", "c", "d"},
			want:      map[string]string{key("1"): "a", key("2"): "b", key("3"): "c", key("4"): "d"},
		},
		{
			name:      "must end alphanumeric; may start with - _ . (key name part starts with sync-revision-)",
			sources:   []v1alpha1.ApplicationSource{{Name: ".hidden"}, {Name: "_x"}, {Name: "x."}, {Name: "-x"}, {Name: "-1"}},
			revisions: []string{"a", "b", "c", "d", "e"},
			want: map[string]string{
				key("1"): "a", key("2"): "b", key("3"): "c", key("4"): "d", key("5"): "e",
				key(".hidden"): "a", key("_x"): "b", key("-x"): "d",
			},
		},
		{
			name:      "number-like names: pure integers skipped, others kept",
			sources:   []v1alpha1.ApplicationSource{{Name: "0"}, {Name: "+1"}, {Name: "1e3"}, {Name: "v1"}, {Name: "2a"}, {Name: "18446744073709551616"}},
			revisions: []string{"a", "b", "c", "d", "e", "f"},
			want: map[string]string{
				key("1"): "a", key("2"): "b", key("3"): "c", key("4"): "d", key("5"): "e", key("6"): "f",
				key("1e3"): "c", key("v1"): "d", key("2a"): "e",
			},
		},
		{
			name:      "name equal to another source's position is skipped",
			sources:   []v1alpha1.ApplicationSource{{Name: "2"}, {Name: "x"}},
			revisions: []string{"a", "b"},
			want:      map[string]string{key("1"): "a", key("2"): "b", key("x"): "b"},
		},
		{
			name:      "same revision under two names",
			sources:   []v1alpha1.ApplicationSource{{Name: "app"}, {Name: "values"}},
			revisions: []string{"same", "same"},
			want:      map[string]string{key("1"): "same", key("2"): "same", key("app"): "same", key("values"): "same"},
		},
		{
			name:      "unresolved revision names the 1-based position",
			sources:   []v1alpha1.ApplicationSource{{}, {}, {}},
			revisions: []string{"a", "b", ""},
			err:       "no resolved revision for source 3",
		},
		{
			name: "no revisions",
			err:  "no resolved revisions",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := syncRevisionAnnotations("", tt.revisions, tt.sources, true)
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestInjectSyncInfoIntoHooks_PreservesAndOverwrites(t *testing.T) {
	job := newSyncInfoHook("batch/v1", "Job", "revision")
	annotations := job.GetAnnotations()
	annotations["team"] = "x"
	annotations[cdcommon.AnnotationSyncRevision] = "user-set"
	job.SetAnnotations(annotations)
	require.NoError(t, unstructured.SetNestedStringMap(job.Object, map[string]string{
		"pod": "kept", cdcommon.AnnotationSyncRevision: "user-set",
	}, "spec", "template", "metadata", "annotations"))
	plain := newSyncInfoHook("v1", "Pod", "")

	hooks, err := injectSyncInfoIntoHooks([]*unstructured.Unstructured{plain, job}, "sha-A", nil, nil, false)
	require.NoError(t, err)
	assert.Same(t, plain, hooks[0])
	assert.Equal(t, "x", hooks[1].GetAnnotations()["team"])
	assert.Equal(t, "sha-A", hooks[1].GetAnnotations()[cdcommon.AnnotationSyncRevision])
	podAnnotations := podTemplateAnnotations(t, hooks[1])
	assert.Equal(t, "kept", podAnnotations["pod"])
	assert.Equal(t, "sha-A", podAnnotations[cdcommon.AnnotationSyncRevision])
	assert.Equal(t, "user-set", job.GetAnnotations()[cdcommon.AnnotationSyncRevision], "desired hook not mutated")
}

// Explicit nulls in a Job's Pod template are valid manifests and must not fail the sync.
func TestInjectSyncInfoIntoHooks_JobTemplateNulls(t *testing.T) {
	tests := map[string]map[string]any{
		"metadata: null":        {"metadata": nil},
		"annotations: null":     {"metadata": map[string]any{"annotations": nil}},
		"null annotation value": {"metadata": map[string]any{"annotations": map[string]any{"empty": nil, "keep": "me"}}},
	}
	for name, template := range tests {
		t.Run(name, func(t *testing.T) {
			job := newSyncInfoHook("batch/v1", "Job", "revision")
			require.NoError(t, unstructured.SetNestedField(job.Object, template, "spec", "template"))
			original := job.DeepCopy()

			hooks, err := injectSyncInfoIntoHooks([]*unstructured.Unstructured{job}, "sha-A", nil, nil, false)
			require.NoError(t, err)
			podAnnotations := podTemplateAnnotations(t, hooks[0])
			assert.Equal(t, "sha-A", podAnnotations[cdcommon.AnnotationSyncRevision])
			if name == "null annotation value" {
				assert.Equal(t, "me", podAnnotations["keep"])
				assert.Contains(t, podAnnotations, "empty")
			}
			assert.Equal(t, original.Object, job.Object, "desired hook not mutated")
			assert.Equal(t, "revision", hooks[0].GetAnnotations()[cdcommon.AnnotationInjectSyncInfo], "hook annotations kept")
		})
	}
}
