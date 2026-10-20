package argo

import (
	"os"
	"testing"

	"github.com/argoproj/argo-cd/v3/util/kube"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestSetAppInstanceLabel(t *testing.T) {
	t.Parallel()
	yamlBytes, err := os.ReadFile("testdata/svc.yaml")
	require.NoError(t, err)

	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	require.NoError(t, err)

	resourceTracking := NewResourceTracking()

	err = resourceTracking.SetAppInstance(&obj, common.LabelKeyAppInstance, "my-app", "", v1alpha1.TrackingMethodLabel, "")
	require.NoError(t, err)
	app := resourceTracking.GetAppName(&obj, common.LabelKeyAppInstance, v1alpha1.TrackingMethodLabel, "")
	assert.Equal(t, "my-app", app)
}

func TestSetAppInstanceAnnotation(t *testing.T) {
	t.Parallel()
	yamlBytes, err := os.ReadFile("testdata/svc.yaml")
	require.NoError(t, err)

	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	require.NoError(t, err)

	resourceTracking := NewResourceTracking()

	err = resourceTracking.SetAppInstance(&obj, common.AnnotationKeyAppInstance, "my-app", "", v1alpha1.TrackingMethodAnnotation, "")
	require.NoError(t, err)

	app := resourceTracking.GetAppName(&obj, common.AnnotationKeyAppInstance, v1alpha1.TrackingMethodAnnotation, "")
	assert.Equal(t, "my-app", app)
}

func TestGetAppName_AnnotationWithExistingInstallationID(t *testing.T) {
	t.Parallel()
	yamlBytes, err := os.ReadFile("testdata/svc.yaml")
	require.NoError(t, err)

	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	require.NoError(t, err)

	resourceTracking := NewResourceTracking()

	err = resourceTracking.SetAppInstance(&obj, common.AnnotationKeyAppInstance, "my-app", "", v1alpha1.TrackingMethodAnnotation, "some-installation-id")
	require.NoError(t, err)

	// When no installationID filter is provided, the app name should still be returned
	// even though the resource has an installation ID annotation.
	app := resourceTracking.GetAppName(&obj, common.AnnotationKeyAppInstance, v1alpha1.TrackingMethodAnnotation, "")
	assert.Equal(t, "my-app", app)

	// When a different installationID is provided, it should not match.
	app = resourceTracking.GetAppName(&obj, common.AnnotationKeyAppInstance, v1alpha1.TrackingMethodAnnotation, "different-id")
	assert.Empty(t, app)

	// When the correct installationID is provided, it should match.
	app = resourceTracking.GetAppName(&obj, common.AnnotationKeyAppInstance, v1alpha1.TrackingMethodAnnotation, "some-installation-id")
	assert.Equal(t, "my-app", app)
}

func TestSetAppInstanceAnnotationAndLabel(t *testing.T) {
	t.Parallel()
	yamlBytes, err := os.ReadFile("testdata/svc.yaml")
	require.NoError(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	require.NoError(t, err)

	resourceTracking := NewResourceTracking()

	err = resourceTracking.SetAppInstance(&obj, common.LabelKeyAppInstance, "my-app", "", v1alpha1.TrackingMethodAnnotationAndLabel, "")
	require.NoError(t, err)

	app := resourceTracking.GetAppName(&obj, common.LabelKeyAppInstance, v1alpha1.TrackingMethodAnnotationAndLabel, "")
	assert.Equal(t, "my-app", app)
}

func TestSetAppInstanceAnnotationAndLabelLongName(t *testing.T) {
	t.Parallel()
	yamlBytes, err := os.ReadFile("testdata/svc.yaml")
	require.NoError(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	require.NoError(t, err)

	resourceTracking := NewResourceTracking()

	err = resourceTracking.SetAppInstance(&obj, common.LabelKeyAppInstance, "my-app-with-an-extremely-long-name-that-is-over-sixty-three-characters", "", v1alpha1.TrackingMethodAnnotationAndLabel, "")
	require.NoError(t, err)

	// the annotation should still work, so the name from GetAppName should not be truncated
	app := resourceTracking.GetAppName(&obj, common.LabelKeyAppInstance, v1alpha1.TrackingMethodAnnotationAndLabel, "")
	assert.Equal(t, "my-app-with-an-extremely-long-name-that-is-over-sixty-three-characters", app)

	// the label should be truncated to 63 characters
	assert.Equal(t, "my-app-with-an-extremely-long-name-that-is-over-sixty-three-cha", obj.GetLabels()[common.LabelKeyAppInstance])
}

func TestSetAppInstanceAnnotationAndLabelLongNameBadEnding(t *testing.T) {
	t.Parallel()
	yamlBytes, err := os.ReadFile("testdata/svc.yaml")
	require.NoError(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	require.NoError(t, err)

	resourceTracking := NewResourceTracking()

	err = resourceTracking.SetAppInstance(&obj, common.LabelKeyAppInstance, "the-very-suspicious-name-with-precisely-sixty-three-characters-with-hyphen", "", v1alpha1.TrackingMethodAnnotationAndLabel, "")
	require.NoError(t, err)

	// the annotation should still work, so the name from GetAppName should not be truncated
	app := resourceTracking.GetAppName(&obj, common.LabelKeyAppInstance, v1alpha1.TrackingMethodAnnotationAndLabel, "")
	assert.Equal(t, "the-very-suspicious-name-with-precisely-sixty-three-characters-with-hyphen", app)

	// the label should be truncated to 63 characters, AND the hyphen should be removed
	assert.Equal(t, "the-very-suspicious-name-with-precisely-sixty-three-characters", obj.GetLabels()[common.LabelKeyAppInstance])
}

func TestSetAppInstanceAnnotationAndLabelOutOfBounds(t *testing.T) {
	t.Parallel()
	yamlBytes, err := os.ReadFile("testdata/svc.yaml")
	require.NoError(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	require.NoError(t, err)

	resourceTracking := NewResourceTracking()

	err = resourceTracking.SetAppInstance(&obj, common.LabelKeyAppInstance, "----------------------------------------------------------------", "", v1alpha1.TrackingMethodAnnotationAndLabel, "")
	// this should error because it can't truncate to a valid value
	assert.EqualError(t, err, "failed to set app instance label: unable to truncate label to not end with a special character")
}

func TestTruncateLabel_Short(t *testing.T) {
	t.Parallel()
	got, err := TruncateLabel("my-app")
	require.NoError(t, err)
	assert.Equal(t, "my-app", got)
}

func TestTruncateLabel_AtLimit(t *testing.T) {
	t.Parallel()
	val := "the-very-suspicious-name-with-precisely-sixty-three-characters0"
	require.Len(t, val, LabelMaxLength)
	got, err := TruncateLabel(val)
	require.NoError(t, err)
	assert.Equal(t, val, got)
}

func TestTruncateLabel_LongName(t *testing.T) {
	t.Parallel()
	got, err := TruncateLabel("my-app-with-an-extremely-long-name-that-is-over-sixty-three-characters")
	require.NoError(t, err)
	assert.Equal(t, "my-app-with-an-extremely-long-name-that-is-over-sixty-three-cha", got)
	assert.LessOrEqual(t, len(got), LabelMaxLength)
}

func TestTruncateLabel_TrailingSpecialCharsStripped(t *testing.T) {
	t.Parallel()
	got, err := TruncateLabel("the-very-suspicious-name-with-precisely-sixty-three-characters-with-hyphen")
	require.NoError(t, err)
	assert.Equal(t, "the-very-suspicious-name-with-precisely-sixty-three-characters", got)
}

func TestTruncateLabel_AllSpecialChars(t *testing.T) {
	t.Parallel()
	_, err := TruncateLabel("----------------------------------------------------------------")
	assert.EqualError(t, err, "unable to truncate label to not end with a special character")
}

func TestRemoveAppInstance_LabelOnly(t *testing.T) {
	t.Parallel()
	yamlBytes, err := os.ReadFile("testdata/svc.yaml")
	require.NoError(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	require.NoError(t, err)

	rt := NewResourceTracking()

	err = rt.SetAppInstance(&obj, common.LabelKeyAppInstance, "my-app", "", v1alpha1.TrackingMethodLabel, "")
	require.NoError(t, err)

	err = rt.RemoveAppInstance(&obj, string(v1alpha1.TrackingMethodLabel))
	require.NoError(t, err)

	_, exists := obj.GetLabels()[common.LabelKeyAppInstance]
	assert.False(t, exists)
}

func TestRemoveAppInstance_AnnotationOnly(t *testing.T) {
	t.Parallel()
	yamlBytes, err := os.ReadFile("testdata/svc.yaml")
	require.NoError(t, err)

	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	require.NoError(t, err)

	rt := NewResourceTracking()

	err = rt.SetAppInstance(&obj, common.AnnotationKeyAppInstance, "my-app", "", v1alpha1.TrackingMethodAnnotation, "")
	require.NoError(t, err)

	err = rt.RemoveAppInstance(&obj, string(v1alpha1.TrackingMethodAnnotation))
	require.NoError(t, err)

	annotations := obj.GetAnnotations()
	assert.NotContains(t, annotations, common.AnnotationKeyAppInstance)
	assert.NotContains(t, annotations, common.AnnotationInstallationID)
	assert.NotContains(t, annotations, v1alpha1.TrackingMethodAnnotation)
}

func TestRemoveAppInstance_AnnotationAndLabel(t *testing.T) {
	t.Parallel()
	yamlBytes, err := os.ReadFile("testdata/svc.yaml")
	require.NoError(t, err)

	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	require.NoError(t, err)

	rt := NewResourceTracking()

	err = rt.SetAppInstance(&obj, common.LabelKeyAppInstance, "my-app", "", v1alpha1.TrackingMethodAnnotationAndLabel, "")
	require.NoError(t, err)

	err = rt.RemoveAppInstance(&obj, string(v1alpha1.TrackingMethodAnnotationAndLabel))
	require.NoError(t, err)

	assert.NotContains(t, obj.GetAnnotations(), common.AnnotationKeyAppInstance)
	assert.NotContains(t, obj.GetAnnotations(), common.AnnotationInstallationID)
	assert.NotContains(t, obj.GetLabels(), common.LabelKeyAppInstance)
}

func TestRemoveAppInstance_DefaultCase(t *testing.T) {
	t.Parallel()
	yamlBytes, err := os.ReadFile("testdata/svc.yaml")
	require.NoError(t, err)

	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	require.NoError(t, err)

	// Add a label manually to verify if this custom label exists at the end
	obj.SetLabels(map[string]string{
		"my-custom-label": "keep-me",
	})

	rt := NewResourceTracking()

	err = rt.SetAppInstance(&obj, common.AnnotationKeyAppInstance, "my-app", "", "", "")
	require.NoError(t, err)

	err = rt.RemoveAppInstance(&obj, "unknown-method")
	require.NoError(t, err)

	assert.NotContains(t, obj.GetAnnotations(), common.AnnotationKeyAppInstance)
	assert.NotContains(t, obj.GetAnnotations(), common.AnnotationInstallationID)

	// Argo CD app-instance label was never added, so it shouldn't exist
	_, argocdLabelExists := obj.GetLabels()[common.LabelKeyAppInstance]
	assert.False(t, argocdLabelExists)
	// Custom label should still exist
	assert.Equal(t, "keep-me", obj.GetLabels()["my-custom-label"])
}

func TestRemoveAppInstance_AnnotationAndLabel_LongName(t *testing.T) {
	t.Parallel()
	yamlBytes, err := os.ReadFile("testdata/svc.yaml")
	require.NoError(t, err)

	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	require.NoError(t, err)

	rt := NewResourceTracking()

	longName := "my-app-with-an-extremely-long-name-that-is-over-sixty-three-characters"
	err = rt.SetAppInstance(&obj, common.LabelKeyAppInstance, longName, "", v1alpha1.TrackingMethodAnnotationAndLabel, "")
	require.NoError(t, err)

	err = rt.RemoveAppInstance(&obj, string(v1alpha1.TrackingMethodAnnotationAndLabel))
	require.NoError(t, err)

	assert.NotContains(t, obj.GetAnnotations(), common.AnnotationKeyAppInstance)
	assert.NotContains(t, obj.GetLabels(), common.LabelKeyAppInstance)
}

func TestSetAppInstanceAnnotationNotFound(t *testing.T) {
	t.Parallel()
	yamlBytes, err := os.ReadFile("testdata/svc.yaml")
	require.NoError(t, err)

	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	require.NoError(t, err)

	resourceTracking := NewResourceTracking()

	app := resourceTracking.GetAppName(&obj, common.LabelKeyAppInstance, v1alpha1.TrackingMethodAnnotation, "")
	assert.Empty(t, app)
}

func TestParseAppInstanceValue(t *testing.T) {
	t.Parallel()
	resourceTracking := NewResourceTracking()
	appInstanceValue, err := resourceTracking.ParseAppInstanceValue("app:<group>/<kind>:<namespace>/<name>")
	require.NoError(t, err)
	assert.Equal(t, "app", appInstanceValue.ApplicationName)
	assert.Equal(t, "<group>", appInstanceValue.Group)
	assert.Equal(t, "<kind>", appInstanceValue.Kind)
	assert.Equal(t, "<namespace>", appInstanceValue.Namespace)
	assert.Equal(t, "<name>", appInstanceValue.Name)
}

func TestParseAppInstanceValueColon(t *testing.T) {
	t.Parallel()
	resourceTracking := NewResourceTracking()
	appInstanceValue, err := resourceTracking.ParseAppInstanceValue("app:<group>/<kind>:<namespace>/<name>:<colon>")
	require.NoError(t, err)
	assert.Equal(t, "app", appInstanceValue.ApplicationName)
	assert.Equal(t, "<group>", appInstanceValue.Group)
	assert.Equal(t, "<kind>", appInstanceValue.Kind)
	assert.Equal(t, "<namespace>", appInstanceValue.Namespace)
	assert.Equal(t, "<name>:<colon>", appInstanceValue.Name)
}

func TestParseAppInstanceValueWrongFormat1(t *testing.T) {
	t.Parallel()
	resourceTracking := NewResourceTracking()
	_, err := resourceTracking.ParseAppInstanceValue("app")
	require.ErrorIs(t, err, ErrWrongResourceTrackingFormat)
}

func TestParseAppInstanceValueWrongFormat2(t *testing.T) {
	t.Parallel()
	resourceTracking := NewResourceTracking()
	_, err := resourceTracking.ParseAppInstanceValue("app;group/kind/ns")
	require.ErrorIs(t, err, ErrWrongResourceTrackingFormat)
}

func TestParseAppInstanceValueCorrectFormat(t *testing.T) {
	t.Parallel()
	resourceTracking := NewResourceTracking()
	_, err := resourceTracking.ParseAppInstanceValue("app:group/kind:test/ns")
	require.NoError(t, err)
}

func sampleResource(t *testing.T) *unstructured.Unstructured {
	t.Helper()
	yamlBytes, err := os.ReadFile("testdata/svc.yaml")
	require.NoError(t, err)
	var obj *unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	require.NoError(t, err)
	return obj
}

func TestResourceIdNormalizer_Normalize(t *testing.T) {
	t.Parallel()
	rt := NewResourceTracking()

	// live object is a resource that has old style tracking label
	liveObj := sampleResource(t)
	err := rt.SetAppInstance(liveObj, common.LabelKeyAppInstance, "my-app", "", v1alpha1.TrackingMethodLabel, "")
	require.NoError(t, err)

	// config object is a resource that has new style tracking annotation
	configObj := sampleResource(t)
	err = rt.SetAppInstance(configObj, common.AnnotationKeyAppInstance, "my-app2", "", v1alpha1.TrackingMethodAnnotation, "")
	require.NoError(t, err)

	_ = rt.Normalize(configObj, liveObj, common.LabelKeyAppInstance, string(v1alpha1.TrackingMethodAnnotation))

	// the normalization should affect add the new style annotation and drop old tracking label from live object
	annotation, err := kube.GetAppInstanceAnnotation(configObj, common.AnnotationKeyAppInstance)
	require.NoError(t, err)
	assert.Equal(t, liveObj.GetAnnotations()[common.AnnotationKeyAppInstance], annotation)
	_, hasOldLabel := liveObj.GetLabels()[common.LabelKeyAppInstance]
	assert.False(t, hasOldLabel)
}

func TestResourceIdNormalizer_NormalizeCRD(t *testing.T) {
	t.Parallel()
	rt := NewResourceTracking()

	// live object is a CRD resource
	liveObj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apiextensions.k8s.io/v1",
			"kind":       "CustomResourceDefinition",
			"metadata": map[string]any{
				"name": "crontabs.stable.example.com",
				"labels": map[string]any{
					common.LabelKeyAppInstance: "my-app",
				},
			},
			"spec": map[string]any{
				"group": "stable.example.com",
				"scope": "Namespaced",
			},
		},
	}

	// config object is a CRD resource
	configObj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apiextensions.k8s.io/v1",
			"kind":       "CustomResourceDefinition",
			"metadata": map[string]any{
				"name": "crontabs.stable.example.com",
				"labels": map[string]any{
					common.LabelKeyAppInstance: "my-app",
				},
			},
			"spec": map[string]any{
				"group": "stable.example.com",
				"scope": "Namespaced",
			},
		},
	}

	require.NoError(t, rt.Normalize(configObj, liveObj, common.LabelKeyAppInstance, string(v1alpha1.TrackingMethodAnnotation)))
	// the normalization should not apply any changes to the live object
	require.NotContains(t, liveObj.GetAnnotations(), common.AnnotationKeyAppInstance)
}

func TestResourceIdNormalizer_Normalize_ConfigHasOldLabel(t *testing.T) {
	t.Parallel()
	rt := NewResourceTracking()

	// live object is a resource that has old style tracking label
	liveObj := sampleResource(t)
	err := rt.SetAppInstance(liveObj, common.LabelKeyAppInstance, "my-app", "", v1alpha1.TrackingMethodLabel, "")
	require.NoError(t, err)

	// config object is a resource that has new style tracking annotation
	configObj := sampleResource(t)
	err = rt.SetAppInstance(configObj, common.AnnotationKeyAppInstance, "my-app2", "", v1alpha1.TrackingMethodAnnotation, "")
	require.NoError(t, err)
	err = rt.SetAppInstance(configObj, common.LabelKeyAppInstance, "my-app", "", v1alpha1.TrackingMethodLabel, "")
	require.NoError(t, err)

	_ = rt.Normalize(configObj, liveObj, common.LabelKeyAppInstance, string(v1alpha1.TrackingMethodAnnotation))

	// the normalization should affect add the new style annotation and drop old tracking label from live object
	annotation, err := kube.GetAppInstanceAnnotation(configObj, common.AnnotationKeyAppInstance)
	require.NoError(t, err)
	assert.Equal(t, liveObj.GetAnnotations()[common.AnnotationKeyAppInstance], annotation)
	_, hasOldLabel := liveObj.GetLabels()[common.LabelKeyAppInstance]
	assert.True(t, hasOldLabel)
}

// movedResource returns a ConfigMap as used in the moved-between-apps scenario
// of issue #17965.
func movedResource() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":      "my-cm",
			"namespace": "default",
		},
	}}
}

func TestResourceIdNormalizer_Normalize_ResourceMovedToAnotherApp(t *testing.T) {
	t.Parallel()
	rt := NewResourceTracking()

	// live object is tracked by app-a with annotation+label tracking
	liveObj := movedResource()
	err := rt.SetAppInstance(liveObj, common.LabelKeyAppInstance, "app-a", "", v1alpha1.TrackingMethodAnnotationAndLabel, "")
	require.NoError(t, err)

	// config object is the same resource, now managed by app-b
	configObj := movedResource()
	err = rt.SetAppInstance(configObj, common.LabelKeyAppInstance, "app-b", "", v1alpha1.TrackingMethodAnnotationAndLabel, "")
	require.NoError(t, err)

	err = rt.Normalize(configObj, liveObj, common.LabelKeyAppInstance, string(v1alpha1.TrackingMethodAnnotationAndLabel))
	require.NoError(t, err)

	// the stale tracking annotation on the live object must be preserved so that
	// the diff surfaces and a sync can update it (issue #17965)
	assert.Equal(t, "app-a:/ConfigMap:default/my-cm", liveObj.GetAnnotations()[common.AnnotationKeyAppInstance])
	assert.Equal(t, "app-b:/ConfigMap:default/my-cm", configObj.GetAnnotations()[common.AnnotationKeyAppInstance])
}

func TestResourceIdNormalizer_Normalize_ResourceMovedToAnotherApp_AnnotationTracking(t *testing.T) {
	t.Parallel()
	rt := NewResourceTracking()

	// live object was previously synced with annotation+label tracking by app-a,
	// so it still carries the instance label
	liveObj := movedResource()
	err := rt.SetAppInstance(liveObj, common.LabelKeyAppInstance, "app-a", "", v1alpha1.TrackingMethodAnnotationAndLabel, "")
	require.NoError(t, err)

	// config object is the same resource, now managed by app-b with annotation tracking
	configObj := movedResource()
	err = rt.SetAppInstance(configObj, common.LabelKeyAppInstance, "app-b", "", v1alpha1.TrackingMethodAnnotation, "")
	require.NoError(t, err)

	err = rt.Normalize(configObj, liveObj, common.LabelKeyAppInstance, string(v1alpha1.TrackingMethodAnnotation))
	require.NoError(t, err)

	// the stale tracking annotation must be preserved so the diff surfaces (issue #17965),
	// while the stale label is still dropped to smooth the label->annotation migration
	assert.Equal(t, "app-a:/ConfigMap:default/my-cm", liveObj.GetAnnotations()[common.AnnotationKeyAppInstance])
	_, hasOldLabel := liveObj.GetLabels()[common.LabelKeyAppInstance]
	assert.False(t, hasOldLabel)
}

func TestResourceIdNormalizer_Normalize_ResourceMovedToAnotherApp_HelmInstanceLabel(t *testing.T) {
	t.Parallel()
	rt := NewResourceTracking()

	// Both apps use annotation-only tracking, but the manifest itself bakes in
	// the app.kubernetes.io/instance label (as Helm charts commonly do). The
	// label alone must not cause the stale tracking annotation to be hidden.
	withHelmLabel := func(un *unstructured.Unstructured) *unstructured.Unstructured {
		labels := un.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels[common.LabelKeyAppInstance] = "my-release"
		un.SetLabels(labels)
		return un
	}

	// live object is tracked by the old standalone app
	liveObj := withHelmLabel(movedResource())
	err := rt.SetAppInstance(liveObj, common.LabelKeyAppInstance, "standalone-app", "", v1alpha1.TrackingMethodAnnotation, "")
	require.NoError(t, err)

	// config object is the same resource, now rendered for the appset-generated app
	configObj := withHelmLabel(movedResource())
	err = rt.SetAppInstance(configObj, common.LabelKeyAppInstance, "appset-app", "", v1alpha1.TrackingMethodAnnotation, "")
	require.NoError(t, err)

	err = rt.Normalize(configObj, liveObj, common.LabelKeyAppInstance, string(v1alpha1.TrackingMethodAnnotation))
	require.NoError(t, err)

	// the stale tracking annotation must be preserved so the diff surfaces (issue #17965)
	assert.Equal(t, "standalone-app:/ConfigMap:default/my-cm", liveObj.GetAnnotations()[common.AnnotationKeyAppInstance])
	// the manifest-defined helm instance label is untouched
	assert.Equal(t, "my-release", liveObj.GetLabels()[common.LabelKeyAppInstance])
}

func TestIsOldTrackingMethod(t *testing.T) {
	t.Parallel()
	assert.True(t, IsOldTrackingMethod(string(v1alpha1.TrackingMethodLabel)))
}
