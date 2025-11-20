package git

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type gpgReadyRepo struct {
	t       *testing.T
	git     Client
	gpgHome string
}

func newGPGReadyRepo(t *testing.T) *gpgReadyRepo {
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

	cmd := exec.Command("gpg", "--batch", "--generate-key", "--homedir", g.gpgHome)
	cmd.Stdin = strings.NewReader(keyInput)
	out, err := cmd.CombinedOutput()
	require.NoError(g.t, err, "gpg key generation failed: %s", out)

	cmd = exec.Command("gpg", "--list-keys", "--with-colons", "--homedir", g.gpgHome)
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
	//time.Sleep(1 * time.Hour)
	cmd := exec.Command(
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

	repo.assertSignedAs(
		repo.commitSHA(),
		"signed="+mainKeyID,                       // main
		"signed="+mainKeyID,                       // merge
		"signed="+mainKeyID, "signed="+otherKeyID, // left + right
		"signed="+mainKeyID, // root
	)

	repo.revokeGPGKey(mainKeyID)
	repo.assertSignedAs(
		repo.commitSHA(),
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

	info, err = repo.git.TagSignature("dev")
	require.NoError(t, err)
	assert.Equal(t, "dev", info.Revision)
	assert.Equal(t, GPGVerificationResultUnsigned, info.VerificationResult)
	assert.Equal(t, "", info.SignatureKeyID)
	assert.Equal(t, `Tagging user "<tagger@argo.io>"`, info.AuthorIdentity)
}
