package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSourceIntegrity_ConfiguredMethods(t *testing.T) {
	t.Run("empty source integrity", func(t *testing.T) {
		sourceIntegrity := &SourceIntegrity{}

		methods := sourceIntegrity.ConfiguredMethods()

		assert.Empty(t, methods)
	})

	t.Run("source integrity with empty policies", func(t *testing.T) {
		sourceIntegrity := &SourceIntegrity{
			Git: &SourceIntegrityGit{
				Policies: []*SourceIntegrityGitPolicy{},
			},
		}

		methods := sourceIntegrity.ConfiguredMethods()

		assert.Empty(t, methods)
	})

	t.Run("source integrity with no GPG policy", func(t *testing.T) {
		sourceIntegrity := &SourceIntegrity{
			Git: &SourceIntegrityGit{
				Policies: []*SourceIntegrityGitPolicy{
					{
						Repos: []SourceIntegrityGitPolicyRepo{
							{
								URL: "*",
							},
						},
						GPG: nil,
					},
				},
			},
		}

		methods := sourceIntegrity.ConfiguredMethods()

		assert.Empty(t, methods)
	})

	t.Run("source integrity with GIT + GPG", func(t *testing.T) {
		sourceIntegrity := &SourceIntegrity{
			Git: &SourceIntegrityGit{
				Policies: []*SourceIntegrityGitPolicy{
					{
						Repos: []SourceIntegrityGitPolicyRepo{
							{
								URL: "*",
							},
						},
						GPG: &SourceIntegrityGitPolicyGPG{
							Mode: SourceIntegrityGitPolicyGPGModeHead,
							Keys: []string{"ABCD1234ABCD1234"},
						},
					},
				},
			},
		}

		methods := sourceIntegrity.ConfiguredMethods()

		assert.Len(t, methods, 1)
		assert.Equal(t, SourceIntegrityMethodNameGitGPG, methods[0])
	})

	t.Run("source integrity with multiple GPG policies", func(t *testing.T) {
		policy := &SourceIntegrityGitPolicy{
			Repos: []SourceIntegrityGitPolicyRepo{
				{
					URL: "*",
				},
			},
			GPG: &SourceIntegrityGitPolicyGPG{
				Mode: SourceIntegrityGitPolicyGPGModeHead,
				Keys: []string{"ABCD1234ABCD1234"},
			},
		}

		sourceIntegrity := &SourceIntegrity{
			Git: &SourceIntegrityGit{
				Policies: []*SourceIntegrityGitPolicy{policy, policy},
			},
		}

		methods := sourceIntegrity.ConfiguredMethods()

		assert.Len(t, methods, 1)
		assert.Equal(t, SourceIntegrityMethodNameGitGPG, methods[0])
	})
}
