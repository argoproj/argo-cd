package sourceintegrity

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
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

func Test_GPGDisabledLogging(t *testing.T) {
	t.Setenv("ARGOCD_GPG_ENABLED", "false")

	si := &v1alpha1.SourceIntegrity{Git: &v1alpha1.SourceIntegrityGit{Policies: []*v1alpha1.SourceIntegrityGitPolicy{{
		Repos: []string{"*"},
		GPG: &v1alpha1.SourceIntegrityGitPolicyGPG{
			Mode: v1alpha1.SourceIntegrityGitPolicyGPGModeStrict,
			Keys: []string{"SOME_KEY_ID"},
		},
	}}}}

	logger := utilTest.LogHook{}
	logrus.AddHook(&logger)
	t.Cleanup(logger.CleanupHook)

	fun := lookupGit(si, "https://github.com/argoproj/argo-cd.git")
	assert.Equal(t, []string{"SourceIntegrity criteria for git+gpg declared, but it is turned off by ARGOCD_GPG_ENABLED"}, logger.GetEntries())
	assert.Nil(t, fun)

	// No logs on the second call
	logger.Entries = []logrus.Entry{}
	lookupGit(si, "https://github.com/argoproj/argo-cd-ext.git")
	assert.Equal(t, []string{}, logger.GetEntries())
	assert.Nil(t, fun)
}

func TestGPGUnknownMode(t *testing.T) {
	gitClient := &gitmocks.Client{}
	gitClient.EXPECT().IsAnnotatedTag(mock.Anything).Return(false)
	gitClient.EXPECT().CommitSHA().Return("DEADBEEF", nil)

	s := &v1alpha1.SourceIntegrityGitPolicyGPG{Mode: "foobar", Keys: []string{}}
	result, err := verify(s, gitClient, "https://github.com/argoproj/argo-cd.git")
	require.ErrorContains(t, err, `unknown GPG mode "foobar" configured for GIT source integrity`)
	assert.Nil(t, result)
}

func TestNullOrEmptyDoesNothing(t *testing.T) {
	repoURL := "https://github.com/argoproj/argo-cd"
	applicationSource := v1alpha1.ApplicationSource{RepoURL: repoURL}

	gitClient := &gitmocks.Client{}
	gitClient.EXPECT().RepoURL().Return(repoURL)

	tests := []struct {
		name   string
		si     *v1alpha1.SourceIntegrity
		logged []string
	}{
		{
			name:   "nil",
			si:     nil,
			logged: []string{},
		},
		{
			name:   "No GIT",
			si:     &v1alpha1.SourceIntegrity{}, // No Git or alternative specified
			logged: []string{},
		},
		{
			name: "No matching policy",
			si: &v1alpha1.SourceIntegrity{Git: &v1alpha1.SourceIntegrityGit{
				// No policies configured here
			}},
			logged: []string{},
		},
		{
			name: "Matching policy does nothing",
			si: &v1alpha1.SourceIntegrity{Git: &v1alpha1.SourceIntegrityGit{Policies: []*v1alpha1.SourceIntegrityGitPolicy{{
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

			assert.False(t, HasCriteria(tt.si, applicationSource))
			assert.Equal(t, tt.logged, logger.GetEntries())
		})
	}
}

func TestPolicyMatching(t *testing.T) {
	eitherOr := &v1alpha1.SourceIntegrityGitPolicy{
		Repos: []string{"https://github.com/group/either.git", "https://github.com/group/or.git"},
		GPG:   &v1alpha1.SourceIntegrityGitPolicyGPG{},
	}
	ignored := &v1alpha1.SourceIntegrityGitPolicy{
		Repos: []string{"https://github.com/group/ignored.git"},
		GPG: &v1alpha1.SourceIntegrityGitPolicyGPG{
			Mode: v1alpha1.SourceIntegrityGitPolicyGPGModeNone,
		},
	}
	group := &v1alpha1.SourceIntegrityGitPolicy{
		Repos: []string{"https://github.com/group/*"},
		GPG:   &v1alpha1.SourceIntegrityGitPolicyGPG{},
	}
	prefix := &v1alpha1.SourceIntegrityGitPolicy{
		Repos: []string{"https://github.com/group*"},
		GPG:   &v1alpha1.SourceIntegrityGitPolicyGPG{},
	}
	sig := &v1alpha1.SourceIntegrityGit{
		Policies: []*v1alpha1.SourceIntegrityGitPolicy{eitherOr, ignored, group, prefix},
	}

	p := func(ps ...*v1alpha1.SourceIntegrityGitPolicy) []*v1alpha1.SourceIntegrityGitPolicy { return ps }
	testCases := []struct {
		repo             string
		expectedPolicies []*v1alpha1.SourceIntegrityGitPolicy
		expectedLogs     []string
		expectedNoFunc   bool
	}{
		{
			repo:             "https://github.com/group/either.git",
			expectedPolicies: p(eitherOr, group, prefix),
			expectedLogs:     []string{"Multiple (3) git source integrity policies found for repo URL: https://github.com/group/either.git. Using the first matching one"},
		},
		{
			repo:             "https://github.com/group/or.git",
			expectedPolicies: p(eitherOr, group, prefix),
			expectedLogs:     []string{"Multiple (3) git source integrity policies found for repo URL: https://github.com/group/or.git. Using the first matching one"},
		},
		{
			repo:             "https://github.com/group/fork.git",
			expectedPolicies: p(group, prefix),
			expectedLogs:     []string{"Multiple (2) git source integrity policies found for repo URL: https://github.com/group/fork.git. Using the first matching one"},
		},
		{
			repo:             "https://github.com/grouplette/main.git",
			expectedPolicies: p(prefix),
			expectedLogs:     []string{},
		},
		{
			repo:             "https://gitlab.com/foo/bar.git",
			expectedPolicies: p(),
			expectedLogs:     []string{"No git source integrity policies found for repo URL: https://gitlab.com/foo/bar.git"},
			expectedNoFunc:   true,
		},
		{
			repo:             "https://github.com/group/ignored.git",
			expectedPolicies: p(ignored, group, prefix),
			expectedLogs:     []string{"Multiple (3) git source integrity policies found for repo URL: https://github.com/group/ignored.git. Using the first matching one"},
			expectedNoFunc:   true,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.repo, func(t *testing.T) {
			actual := findMatchingGitPolicies(sig, tt.repo)

			assert.Equal(t, tt.expectedPolicies, actual)

			hook := utilTest.NewLogHook(logrus.InfoLevel)
			logrus.AddHook(hook)
			defer hook.CleanupHook()
			si := &v1alpha1.SourceIntegrity{Git: sig}
			forGitFunc := lookupGit(si, tt.repo)
			if tt.expectedNoFunc {
				assert.Nil(t, forGitFunc)
			} else {
				assert.NotNil(t, forGitFunc)
			}
			assert.Equal(t, tt.expectedLogs, hook.GetEntries())
		})
	}
}

// Verify that when a user has configured the full fingerprint, it is still accepted
func TestComparingWithGPGFingerprint(t *testing.T) {
	const shortKey = "D56C4FCA57A46444"
	const fingerprint = "01234567890123456789abcd" + shortKey
	require.True(t, IsShortKeyID(shortKey))
	require.True(t, IsLongKeyID(fingerprint))

	gitClient := &gitmocks.Client{}
	gitClient.EXPECT().IsAnnotatedTag(mock.Anything).Return(true)
	gitClient.EXPECT().CommitSHA().Return("ignored", nil)
	gitClient.EXPECT().TagSignature(mock.Anything).Return(&git.RevisionSignatureInfo{
		Revision: "1.0", VerificationResult: git.GPGVerificationResultGood, SignatureKeyID: shortKey, Date: "ignored", AuthorIdentity: "ignored",
	}, nil)

	gpgWithTag := &v1alpha1.SourceIntegrityGitPolicyGPG{Mode: v1alpha1.SourceIntegrityGitPolicyGPGModeHead, Keys: []string{fingerprint}}
	// And verifying a given revision
	result, err := verify(gpgWithTag, gitClient, "1.0")
	require.NoError(t, err)

	assert.True(t, result.IsValid())
	assert.NoError(t, result.AsError())
}

func TestGPGHeadValid(t *testing.T) {
	const sha = "0c7a9c3f939c1f19b518bcdd11e2fce9703c4901"
	const tag = "tag"
	const keyId = "4cfe068f80b1681b"
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
			// Given repo with a tagged commit
			gitClient := &gitmocks.Client{}
			gitClient.EXPECT().CommitSHA().RunAndReturn(func() (string, error) { return sha, nil })
			gitClient.EXPECT().IsAnnotatedTag(mock.Anything).RunAndReturn(func(s string) bool { return tag == s })
			gitClient.EXPECT().LsSignatures(mock.Anything, mock.Anything).RunAndReturn(func(revision string, _ bool) ([]git.RevisionSignatureInfo, error) {
				return []git.RevisionSignatureInfo{{
					Revision: revision, VerificationResult: git.GPGVerificationResultGood, SignatureKeyID: keyId, Date: "ignored", AuthorIdentity: "ignored",
				}}, nil
			})
			gitClient.EXPECT().TagSignature(mock.Anything).RunAndReturn(func(revision string) (*git.RevisionSignatureInfo, error) {
				return &git.RevisionSignatureInfo{
					Revision: revision, VerificationResult: git.GPGVerificationResultGood, SignatureKeyID: keyId, Date: "ignored", AuthorIdentity: "ignored",
				}, nil
			})

			logger := utilTest.LogHook{}
			logrus.AddHook(&logger)
			t.Cleanup(logger.CleanupHook)

			// When using head mode
			gpgWithTag := &v1alpha1.SourceIntegrityGitPolicyGPG{Mode: v1alpha1.SourceIntegrityGitPolicyGPGModeHead, Keys: []string{keyId, "0000000000000000"}}
			// And verifying a given revision
			result, err := verify(gpgWithTag, gitClient, test.revision)
			require.NoError(t, err)
			// Then it is checked and valid
			assert.True(t, result.IsValid())
			assert.Equal(t, []string{"GIT/GPG"}, result.PassedChecks())
			test.check(gitClient, logger)
			require.NoError(t, result.AsError())
		})
	}
}

func TestDescribeProblems(t *testing.T) {
	const r = "aafc9e88599f24802b113b6278e42eaadda32cd6"
	const a = "Commit Author <nereply@acme.com>"
	const kGood = "AAAAAAAAAAAAAAAA"
	const kOk = "BBBBBBBBBBBBBBB"
	policy := v1alpha1.SourceIntegrityGitPolicyGPG{Keys: []string{kGood, kOk}}

	sig := func(key string, result git.GPGVerificationResult) git.RevisionSignatureInfo {
		return git.RevisionSignatureInfo{
			Revision:           r,
			VerificationResult: result,
			SignatureKeyID:     key,
			AuthorIdentity:     a,
		}
	}

	tests := []struct {
		name     string
		gpg      *v1alpha1.SourceIntegrityGitPolicyGPG
		sigs     []git.RevisionSignatureInfo
		expected []string
	}{
		{
			name: "report only problems",
			gpg:  &policy,
			sigs: []git.RevisionSignatureInfo{
				sig("bad", git.GPGVerificationResultRevokedKey),
				sig(kGood, git.GPGVerificationResultGood),
				sig("also_bad", git.GPGVerificationResultUntrusted),
			},
			expected: []string{
				"Failed verifying revision " + r + " by '" + a + "': signed with revoked key (key_id=bad)",
				"Failed verifying revision " + r + " by '" + a + "': signed with untrusted key (key_id=also_bad)",
			},
		},
		{
			name: "collapse problems of the same key",
			gpg:  &policy,
			sigs: []git.RevisionSignatureInfo{
				sig("bad", git.GPGVerificationResultRevokedKey),
				sig(kGood, git.GPGVerificationResultGood),
				sig("also_bad", git.GPGVerificationResultUntrusted),
				sig("bad", git.GPGVerificationResultRevokedKey),
			},
			expected: []string{
				"Failed verifying revision " + r + " by '" + a + "': signed with revoked key (key_id=bad)",
				"Failed verifying revision " + r + " by '" + a + "': signed with untrusted key (key_id=also_bad)",
			},
		},
		{
			name: "do not collapse unsigned commits, as they can differ by author",
			gpg:  &policy,
			sigs: []git.RevisionSignatureInfo{
				sig("", git.GPGVerificationResultUnsigned),
				sig("", git.GPGVerificationResultUnsigned),
				sig("", git.GPGVerificationResultUnsigned),
			},
			expected: []string{
				"Failed verifying revision " + r + " by '" + a + "': unsigned (key_id=)",
				"Failed verifying revision " + r + " by '" + a + "': unsigned (key_id=)",
				"Failed verifying revision " + r + " by '" + a + "': unsigned (key_id=)",
			},
		},
		{
			name: "Report first ten problems only",
			gpg:  &policy,
			sigs: []git.RevisionSignatureInfo{
				sig("revoked", git.GPGVerificationResultRevokedKey),
				sig("", git.GPGVerificationResultUnsigned),
				sig("untrusted", git.GPGVerificationResultUntrusted),
				sig("missing", git.GPGVerificationResultMissingKey),
				sig("expired_key", git.GPGVerificationResultExpiredKey),
				sig("expired_sig", git.GPGVerificationResultExpiredSignature),
				sig("bad", git.GPGVerificationResultBad),
				sig("also_bad", git.GPGVerificationResultBad),
				sig("more_bad", git.GPGVerificationResultBad),
				sig("outright_terrible", git.GPGVerificationResultBad),
				// the rest is cut off
				sig("OMG", git.GPGVerificationResultBad),
				sig("nope", git.GPGVerificationResultBad),
				sig("you_gott_be_kidding_me", git.GPGVerificationResultBad),
			},
			expected: []string{
				"Failed verifying revision " + r + " by '" + a + "': signed with revoked key (key_id=revoked)",
				"Failed verifying revision " + r + " by '" + a + "': unsigned (key_id=)",
				"Failed verifying revision " + r + " by '" + a + "': signed with untrusted key (key_id=untrusted)",
				"Failed verifying revision " + r + " by '" + a + "': signed with key not in keyring (key_id=missing)",
				"Failed verifying revision " + r + " by '" + a + "': signed with expired key (key_id=expired_key)",
				"Failed verifying revision " + r + " by '" + a + "': expired signature (key_id=expired_sig)",
				"Failed verifying revision " + r + " by '" + a + "': bad signature (key_id=bad)",
				"Failed verifying revision " + r + " by '" + a + "': bad signature (key_id=also_bad)",
				"Failed verifying revision " + r + " by '" + a + "': bad signature (key_id=more_bad)",
				"Failed verifying revision " + r + " by '" + a + "': bad signature (key_id=outright_terrible)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			problems := describeProblems(tt.gpg, tt.sigs)
			assert.Equal(t, tt.expected, problems)
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
	const keyOfThird = "9c698b961c1088db"
	const keyOfSecond = "f4b9db205449e1d9"
	const keyOfFirst = "92bfcec2e8161558"
	rsi3 := git.RevisionSignatureInfo{
		Revision: shaThird, VerificationResult: git.GPGVerificationResultGood,
		SignatureKeyID: keyOfThird, Date: "ignored", AuthorIdentity: "ignored",
	}
	rsi2 := git.RevisionSignatureInfo{
		Revision: shaSecond, VerificationResult: git.GPGVerificationResultGood,
		SignatureKeyID: keyOfSecond, Date: "ignored", AuthorIdentity: "ignored",
	}
	rsi1 := git.RevisionSignatureInfo{
		Revision: shaFirst, VerificationResult: git.GPGVerificationResultGood,
		SignatureKeyID: keyOfFirst, Date: "ignored", AuthorIdentity: "ignored",
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
			expectedErr:    fmt.Sprintf("GIT/GPG: Failed verifying revision %s by 'ignored': signed with unallowed key (key_id=%s)", shaThird, keyOfThird),
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
			expectedErr: fmt.Sprintf(`GIT/GPG: Failed verifying revision %s by 'ignored': signed with unallowed key (key_id=%s)
GIT/GPG: Failed verifying revision %s by 'ignored': signed with unallowed key (key_id=%s)`, tagThird, keyOfThird, shaThird, keyOfThird),
			expectedTagArgs: []any{tagThird},
			expectedLsArgs:  []any{shaThird, true},
		},
	}

	for _, test := range tests {
		t.Run("verify "+test.revision, func(t *testing.T) {
			// Given repo with a tagged commit
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
					keyId = keyOfFirst
				case tagSecond:
					keyId = keyOfSecond
				case tagThird:
					keyId = keyOfThird
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
			gpgWithTag := &v1alpha1.SourceIntegrityGitPolicyGPG{
				Mode: v1alpha1.SourceIntegrityGitPolicyGPGModeStrict,
				Keys: []string{keyOfFirst, keyOfSecond},
			}
			// And verifying a given revision
			result, err := verify(gpgWithTag, gitClient, test.revision)
			require.NoError(t, err)
			// Then it is checked and valid
			err = result.AsError()
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
