package argo

import (
	"testing"

	"github.com/stretchr/testify/assert"

	argoappv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestMatchesLabels(t *testing.T) {
	app := &argoappv1.Application{
		Labels: map[string]string{
			"team":  "platform",
			"empty": "",
		},
	}

	t.Run("empty filter matches", func(t *testing.T) {
		assert.True(t, MatchesLabels(app, nil))
		assert.True(t, MatchesLabels(app, []string{}))
	})

	t.Run("key=value match", func(t *testing.T) {
		assert.True(t, MatchesLabels(app, []string{"team=platform"}))
	})

	t.Run("key=value mismatch", func(t *testing.T) {
		assert.False(t, MatchesLabels(app, []string{"team=other"}))
	})

	t.Run("empty value matches when label is present", func(t *testing.T) {
		assert.True(t, MatchesLabels(app, []string{"empty="}))
	})

	t.Run("empty value does not match missing key", func(t *testing.T) {
		assert.False(t, MatchesLabels(app, []string{"missing="}))
	})

	t.Run("key-only matches presence", func(t *testing.T) {
		assert.True(t, MatchesLabels(app, []string{"team"}))
		assert.False(t, MatchesLabels(app, []string{"missing"}))
	})

	t.Run("nil labels map does not match", func(t *testing.T) {
		assert.False(t, MatchesLabels(&argoappv1.Application{}, []string{"team=platform"}))
	})
}

func TestMatchesAutoSync(t *testing.T) {
	boolPtr := func(v bool) *bool { return &v }

	t.Run("empty filter matches", func(t *testing.T) {
		assert.True(t, MatchesAutoSync(&argoappv1.Application{}, nil))
	})

	t.Run("no sync policy is disabled", func(t *testing.T) {
		app := &argoappv1.Application{}
		assert.True(t, MatchesAutoSync(app, []string{"Disabled"}))
		assert.False(t, MatchesAutoSync(app, []string{"Enabled"}))
	})

	t.Run("nil automated is disabled", func(t *testing.T) {
		app := &argoappv1.Application{Spec: argoappv1.ApplicationSpec{SyncPolicy: &argoappv1.SyncPolicy{}}}
		assert.True(t, MatchesAutoSync(app, []string{"Disabled"}))
		assert.False(t, MatchesAutoSync(app, []string{"Enabled"}))
	})

	t.Run("automated with nil Enabled is enabled", func(t *testing.T) {
		app := &argoappv1.Application{
			Spec: argoappv1.ApplicationSpec{
				SyncPolicy: &argoappv1.SyncPolicy{Automated: &argoappv1.SyncPolicyAutomated{}},
			},
		}
		assert.True(t, MatchesAutoSync(app, []string{"Enabled"}))
		assert.False(t, MatchesAutoSync(app, []string{"Disabled"}))
	})

	t.Run("automated with Enabled true is enabled", func(t *testing.T) {
		app := &argoappv1.Application{
			Spec: argoappv1.ApplicationSpec{
				SyncPolicy: &argoappv1.SyncPolicy{
					Automated: &argoappv1.SyncPolicyAutomated{Enabled: boolPtr(true)},
				},
			},
		}
		assert.True(t, MatchesAutoSync(app, []string{"Enabled"}))
		assert.False(t, MatchesAutoSync(app, []string{"Disabled"}))
	})

	t.Run("automated with Enabled false is disabled", func(t *testing.T) {
		app := &argoappv1.Application{
			Spec: argoappv1.ApplicationSpec{
				SyncPolicy: &argoappv1.SyncPolicy{
					Automated: &argoappv1.SyncPolicyAutomated{Enabled: boolPtr(false)},
				},
			},
		}
		assert.True(t, MatchesAutoSync(app, []string{"Disabled"}))
		assert.False(t, MatchesAutoSync(app, []string{"Enabled"}))
	})
}
