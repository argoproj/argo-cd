package git

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/common"
)

type gpgReadyRepo struct {
	t       *testing.T
	git     Client
	gpgHome string
}

func newGPGReadyRepo(t *testing.T) *gpgReadyRepo {
	t.Helper()
	repo := &gpgReadyRepo{t, nil, t.TempDir()}

	t.Setenv(common.EnvGnuPGHome, repo.gpgHome)

	err := os.Chmod(repo.gpgHome, 0o700)
	require.NoError(t, err)

	repo.git, err = NewClient("https://fake.url/org/repo.git", NopCreds{}, true, false, "", "")
	require.NoError(t, err)
	_ = os.RemoveAll(repo.git.Root())
	t.Cleanup(func() {
		_ = os.RemoveAll(repo.git.Root())
	})
	err = repo.git.Init()
	require.NoError(t, err)

	require.NoError(t, repo.cmd("checkout", "-b", "main"))
	repo.setUser("Test User", "test@example.com")

	return repo
}

func (g *gpgReadyRepo) setUser(name string, email string) {
	require.NoError(g.t, g.cmd("config", "--local", "user.name", name))
	require.NoError(g.t, g.cmd("config", "--local", "user.email", email))
}

func (g *gpgReadyRepo) generateGPGKey(name string) (keyID string) {
	g.t.Helper()
	keyInput := fmt.Sprintf(`%%echo Generating test key
Key-Type: RSA
Key-Length: 2048
Name-Real: %s User
Name-Email: %s@example.com
Expire-Date: 0
%%no-protection
%%commit
%%echo Done`, name, name)

	cmd := exec.CommandContext(g.t.Context(), "gpg", "--batch", "--generate-key", "--homedir", g.gpgHome)
	cmd.Stdin = strings.NewReader(keyInput)
	out, err := cmd.CombinedOutput()
	require.NoError(g.t, err, "gpg key generation failed: %s", out)

	cmd = exec.CommandContext(g.t.Context(), "gpg", "--list-keys", "--with-colons", "--homedir", g.gpgHome)
	out, err = cmd.Output()
	require.NoError(g.t, err)

	// Parse output to get key ID
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "pub:") {
			fields := strings.Split(line, ":")
			if len(fields) > 4 {
				keyID = fields[4]
				// Loop even after found intentionally, expected the newest key will be the last one
			}
		}
	}
	require.NotEmpty(g.t, keyID, "failed to get GPG key ID")

	return keyID
}

func (g *gpgReadyRepo) revokeGPGKey(keyID string) {
	cmd := exec.CommandContext(
		g.t.Context(),
		"gpg", "--batch",
		"--command-fd=0", "--status-fd=1",
		"--homedir", g.gpgHome,
		"--edit-key", keyID,
	)
	// gpg is so not meant to be used from automation. This is why `--command-fd=0 --status-fd=1` is needed
	cmd.Stdin = strings.NewReader(`revkey
y
2
Automated revocation

y
save
`)
	out, err := cmd.CombinedOutput()
	require.NoError(g.t, err, "gpg key revocation generation failed: %s", out)
}

func (g *gpgReadyRepo) cmd(args ...string) error {
	cmd := exec.CommandContext(g.t.Context(), "git", args...)
	cmd.Dir = g.git.Root()
	cmd.Env = append(cmd.Env, "GNUPGHOME="+g.gpgHome)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (g *gpgReadyRepo) commitSHA() string {
	sha, err := g.git.CommitSHA()
	require.NoError(g.t, err)
	return sha
}

func (g *gpgReadyRepo) assertSignedAs(revision string, expectedSign ...string) {
	info, err := g.git.LsSignatures(revision, true)
	require.NoError(g.t, err)

	var actualSign []string
	for _, record := range info {
		actualSign = append(actualSign, string(record.VerificationResult)+"="+record.SignatureKeyID)
	}
	assert.Equal(g.t, expectedSign, actualSign)
}

func Test_LsSignatures_SignedAndMerged(t *testing.T) {
	repo := newGPGReadyRepo(t)
	mainKeyID := repo.generateGPGKey("main")
	otherKeyID := repo.generateGPGKey("other")

	require.NoError(t, repo.cmd("commit", "--allow-empty", "--no-edit", "--message=root", "--gpg-sign="+mainKeyID))

	require.NoError(t, repo.cmd("checkout", "-b", "left", "main"))
	require.NoError(t, repo.cmd("commit", "--allow-empty", "--no-edit", "--message=right", "--gpg-sign="+otherKeyID))

	require.NoError(t, repo.cmd("checkout", "main"))
	require.NoError(t, repo.cmd("commit", "--allow-empty", "--no-edit", "--message=left", "--gpg-sign="+mainKeyID))

	require.NoError(t, repo.cmd("merge", "left", "--no-edit", "--message=merge", "--gpg-sign="+mainKeyID))

	require.NoError(t, repo.cmd("commit", "--allow-empty", "--no-edit", "--message=main", "--gpg-sign="+mainKeyID))

	tip := repo.commitSHA()

	repo.assertSignedAs(
		tip,
		"signed="+mainKeyID,                       // main
		"signed="+mainKeyID,                       // merge
		"signed="+mainKeyID, "signed="+otherKeyID, // left + right
		"signed="+mainKeyID, // root
	)

	repo.revokeGPGKey(mainKeyID)
	repo.assertSignedAs(
		tip,
		"signed with revoked key="+mainKeyID,                       // main
		"signed with revoked key="+mainKeyID,                       // merge
		"signed with revoked key="+mainKeyID, "signed="+otherKeyID, // left + right
		"signed with revoked key="+mainKeyID, // root
	)
}

func Test_LsSignatures_Sealed_linear(t *testing.T) {
	repo := newGPGReadyRepo(t)
	trustedKeyID := repo.generateGPGKey("trusted")

	require.NoError(t, repo.cmd("commit", "--allow-empty", "--no-edit", "--message=signed", "--gpg-sign="+trustedKeyID))
	repo.assertSignedAs(repo.commitSHA(), "signed="+trustedKeyID)
	require.NoError(t, repo.cmd("commit", "--allow-empty", "--no-edit", "--message=unsigned"))
	repo.assertSignedAs(repo.commitSHA(), "unsigned=", "signed="+trustedKeyID)
	require.NoError(t, repo.cmd("commit", "--allow-empty", "--no-edit", "--message=signed", "--gpg-sign="+trustedKeyID))
	repo.assertSignedAs(repo.commitSHA(), "signed="+trustedKeyID, "unsigned=", "signed="+trustedKeyID)
	require.NoError(t, repo.cmd("commit", "--allow-empty", "--no-edit", "--message=seal", "--gpg-sign="+trustedKeyID, "--trailer=ArgoCD-gpg-seal: XXX"))
	repo.assertSignedAs(repo.commitSHA(), "signed="+trustedKeyID)
	require.NoError(t, repo.cmd("commit", "--allow-empty", "--no-edit", "--message=signed", "--gpg-sign="+trustedKeyID))
	repo.assertSignedAs(repo.commitSHA(), "signed="+trustedKeyID, "signed="+trustedKeyID)
	require.NoError(t, repo.cmd("commit", "--allow-empty", "--no-edit", "--message=unsigned"))
	repo.assertSignedAs(repo.commitSHA(), "unsigned=", "signed="+trustedKeyID, "signed="+trustedKeyID)
	require.NoError(t, repo.cmd("commit", "--allow-empty", "--no-edit", "--message=seal", "--gpg-sign="+trustedKeyID, "--trailer=ArgoCD-gpg-seal: XXX"))
	repo.assertSignedAs(repo.commitSHA(), "signed="+trustedKeyID)
}

func Test_LsSignatures_UnsignedSealedCommitDoesNotStopHistorySearch(t *testing.T) {
	// The seal commit must be signed and trusted. When it is not, it is not considered a seal commit and the history is searched further.
	repo := newGPGReadyRepo(t)
	trustedKeyID := repo.generateGPGKey("trusted")

	// Will not be listed
	require.NoError(t, repo.cmd("commit", "--allow-empty", "--no-edit", "--message=unsigned init"))
	// The seal commit we stop on
	require.NoError(t, repo.cmd("commit", "--allow-empty", "--no-edit", "--message=signed seal", "--trailer=ArgoCD-gpg-seal: XXX", "--gpg-sign="+trustedKeyID))
	signedSealSha := repo.commitSHA()
	require.NoError(t, repo.cmd("commit", "--allow-empty", "--no-edit", "--message=unsigned past"))
	unsignedPastSha := repo.commitSHA()
	// The wannabe seal commit we ignore - unsigned
	require.NoError(t, repo.cmd("commit", "--allow-empty", "--no-edit", "--message=unsigned seal", "--trailer=ArgoCD-gpg-seal: XXX"))
	unsignedSealSha := repo.commitSHA()
	require.NoError(t, repo.cmd("commit", "--allow-empty", "--no-edit", "--message=signed", "--gpg-sign="+trustedKeyID))
	signedSha := repo.commitSHA()

	info, err := repo.git.LsSignatures(signedSha, true)
	require.NoError(t, err)

	assert.Len(t, info, 4)
	assert.Equal(t, GPGVerificationResultGood, info[0].VerificationResult)
	assert.Equal(t, signedSha, info[0].Revision)

	assert.Equal(t, GPGVerificationResultUnsigned, info[1].VerificationResult)
	assert.Equal(t, unsignedSealSha, info[1].Revision)

	assert.Equal(t, GPGVerificationResultUnsigned, info[2].VerificationResult)
	assert.Equal(t, unsignedPastSha, info[2].Revision)

	assert.Equal(t, GPGVerificationResultGood, info[3].VerificationResult)
	assert.Equal(t, signedSealSha, info[3].Revision)
}

func Test_SignedTag(t *testing.T) {
	repo := newGPGReadyRepo(t)
	commitKeyId := repo.generateGPGKey("commit gpg")
	tagKeyId := repo.generateGPGKey("tag gpg")

	require.NoError(t, repo.cmd("commit", "--allow-empty", "--message=unsigned"))
	require.NoError(t, repo.cmd("commit", "--allow-empty", "--message=signed", "--gpg-sign="+commitKeyId))

	// Tags are made by different user and key
	repo.setUser("Tagging user", "tagger@argo.io")
	require.NoError(t, repo.cmd("tag", "--message=signed tag", "--local-user="+tagKeyId, "1.0", "HEAD~1"))
	require.NoError(t, repo.cmd("tag", "--message=signed tag", "--local-user="+tagKeyId, "2.0", "HEAD"))
	require.NoError(t, repo.cmd("tag", "--message=unsigned tag", "dev", "HEAD"))

	info, err := repo.git.TagSignature("1.0")
	require.NoError(t, err)
	assert.Equal(t, "1.0", info.Revision)
	assert.Equal(t, GPGVerificationResultGood, info.VerificationResult)
	assert.Equal(t, tagKeyId, info.SignatureKeyID)
	assert.Equal(t, `Tagging user "<tagger@argo.io>"`, info.AuthorIdentity)

	info, err = repo.git.TagSignature("2.0")
	require.NoError(t, err)
	assert.Equal(t, "2.0", info.Revision)
	assert.Equal(t, GPGVerificationResultGood, info.VerificationResult)
	assert.Equal(t, tagKeyId, info.SignatureKeyID)
	assert.Equal(t, `Tagging user "<tagger@argo.io>"`, info.AuthorIdentity)

	info, err = repo.git.TagSignature("no-such-tag")
	require.ErrorContains(t, err, `no tag found: "no-such-tag"`)
	assert.Nil(t, info)

	info, err = repo.git.TagSignature("dev")
	require.NoError(t, err)
	assert.Equal(t, "dev", info.Revision)
	assert.Equal(t, GPGVerificationResultUnsigned, info.VerificationResult)
	assert.Empty(t, info.SignatureKeyID)
	assert.Equal(t, `Tagging user "<tagger@argo.io>"`, info.AuthorIdentity)
}

func Test_parseGpgSignStatus(t *testing.T) {
	testCases := []struct {
		cmdErr    error
		tagGpgOut string
		expError  string
		expResult GPGVerificationResult
		expKeyID  string
	}{
		{
			errors.New("fake"),
			"error: no signature found",
			"", GPGVerificationResultUnsigned, "",
		},
		{
			errors.New("fake"),
			"the unexpected have happened",
			"fake", "", "",
		},
		{
			nil,
			"Buahahaha!",
			"unexpected `git verify-tag --raw` output: \"Buahahaha!\"", "", "",
		},
		{
			nil,
			`[GNUPG:] NEWSIG
[GNUPG:] ERRSIG D56C4FCA57A46444 1 10 00 1763632400 9 EA459B49595CBE3FD1FBA303D56C4FCA57A46444
[GNUPG:] NO_PUBKEY D56C4FCA57A46444
[GNUPG:] FAILURE gpg-exit 33554433`,
			"", GPGVerificationResultMissingKey, "D56C4FCA57A46444",
		},
		{
			nil,
			`[GNUPG:] NEWSIG user17@argo.io
[GNUPG:] KEY_CONSIDERED D7E87AF6B99E64079FFECC029515ACB41E14E7F9 0
[GNUPG:] SIG_ID ES7wSYaAnVXVsRjW15LzE4TMp+U 2025-11-19 3671527729
[GNUPG:] GOODSIG 9515ACB41E14E7F9 User N17 <user17@argo.io>
[GNUPG:] VALIDSIG D7E87AF6B99E64079FFECC029515ACB41E14E7F9 2025-11-19 3671527729 0 4 0 1 10 00 D7E87AF6B99E64079FFECC029515ACB41E14E7F9
[GNUPG:] TRUST_ULTIMATE 0 pgp user17@argo.io`,
			"", GPGVerificationResultGood, "9515ACB41E14E7F9",
		},
	}

	for _, tt := range testCases {
		result, keyId, err := evaluateGpgSignStatus(tt.cmdErr, tt.tagGpgOut)

		if tt.expError != "" {
			require.Error(t, err)
			assert.Equal(t, tt.expError, err.Error())
		}
		assert.Equal(t, tt.expResult, result)
		assert.Equal(t, tt.expKeyID, keyId)
	}
}
