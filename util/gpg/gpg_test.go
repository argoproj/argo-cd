package gpg

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/test"
)

const (
	longKeyID  = "5DE3E0509C47EA3CF04A42D34AEE18F83AFDEB23"
	shortKeyID = "4AEE18F83AFDEB23"
)

var syncTestSources = map[string]string{
	"F7842A5CEAA9C0B1": "testdata/janedoe.asc",
	"FDC79815400D88A9": "testdata/johndoe.asc",
	"4AEE18F83AFDEB23": "testdata/github.asc",
}

// Helper function to create temporary GNUPGHOME
func initTempDir(t *testing.T) string {
	// Intentionally avoid using t.TempDir. That function creates really long paths, which can exceed the socket file
	// path length on some OSes. The GPG tests rely on sockets.
	p, err := os.MkdirTemp(os.TempDir(), "")
	if err != nil {
		panic(err)
	}
	fmt.Printf("-> Using %s as GNUPGHOME\n", p)
	t.Setenv(common.EnvGnuPGHome, p)
	t.Cleanup(func() {
		err := os.RemoveAll(p)
		if err != nil {
			panic(err)
		}
	})
	return p
}

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

func Test_GPG_InitializeGnuPG(t *testing.T) {
	p := initTempDir(t)

	// First run should initialize fine
	err := InitializeGnuPG()
	require.NoError(t, err)

	// We should have exactly one public key with ultimate trust (our own) in the keyring
	keys, err := GetInstalledPGPKeys(nil)
	require.NoError(t, err)
	assert.Len(t, keys, 1)
	assert.Equal(t, "ultimate", keys[0].Trust)

	// During unit-tests, we need to also kill gpg-agent so we can create a new key.
	// In real world scenario -- i.e. container crash -- gpg-agent is not running yet.
	cmd := exec.Command("gpgconf", "--kill", "gpg-agent")
	cmd.Env = []string{fmt.Sprintf("GNUPGHOME=%s", p)}
	err = cmd.Run()
	require.NoError(t, err)

	// Second run should not return error
	err = InitializeGnuPG()
	require.NoError(t, err)
	keys, err = GetInstalledPGPKeys(nil)
	require.NoError(t, err)
	assert.Len(t, keys, 1)
	assert.Equal(t, "ultimate", keys[0].Trust)

	t.Run("GNUPGHOME is a file", func(t *testing.T) {
		f, err := os.CreateTemp("", "gpg-test")
		require.NoError(t, err)
		defer os.Remove(f.Name())

		// we need to error out
		t.Setenv(common.EnvGnuPGHome, f.Name())
		err = InitializeGnuPG()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not point to a directory")
	})

	t.Run("Unaccessible GNUPGHOME", func(t *testing.T) {
		p := initTempDir(t)
		fp := fmt.Sprintf("%s/gpg", p)
		err = os.Mkdir(fp, 0o000)
		if err != nil {
			panic(err.Error())
		}
		if err != nil {
			panic(err.Error())
		}
		t.Setenv(common.EnvGnuPGHome, fp)
		err := InitializeGnuPG()
		require.Error(t, err)
		// Restore permissions so path can be deleted
		err = os.Chmod(fp, 0o700)
		if err != nil {
			panic(err.Error())
		}
	})

	t.Run("GNUPGHOME with too wide permissions", func(t *testing.T) {
		// We do not expect an error here, because of openshift's random UIDs that
		// forced us to use an emptyDir mount (#4127)
		p := initTempDir(t)
		err := os.Chmod(p, 0o777)
		if err != nil {
			panic(err.Error())
		}
		t.Setenv(common.EnvGnuPGHome, p)
		err = InitializeGnuPG()
		require.NoError(t, err)
	})
}

func Test_GPG_KeyManagement(t *testing.T) {
	initTempDir(t)

	err := InitializeGnuPG()
	require.NoError(t, err)

	// Import a single good key
	keys, err := ImportPGPKeys("testdata/github.asc")
	require.NoError(t, err)
	assert.Len(t, keys, 1)
	assert.Equal(t, "4AEE18F83AFDEB23", keys[0].KeyID)
	assert.Contains(t, keys[0].Owner, "noreply@github.com")
	assert.Equal(t, "unknown", keys[0].Trust)
	assert.Equal(t, "unknown", keys[0].SubType)

	kids := make([]string, 0)
	importedKeyId := keys[0].KeyID

	// We should have a total of 2 keys in the keyring now
	{
		keys, err := GetInstalledPGPKeys(nil)
		require.NoError(t, err)
		assert.Len(t, keys, 2)
	}

	// We should now have that key in our keyring with unknown trust (trustdb not updated)
	{
		keys, err := GetInstalledPGPKeys([]string{importedKeyId})
		require.NoError(t, err)
		assert.Len(t, keys, 1)
		assert.Equal(t, "4AEE18F83AFDEB23", keys[0].KeyID)
		assert.Contains(t, keys[0].Owner, "noreply@github.com")
		assert.Equal(t, "unknown", keys[0].Trust)
		assert.Equal(t, "rsa2048", keys[0].SubType)
		kids = append(kids, keys[0].Fingerprint)
	}

	assert.Len(t, kids, 1)

	// Set trust level for our key and check the result
	{
		err := SetPGPTrustLevelById(kids, "ultimate")
		require.NoError(t, err)
		keys, err := GetInstalledPGPKeys(kids)
		require.NoError(t, err)
		assert.Len(t, keys, 1)
		assert.Equal(t, kids[0], keys[0].Fingerprint)
		assert.Equal(t, "ultimate", keys[0].Trust)
	}

	// Import garbage - error expected
	keys, err = ImportPGPKeys("testdata/garbage.asc")
	require.Error(t, err)
	assert.Empty(t, keys)

	// We should still have a total of 2 keys in the keyring now
	{
		keys, err := GetInstalledPGPKeys(nil)
		require.NoError(t, err)
		assert.Len(t, keys, 2)
	}

	// Delete previously imported public key
	{
		err := DeletePGPKey(importedKeyId)
		require.NoError(t, err)
		keys, err := GetInstalledPGPKeys(nil)
		require.NoError(t, err)
		assert.Len(t, keys, 1)
	}

	// Delete non-existing key
	{
		err := DeletePGPKey(importedKeyId)
		require.Error(t, err)
	}

	// Import multiple keys
	{
		keys, err := ImportPGPKeys("testdata/multi.asc")
		require.NoError(t, err)
		assert.Len(t, keys, 2)
		assert.Contains(t, keys[0].Owner, "john.doe@example.com")
		assert.Contains(t, keys[1].Owner, "jane.doe@example.com")
	}

	// Check if they were really imported
	{
		keys, err := GetInstalledPGPKeys(nil)
		require.NoError(t, err)
		assert.Len(t, keys, 3)
	}
}

func Test_ImportPGPKeysFromString(t *testing.T) {
	initTempDir(t)

	err := InitializeGnuPG()
	require.NoError(t, err)

	// Import a single good key
	keys, err := ImportPGPKeysFromString(test.MustLoadFileToString("testdata/github.asc"))
	require.NoError(t, err)
	assert.Len(t, keys, 1)
	assert.Equal(t, "4AEE18F83AFDEB23", keys[0].KeyID)
	assert.Contains(t, keys[0].Owner, "noreply@github.com")
	assert.Equal(t, "unknown", keys[0].Trust)
	assert.Equal(t, "unknown", keys[0].SubType)
}

func Test_ValidateGPGKeysFromString(t *testing.T) {
	initTempDir(t)

	err := InitializeGnuPG()
	require.NoError(t, err)

	{
		keyData := test.MustLoadFileToString("testdata/github.asc")
		keys, err := ValidatePGPKeysFromString(keyData)
		require.NoError(t, err)
		assert.Len(t, keys, 1)
	}

	{
		keyData := test.MustLoadFileToString("testdata/multi.asc")
		keys, err := ValidatePGPKeysFromString(keyData)
		require.NoError(t, err)
		assert.Len(t, keys, 2)
	}
}

func Test_ValidateGPGKeys(t *testing.T) {
	initTempDir(t)

	err := InitializeGnuPG()
	require.NoError(t, err)

	// Validation good case - 1 key
	{
		keys, err := ValidatePGPKeys("testdata/github.asc")
		require.NoError(t, err)
		assert.Len(t, keys, 1)
		assert.Contains(t, keys, "4AEE18F83AFDEB23")
	}

	// Validation bad case
	{
		keys, err := ValidatePGPKeys("testdata/garbage.asc")
		require.Error(t, err)
		assert.Empty(t, keys)
	}

	// We should still have a total of 1 keys in the keyring now
	{
		keys, err := GetInstalledPGPKeys(nil)
		require.NoError(t, err)
		assert.Len(t, keys, 1)
	}
}

func Test_GPG_ParseGitCommitVerification(t *testing.T) {
	initTempDir(t)

	err := InitializeGnuPG()
	require.NoError(t, err)

	keys, err := ImportPGPKeys("testdata/github.asc")
	require.NoError(t, err)
	assert.Len(t, keys, 1)

	// Good case
	{
		c, err := os.ReadFile("testdata/good_signature.txt")
		if err != nil {
			panic(err.Error())
		}
		res := ParseGitCommitVerification(string(c))
		assert.Equal(t, "4AEE18F83AFDEB23", res.KeyID)
		assert.Equal(t, "RSA", res.Cipher)
		assert.Equal(t, "ultimate", res.Trust)
		assert.Equal(t, "Wed Feb 26 23:22:34 2020 CET", res.Date)
		assert.Equal(t, VerifyResultGood, res.Result)
	}

	// Signature with unknown key - considered invalid
	{
		c, err := os.ReadFile("testdata/unknown_signature1.txt")
		if err != nil {
			panic(err.Error())
		}
		res := ParseGitCommitVerification(string(c))
		assert.Equal(t, "4AEE18F83AFDEB23", res.KeyID)
		assert.Equal(t, "RSA", res.Cipher)
		assert.Equal(t, TrustUnknown, res.Trust)
		assert.Equal(t, "Mon Aug 26 20:59:48 2019 CEST", res.Date)
		assert.Equal(t, VerifyResultInvalid, res.Result)
	}

	// Signature with unknown key and additional fields - considered invalid
	{
		c, err := os.ReadFile("testdata/unknown_signature2.txt")
		if err != nil {
			panic(err.Error())
		}
		res := ParseGitCommitVerification(string(c))
		assert.Equal(t, "4AEE18F83AFDEB23", res.KeyID)
		assert.Equal(t, "RSA", res.Cipher)
		assert.Equal(t, TrustUnknown, res.Trust)
		assert.Equal(t, "Mon Aug 26 20:59:48 2019 CEST", res.Date)
		assert.Equal(t, VerifyResultInvalid, res.Result)
	}

	// Bad signature with known key
	{
		c, err := os.ReadFile("testdata/bad_signature_bad.txt")
		if err != nil {
			panic(err.Error())
		}
		res := ParseGitCommitVerification(string(c))
		assert.Equal(t, "4AEE18F83AFDEB23", res.KeyID)
		assert.Equal(t, "RSA", res.Cipher)
		assert.Equal(t, "ultimate", res.Trust)
		assert.Equal(t, "Wed Feb 26 23:22:34 2020 CET", res.Date)
		assert.Equal(t, VerifyResultBad, res.Result)
	}

	// Bad case: Manipulated/invalid clear text signature
	{
		c, err := os.ReadFile("testdata/bad_signature_manipulated.txt")
		if err != nil {
			panic(err.Error())
		}
		res := ParseGitCommitVerification(string(c))
		assert.Equal(t, VerifyResultUnknown, res.Result)
		assert.Contains(t, res.Message, "Could not parse output")
	}

	// Bad case: Incomplete signature data #1
	{
		c, err := os.ReadFile("testdata/bad_signature_preeof1.txt")
		if err != nil {
			panic(err.Error())
		}
		res := ParseGitCommitVerification(string(c))
		assert.Equal(t, VerifyResultUnknown, res.Result)
		assert.Contains(t, res.Message, "end-of-file")
	}

	// Bad case: Incomplete signature data #2
	{
		c, err := os.ReadFile("testdata/bad_signature_preeof2.txt")
		if err != nil {
			panic(err.Error())
		}
		res := ParseGitCommitVerification(string(c))
		assert.Equal(t, VerifyResultUnknown, res.Result)
		assert.Contains(t, res.Message, "end-of-file")
	}

	// Bad case: No signature data #1
	{
		c, err := os.ReadFile("testdata/bad_signature_nodata.txt")
		if err != nil {
			panic(err.Error())
		}
		res := ParseGitCommitVerification(string(c))
		assert.Equal(t, VerifyResultUnknown, res.Result)
		assert.Contains(t, res.Message, "no verification data found")
	}

	// Bad case: Malformed signature data #1
	{
		c, err := os.ReadFile("testdata/bad_signature_malformed1.txt")
		if err != nil {
			panic(err.Error())
		}
		res := ParseGitCommitVerification(string(c))
		assert.Equal(t, VerifyResultUnknown, res.Result)
		assert.Contains(t, res.Message, "no verification data found")
	}

	// Bad case: Malformed signature data #2
	{
		c, err := os.ReadFile("testdata/bad_signature_malformed2.txt")
		if err != nil {
			panic(err.Error())
		}
		res := ParseGitCommitVerification(string(c))
		assert.Equal(t, VerifyResultUnknown, res.Result)
		assert.Contains(t, res.Message, "Could not parse key ID")
	}

	// Bad case: Malformed signature data #3
	{
		c, err := os.ReadFile("testdata/bad_signature_malformed3.txt")
		if err != nil {
			panic(err.Error())
		}
		res := ParseGitCommitVerification(string(c))
		assert.Equal(t, VerifyResultUnknown, res.Result)
		assert.Contains(t, res.Message, "Could not parse result of verify")
	}

	// Bad case: Invalid key ID in signature
	{
		c, err := os.ReadFile("testdata/bad_signature_badkeyid.txt")
		if err != nil {
			panic(err.Error())
		}
		res := ParseGitCommitVerification(string(c))
		assert.Equal(t, VerifyResultUnknown, res.Result)
		assert.Contains(t, res.Message, "Invalid PGP key ID")
	}
}

func Test_GetGnuPGHomePath(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		t.Setenv(common.EnvGnuPGHome, "")
		p := common.GetGnuPGHomePath()
		assert.Equal(t, common.DefaultGnuPgHomePath, p)
	})

	t.Run("tempdir", func(t *testing.T) {
		t.Setenv(common.EnvGnuPGHome, "/tmp/gpghome")
		p := common.GetGnuPGHomePath()
		assert.Equal(t, "/tmp/gpghome", p)
	})
}

func Test_KeyID(t *testing.T) {
	// Good case - long key ID (aka fingerprint) to short key ID
	{
		res := KeyID(longKeyID)
		assert.Equal(t, shortKeyID, res)
	}
	// Good case - short key ID remains same
	{
		res := KeyID(shortKeyID)
		assert.Equal(t, shortKeyID, res)
	}
	// Bad case - key ID too short
	{
		keyID := "AEE18F83AFDEB23"
		res := KeyID(keyID)
		assert.Empty(t, res)
	}
	// Bad case - key ID too long
	{
		keyID := "5DE3E0509C47EA3CF04A42D34AEE18F83AFDEB2323"
		res := KeyID(keyID)
		assert.Empty(t, res)
	}
	// Bad case - right length, but not hex string
	{
		keyID := "abcdefghijklmn"
		res := KeyID(keyID)
		assert.Empty(t, res)
	}
}

func Test_IsShortKeyID(t *testing.T) {
	assert.True(t, IsShortKeyID(shortKeyID))
	assert.False(t, IsShortKeyID(longKeyID))
	assert.False(t, IsShortKeyID("ab"))
}

func Test_IsLongKeyID(t *testing.T) {
	assert.True(t, IsLongKeyID(longKeyID))
	assert.False(t, IsLongKeyID(shortKeyID))
	assert.False(t, IsLongKeyID(longKeyID+"a"))
}

func Test_isHexString(t *testing.T) {
	assert.True(t, isHexString("ab0099"))
	assert.True(t, isHexString("AB0099"))
	assert.False(t, isHexString("foobar"))
}

func Test_IsSecretKey(t *testing.T) {
	initTempDir(t)

	// First run should initialize fine
	err := InitializeGnuPG()
	require.NoError(t, err)

	// We should have exactly one public key with ultimate trust (our own) in the keyring
	keys, err := GetInstalledPGPKeys(nil)
	require.NoError(t, err)
	assert.Len(t, keys, 1)
	assert.Equal(t, "ultimate", keys[0].Trust)

	{
		secret, err := IsSecretKey(keys[0].KeyID)
		require.NoError(t, err)
		assert.True(t, secret)
	}

	{
		secret, err := IsSecretKey("invalid")
		require.NoError(t, err)
		assert.False(t, secret)
	}
}

func Test_SyncKeyRingFromDirectory(t *testing.T) {
	initTempDir(t)

	// First run should initialize fine
	err := InitializeGnuPG()
	require.NoError(t, err)

	tempDir := t.TempDir()

	{
		new, removed, err := SyncKeyRingFromDirectory(tempDir)
		require.NoError(t, err)
		assert.Empty(t, new)
		assert.Empty(t, removed)
	}

	{
		for k, v := range syncTestSources {
			src, err := os.Open(v)
			if err != nil {
				panic(err.Error())
			}
			defer src.Close()
			dst, err := os.Create(path.Join(tempDir, k))
			if err != nil {
				panic(err.Error())
			}
			defer dst.Close()
			_, err = io.Copy(dst, src)
			if err != nil {
				panic(err.Error())
			}
			dst.Close()
		}

		new, removed, err := SyncKeyRingFromDirectory(tempDir)
		require.NoError(t, err)
		assert.Len(t, new, 3)
		assert.Empty(t, removed)

		installed, err := GetInstalledPGPKeys(new)
		require.NoError(t, err)
		for _, k := range installed {
			assert.Contains(t, new, k.KeyID)
		}
	}

	{
		err := os.Remove(path.Join(tempDir, "4AEE18F83AFDEB23"))
		if err != nil {
			panic(err.Error())
		}
		new, removed, err := SyncKeyRingFromDirectory(tempDir)
		require.NoError(t, err)
		assert.Empty(t, new)
		assert.Len(t, removed, 1)

		installed, err := GetInstalledPGPKeys(new)
		require.NoError(t, err)
		for _, k := range installed {
			assert.NotEqual(t, k.KeyID, removed[0])
		}
	}
}
