package v1alpha1

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/util/git"
	gitmocks "github.com/argoproj/argo-cd/v3/util/git/mocks"
	utilTest "github.com/argoproj/argo-cd/v3/util/test"
)

func Test_IsGPGEnabled(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		t.Setenv("ARGOCD_GPG_ENABLED", "true")
		assert.True(t, IsGPGEnabled())
	})

	t.Run("false", func(t *testing.T) {
		t.Setenv("ARGOCD_GPG_ENABLED", "false")
		assert.False(t, IsGPGEnabled())
	})

	t.Run("empty", func(t *testing.T) {
		t.Setenv("ARGOCD_GPG_ENABLED", "")
		assert.True(t, IsGPGEnabled())
	})
}

func TestNullOrEmptyDoesNothing(t *testing.T) {
	gitClient := &gitmocks.Client{}
	repoURL := "https://github.com/argoproj/argo-cd"
	gitClient.EXPECT().RepoURL().Return(repoURL)

	tests := []struct {
		name   string
		si     *SourceIntegrity
		logged []string
	}{
		{
			name: "nil",
			si:   nil,
		},
		{
			name: "No GIT",
			si:   &SourceIntegrity{
				// No Git or alternative specified
			},
		},
		{
			name: "No matching policy",
			si:   &SourceIntegrity{Git: &SourceIntegrityGit{
				// No policies configured here
			}},
		},
		{
			name: "Matching policy does nothing",
			si: &SourceIntegrity{Git: &SourceIntegrityGit{Policies: []*SourceIntegrityGitPolicy{{
				Repos: []string{"*"},
				// No GPG or alternative specified
			}}}},
			logged: []string{"No verification configured for SourceIntegrity policy for [*]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := utilTest.LogHook{}
			logrus.AddHook(&logger)
			t.Cleanup(logger.CleanupHook)

			assert.Nil(t, tt.si.ForGit(repoURL))
			assert.Equal(t, tt.logged, logger.GetEntries())
		})
	}
}

func TestPolicyMatching(t *testing.T) {
	eitherOr := &SourceIntegrityGitPolicy{
		Repos: []string{"https://github.com/group/either.git", "https://github.com/group/or.git"},
		GPG:   &SourceIntegrityGitPolicyGPG{},
	}
	group := &SourceIntegrityGitPolicy{
		Repos: []string{"https://github.com/group/*"},
		GPG:   &SourceIntegrityGitPolicyGPG{},
	}
	prefix := &SourceIntegrityGitPolicy{
		Repos: []string{"https://github.com/group*"},
		GPG:   &SourceIntegrityGitPolicyGPG{},
	}
	p := func(ps ...*SourceIntegrityGitPolicy) []*SourceIntegrityGitPolicy { return ps }

	sig := &SourceIntegrityGit{
		Policies: []*SourceIntegrityGitPolicy{eitherOr, group, prefix},
	}

	assert.Equal(t, p(eitherOr, group, prefix), sig.findMatchingPolicies("https://github.com/group/either.git"))
	assert.Equal(t, p(eitherOr, group, prefix), sig.findMatchingPolicies("https://github.com/group/or.git"))

	assert.Equal(t, p(group, prefix), sig.findMatchingPolicies("https://github.com/group/fork.git"))

	assert.Equal(t, p(prefix), sig.findMatchingPolicies("https://github.com/grouplette/main.git"))

	assert.Equal(t, p(), sig.findMatchingPolicies("https://gitlab.com/foo/bar.git"))
}

func TestGPGHeadValid(t *testing.T) {
	const sha = "0c7a9c3f939c1f19b518bcdd11e2fce9703c4901"
	const tag = "tag"
	testCases := []struct {
		revision string
		check    func(gitClient *gitmocks.Client, logger utilTest.LogHook)
	}{
		{
			revision: sha,
			check: func(gitClient *gitmocks.Client, logger utilTest.LogHook) {
				gitClient.AssertCalled(t, "IsAnnotatedTag", sha)
				gitClient.AssertCalled(t, "LsSignatures", sha, false)
				gitClient.AssertNotCalled(t, "TagSignature", mock.Anything)
				assert.Empty(t, logger.GetEntries())
			},
		},
		{
			revision: tag,
			check: func(gitClient *gitmocks.Client, logger utilTest.LogHook) {
				gitClient.AssertCalled(t, "IsAnnotatedTag", tag)
				gitClient.AssertCalled(t, "TagSignature", tag)
				gitClient.AssertNotCalled(t, "LsSignatures", mock.Anything, mock.Anything)
				assert.Empty(t, logger.GetEntries())
			},
		},
	}

	for _, test := range testCases {
		t.Run("verify "+test.revision, func(t *testing.T) {
			// Given repo with tagged commit
			gitClient := &gitmocks.Client{}
			gitClient.EXPECT().CommitSHA().RunAndReturn(func() (string, error) { return sha, nil })
			gitClient.EXPECT().IsAnnotatedTag(mock.Anything).RunAndReturn(func(s string) bool { return tag == s })
			gitClient.EXPECT().LsSignatures(mock.Anything, mock.Anything).RunAndReturn(func(revision string, _ bool) ([]git.RevisionSignatureInfo, error) {
				return []git.RevisionSignatureInfo{{
					Revision: revision, VerificationResult: git.GPGVerificationResultGood, SignatureKeyID: "KEYID", Date: "ignored", AuthorIdentity: "ignored",
				}}, nil
			})
			gitClient.EXPECT().TagSignature(mock.Anything).RunAndReturn(func(revision string) (*git.RevisionSignatureInfo, error) {
				return &git.RevisionSignatureInfo{
					Revision: revision, VerificationResult: git.GPGVerificationResultGood, SignatureKeyID: "KEYID", Date: "ignored", AuthorIdentity: "ignored",
				}, nil
			})

			logger := utilTest.LogHook{}
			logrus.AddHook(&logger)
			t.Cleanup(logger.CleanupHook)

			// When using head mode
			gpgWithTag := SourceIntegrityGitPolicyGPG{SourceIntegrityGitPolicyGPGModeHead, []string{"KEYID", "one_more"}}
			// And verifying a given revision
			result, err := gpgWithTag.verify(gitClient, test.revision)
			require.NoError(t, err)
			// Then it is checked and valid
			assert.True(t, result.IsValid())
			assert.Equal(t, []string{"GIT/GPG"}, result.PassedChecks())
			test.check(gitClient, logger)
			require.NoError(t, result.Error())
		})
	}
}

func TestGPGStrictValid(t *testing.T) {
	const shaFirst = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const shaSecond = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	const shaThird = "cccccccccccccccccccccccccccccccccccccccc"
	const tagFirst = "tag-first"
	const tagSecond = "tag-second"
	const tagThird = "tag-third"
	tag2commit := map[string]string{
		tagFirst:  shaFirst,
		tagSecond: shaSecond,
		tagThird:  shaThird,
	}
	rsi3 := git.RevisionSignatureInfo{
		Revision: shaThird, VerificationResult: git.GPGVerificationResultGood,
		SignatureKeyID: "KEY_OF_THIRD", Date: "ignored", AuthorIdentity: "ignored",
	}
	rsi2 := git.RevisionSignatureInfo{
		Revision: shaSecond, VerificationResult: git.GPGVerificationResultGood,
		SignatureKeyID: "KEY_OF_SECOND", Date: "ignored", AuthorIdentity: "ignored",
	}
	rsi1 := git.RevisionSignatureInfo{
		Revision: shaFirst, VerificationResult: git.GPGVerificationResultGood,
		SignatureKeyID: "KEY_OF_FIRST", Date: "ignored", AuthorIdentity: "ignored",
	}

	tests := []struct {
		revision        string
		expectedErr     string
		expectedPassed  []string
		expectedTagArgs []any
		expectedLsArgs  []any
	}{
		{
			revision:       shaFirst,
			expectedPassed: []string{"GIT/GPG"},
			expectedLsArgs: []any{shaFirst, true},
		},
		{
			revision:       shaSecond,
			expectedPassed: []string{"GIT/GPG"},
			expectedLsArgs: []any{shaSecond, true},
		},
		{
			revision:       shaThird,
			expectedPassed: []string{},
			expectedErr:    fmt.Sprintf("GIT/GPG: Revision %s by 'ignored': signed with disallowed key 'KEY_OF_THIRD'", shaThird),
			expectedLsArgs: []any{shaThird, true},
		},
		{
			revision:        tagFirst,
			expectedPassed:  []string{"GIT/GPG"},
			expectedTagArgs: []any{tagFirst},
			expectedLsArgs:  []any{shaFirst, true},
		},
		{
			revision:        tagSecond,
			expectedPassed:  []string{"GIT/GPG"},
			expectedTagArgs: []any{tagSecond},
			expectedLsArgs:  []any{shaSecond, true},
		},
		{
			revision:       tagThird,
			expectedPassed: []string{},
			expectedErr: fmt.Sprintf(`GIT/GPG: Revision %s by 'ignored': signed with disallowed key 'KEY_OF_THIRD'
GIT/GPG: Revision %s by 'ignored': signed with disallowed key 'KEY_OF_THIRD'`, tagThird, shaThird),
			expectedTagArgs: []any{tagThird},
			expectedLsArgs:  []any{shaThird, true},
		},
	}

	for _, test := range tests {
		t.Run("verify "+test.revision, func(t *testing.T) {
			// Given repo with tagged commit
			gitClient := &gitmocks.Client{}
			gitClient.EXPECT().CommitSHA().RunAndReturn(func() (string, error) {
				if sha, ok := tag2commit[test.revision]; ok {
					return sha, nil
				}
				return test.revision, nil
			})
			gitClient.EXPECT().IsAnnotatedTag(mock.Anything).RunAndReturn(func(revision string) bool {
				return strings.HasPrefix(revision, "tag-")
			})
			gitClient.EXPECT().TagSignature(mock.Anything).RunAndReturn(func(tagRevision string) (*git.RevisionSignatureInfo, error) {
				keyId := ""
				switch tagRevision {
				case tagFirst:
					keyId = "KEY_OF_FIRST"
				case tagSecond:
					keyId = "KEY_OF_SECOND"
				case tagThird:
					keyId = "KEY_OF_THIRD"
				default:
					require.Fail(t, "tag revision '"+tagRevision+"' not recognized")
				}
				return &git.RevisionSignatureInfo{
					Revision: tagRevision, VerificationResult: git.GPGVerificationResultGood, SignatureKeyID: keyId, Date: "ignored", AuthorIdentity: "ignored",
				}, nil
			})
			gitClient.EXPECT().LsSignatures(mock.Anything, mock.Anything).RunAndReturn(func(revision string, deep bool) (info []git.RevisionSignatureInfo, err error) {
				// Return current revision info if not `deep`, return with all ancestry otherwise.
				if revision == shaThird || revision == tagThird {
					info = append(info, rsi3)
					if !deep {
						return info, err
					}
				}
				if revision == shaSecond || revision == tagSecond {
					info = append(info, rsi2)
					if !deep {
						return info, err
					}
				}
				if revision == shaFirst || revision == tagFirst {
					info = append(info, rsi1)
				}

				if len(info) == 0 {
					// Expected one of the 6
					panic("unknown revision " + revision)
				}

				return info, err
			})

			logger := utilTest.LogHook{}
			logrus.AddHook(&logger)
			t.Cleanup(logger.CleanupHook)

			// When using head mode
			gpgWithTag := SourceIntegrityGitPolicyGPG{
				Mode: SourceIntegrityGitPolicyGPGModeStrict,
				Keys: []string{"KEY_OF_FIRST", "KEY_OF_SECOND"},
			}
			// And verifying a given revision
			result, err := gpgWithTag.verify(gitClient, test.revision)
			require.NoError(t, err)
			// Then it is checked and valid
			err = result.Error()
			if test.expectedErr == "" {
				require.NoError(t, err)
				assert.True(t, result.IsValid())
			} else {
				require.Error(t, err)
				assert.Equal(t, test.expectedErr, err.Error())
			}
			assert.Equal(t, test.expectedPassed, result.PassedChecks())

			// verify if only the intended interaction happened
			if len(test.expectedTagArgs) > 0 {
				gitClient.AssertCalled(t, "TagSignature", test.expectedTagArgs...)
			} else {
				gitClient.AssertNotCalled(t, "TagSignature")
			}
			if len(test.expectedLsArgs) > 0 {
				gitClient.AssertCalled(t, "LsSignatures", test.expectedLsArgs...)
			} else {
				gitClient.AssertNotCalled(t, "LsSignatures")
			}

			assert.Empty(t, logger.GetEntries())
		})
	}
}

// TODO LS Revisions called with unknown commit/tag
