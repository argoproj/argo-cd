package manifestgen

import (
	"testing"

	"github.com/stretchr/testify/assert"

	v1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func ptr(p v1alpha1.ManifestGeneratePolicy) *v1alpha1.ManifestGeneratePolicy {
	return &p
}

func TestResolveManifestGeneratePolicy(t *testing.T) {
	strict := v1alpha1.ManifestGeneratePolicyStrict
	none := v1alpha1.ManifestGeneratePolicyNone

	tests := []struct {
		name          string
		appPolicy     *v1alpha1.ManifestGeneratePolicy
		projectPolicy *v1alpha1.ManifestGeneratePolicy
		globalPolicy  string
		expected      v1alpha1.ManifestGeneratePolicy
	}{
		{
			name:     "all nil/empty returns none",
			expected: none,
		},
		{
			name:         "global only",
			globalPolicy: "strict",
			expected:     strict,
		},
		{
			name:          "project overrides global",
			projectPolicy: ptr(strict),
			globalPolicy:  "",
			expected:      strict,
		},
		{
			name:          "app overrides project",
			appPolicy:     ptr(strict),
			projectPolicy: ptr(none),
			globalPolicy:  "",
			expected:      strict,
		},
		{
			name:          "app overrides global",
			appPolicy:     ptr(strict),
			globalPolicy:  "",
			expected:      strict,
		},
		{
			name:          "empty app falls through to project",
			appPolicy:     ptr(none),
			projectPolicy: ptr(strict),
			expected:      strict,
		},
		{
			name:          "empty app and project falls through to global",
			appPolicy:     ptr(none),
			projectPolicy: ptr(none),
			globalPolicy:  "strict",
			expected:      strict,
		},
		{
			name:      "nil app falls through to global",
			appPolicy: nil,
			globalPolicy: "strict",
			expected:  strict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveManifestGeneratePolicy(tt.appPolicy, tt.projectPolicy, tt.globalPolicy)
			assert.Equal(t, tt.expected, result)
		})
	}
}
