package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func TestParseLabels(t *testing.T) {
	validLabels := []string{"key=value", "foo=bar", "intuit=inc"}

	result, err := parseLabels(validLabels)
	assert.NoError(t, err)
	assert.Len(t, result, 3)

	invalidLabels := []string{"key=value", "too=many=equals"}
	_, err = parseLabels(invalidLabels)
	assert.Error(t, err)

	emptyLabels := []string{}
	result, err = parseLabels(emptyLabels)
	assert.NoError(t, err)
	assert.Len(t, result, 0)

}

func Test_setHelmOpt(t *testing.T) {
	t.Run("Zero", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setHelmOpt(&src, helmOpts{})
		assert.Nil(t, src.Helm)
	})
	t.Run("ValueFiles", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setHelmOpt(&src, helmOpts{valueFiles: []string{"foo"}})
		assert.Equal(t, []string{"foo"}, src.Helm.ValueFiles)
	})
	t.Run("ReleaseName", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setHelmOpt(&src, helmOpts{releaseName: "foo"})
		assert.Equal(t, "foo", src.Helm.ReleaseName)
	})
	t.Run("HelmSets", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setHelmOpt(&src, helmOpts{helmSets: []string{"foo=bar"}})
		assert.Equal(t, []v1alpha1.HelmParameter{{Name: "foo", Value: "bar"}}, src.Helm.Parameters)
	})
	t.Run("HelmSetStrings", func(t *testing.T) {
		src := v1alpha1.ApplicationSource{}
		setHelmOpt(&src, helmOpts{helmSetStrings: []string{"foo=bar"}})
		assert.Equal(t, []v1alpha1.HelmParameter{{Name: "foo", Value: "bar", ForceString: true}}, src.Helm.Parameters)
	})
}

func Test_setMaintenanceWindows(t *testing.T) {
	t.Run("SingleWindow", func(t *testing.T) {
		sched := "0 10 * * *"
		dur := "1h"
		windows := sched + ":" + dur
		ap := v1alpha1.Application{}
		ap.Spec.SyncPolicy = &v1alpha1.SyncPolicy{Automated: &v1alpha1.SyncPolicyAutomated{}}
		setMaintenanceWindows(&ap, windows)
		assert.Equal(t, &v1alpha1.MaintenanceWindow{Schedule: &sched, Duration: &dur}, ap.Spec.SyncPolicy.Automated.MaintenanceWindows[0])
	})
	t.Run("MultipleWindows", func(t *testing.T) {
		sched1 := "0 10 * * *"
		dur1 := "1h"
		sched2 := "0 22 * * *"
		dur2 := "1h"
		windows := sched1 + ":" + dur1 + "," + sched2 + ":" + dur2
		ap := v1alpha1.Application{}
		ap.Spec.SyncPolicy = &v1alpha1.SyncPolicy{Automated: &v1alpha1.SyncPolicyAutomated{}}
		setMaintenanceWindows(&ap, windows)
		assert.Equal(t, &v1alpha1.MaintenanceWindow{Schedule: &sched1, Duration: &dur1}, ap.Spec.SyncPolicy.Automated.MaintenanceWindows[0])
		assert.Equal(t, &v1alpha1.MaintenanceWindow{Schedule: &sched2, Duration: &dur2}, ap.Spec.SyncPolicy.Automated.MaintenanceWindows[1])
	})
}
