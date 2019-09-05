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

func Test_setSyncPolicy(t *testing.T) {
	t.Run("automated", func(t *testing.T) {
		spec := &v1alpha1.ApplicationSpec{}
		setSyncPolicy(spec, "automated")
		assert.Equal(t, &v1alpha1.SyncPolicyAutomated{}, spec.SyncPolicy.Automated)
	})
}

func Test_setMaintenance(t *testing.T) {
	t.Run("enable", func(t *testing.T) {
		spec := &v1alpha1.ApplicationSpec{}
		setMaintenance(spec, "enable")
		assert.True(t, spec.SyncPolicy.Maintenance.Enabled)
	})
	t.Run("disable", func(t *testing.T) {
		spec := &v1alpha1.ApplicationSpec{SyncPolicy: &v1alpha1.SyncPolicy{
			Maintenance: &v1alpha1.Maintenance{Enabled: true},
		}}
		setMaintenance(spec, "disable")
		assert.False(t, spec.SyncPolicy.Maintenance.Enabled)
	})
}

func Test_setMaintenanceWindows(t *testing.T) {
	t.Run("single", func(t *testing.T) {
		spec := &v1alpha1.ApplicationSpec{SyncPolicy: &v1alpha1.SyncPolicy{
			Maintenance: &v1alpha1.Maintenance{Enabled: true}}}
		setMaintenanceWindows(spec, "* * * * *:1h")
		assert.Equal(t, "* * * * *", spec.SyncPolicy.Maintenance.Windows[0].Schedule)
		assert.Equal(t, "1h", spec.SyncPolicy.Maintenance.Windows[0].Duration)
	})
	t.Run("multiple", func(t *testing.T) {
		spec := &v1alpha1.ApplicationSpec{SyncPolicy: &v1alpha1.SyncPolicy{
			Maintenance: &v1alpha1.Maintenance{Enabled: true}}}
		setMaintenanceWindows(spec, "* * * * *:1h,1 1 1 1 1:1h")
		assert.Equal(t, "* * * * *", spec.SyncPolicy.Maintenance.Windows[0].Schedule)
		assert.Equal(t, "1h", spec.SyncPolicy.Maintenance.Windows[0].Duration)
	})
}
