package gpg

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Helper function to create temporary GNUPGHOME
func initTempDir() string {
	p, err := ioutil.TempDir("", "gpg-test")
	if err != nil {
		// makes no sense to continue test without temp dir
		panic(err.Error())
	}
	fmt.Printf("-> Using %s as GNUPGHOME\n", p)
	os.Setenv("GNUPGHOME", p)
	return p
}

func Test_GPG_InitializeGnuPG(t *testing.T) {
	p := initTempDir()
	defer os.RemoveAll(p)

	// First run should initialize fine
	err := InitializeGnuPG()
	assert.NoError(t, err)

	// We should have exactly one public key with ultimate trust (our own) in the keyring
	keys, err := GetInstalledPGPKeys(nil)
	assert.NoError(t, err)
	assert.Len(t, keys, 1)
	assert.Equal(t, keys[0].Trust, "ultimate")

	// Second run should return error
	err = InitializeGnuPG()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already initialized")

	// GNUPGHOME is a file - we need to error out
	f, err := ioutil.TempFile("", "gpg-test")
	assert.NoError(t, err)
	defer os.Remove(f.Name())

	os.Setenv("GNUPGHOME", f.Name())
	err = InitializeGnuPG()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not point to a directory")

	// Unaccessible GNUPGHOME
	p = initTempDir()
	defer os.RemoveAll(p)
	fp := fmt.Sprintf("%s/gpg", p)
	err = os.Mkdir(fp, 0000)
	if err != nil {
		panic(err.Error())
	}
	if err != nil {
		panic(err.Error())
	}
	os.Setenv("GNUPGHOME", fp)
	err = InitializeGnuPG()
	assert.Error(t, err)
	// Restore permissions so path can be deleted
	os.Chmod(fp, 0700)

	// GNUPGHOME with too wide permissions
	p = initTempDir()
	defer os.RemoveAll(p)
	err = os.Chmod(p, 0777)
	if err != nil {
		panic(err.Error())
	}
	os.Setenv("GNUPGHOME", p)
	err = InitializeGnuPG()
	assert.Error(t, err)

}

func Test_GPG_KeyManagement(t *testing.T) {
	p := initTempDir()
	defer os.RemoveAll(p)

	err := InitializeGnuPG()
	assert.NoError(t, err)

	// Import a single good key
	keys, err := ImportPGPKeys("testdata/github.asc")
	assert.NoError(t, err)
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
		assert.NoError(t, err)
		assert.Len(t, keys, 2)
	}

	// We should now have that key in our keyring with unknown trust (trustdb not updated)
	{
		keys, err := GetInstalledPGPKeys([]string{importedKeyId})
		assert.NoError(t, err)
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
		assert.NoError(t, err)
		keys, err := GetInstalledPGPKeys(kids)
		assert.NoError(t, err)
		assert.Len(t, keys, 1)
		assert.Equal(t, kids[0], keys[0].Fingerprint)
		assert.Equal(t, "ultimate", keys[0].Trust)
	}

	// Import garbage - error expected
	keys, err = ImportPGPKeys("testdata/garbage.asc")
	assert.Error(t, err)
	assert.Len(t, keys, 0)

	// We should still have a total of 2 keys in the keyring now
	{
		keys, err := GetInstalledPGPKeys(nil)
		assert.NoError(t, err)
		assert.Len(t, keys, 2)
	}

	// Delete previously imported public key
	{
		err := DeletePGPKey(importedKeyId)
		assert.NoError(t, err)
		keys, err := GetInstalledPGPKeys(nil)
		assert.NoError(t, err)
		assert.Len(t, keys, 1)
	}

	// Delete non-existing key
	{
		err := DeletePGPKey(importedKeyId)
		assert.Error(t, err)
	}

	// Import multiple keys
	{
		keys, err := ImportPGPKeys("testdata/multi.asc")
		assert.NoError(t, err)
		assert.Len(t, keys, 2)
		assert.Contains(t, keys[0].Owner, "john.doe@example.com")
		assert.Contains(t, keys[1].Owner, "jane.doe@example.com")
	}

	// Check if they were really imported
	{
		keys, err := GetInstalledPGPKeys(nil)
		assert.NoError(t, err)
		assert.Len(t, keys, 3)
	}

}

func Test_GPG_ParseGitCommitVerification(t *testing.T) {
	p := initTempDir()
	defer os.RemoveAll(p)

	err := InitializeGnuPG()
	assert.NoError(t, err)

	keys, err := ImportPGPKeys("testdata/github.asc")
	assert.NoError(t, err)
	assert.Len(t, keys, 1)

	// Good case
	{
		c, err := ioutil.ReadFile("testdata/good_signature.txt")
		if err != nil {
			panic(err.Error())
		}
		res, err := ParseGitCommitVerification(string(c))
		assert.NoError(t, err)
		assert.Equal(t, "4AEE18F83AFDEB23", res.KeyID)
		assert.Equal(t, "RSA", res.Cipher)
		assert.Equal(t, "ultimate", res.Trust)
		assert.Equal(t, "Wed Feb 26 23:22:34 2020 CET", res.Date)
		assert.Equal(t, "Good", res.Result)
	}

	// Bad case: Incomplete signature data #1
	{
		c, err := ioutil.ReadFile("testdata/bad_signature_preeof1.txt")
		if err != nil {
			panic(err.Error())
		}
		_, err = ParseGitCommitVerification(string(c))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "end-of-file")
	}

	// Bad case: Incomplete signature data #2
	{
		c, err := ioutil.ReadFile("testdata/bad_signature_preeof2.txt")
		if err != nil {
			panic(err.Error())
		}
		_, err = ParseGitCommitVerification(string(c))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "end-of-file")
	}

	// Bad case: No signature data #1
	{
		c, err := ioutil.ReadFile("testdata/bad_signature_nodata.txt")
		if err != nil {
			panic(err.Error())
		}
		_, err = ParseGitCommitVerification(string(c))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no verification data found")
	}

	// Bad case: Malformed signature data #1
	{
		c, err := ioutil.ReadFile("testdata/bad_signature_malformed1.txt")
		if err != nil {
			panic(err.Error())
		}
		_, err = ParseGitCommitVerification(string(c))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no verification data found")
	}

	// Bad case: Malformed signature data #2
	{
		c, err := ioutil.ReadFile("testdata/bad_signature_malformed2.txt")
		if err != nil {
			panic(err.Error())
		}
		_, err = ParseGitCommitVerification(string(c))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Could not parse key ID")
	}

	// Bad case: Malformed signature data #3
	{
		c, err := ioutil.ReadFile("testdata/bad_signature_malformed3.txt")
		if err != nil {
			panic(err.Error())
		}
		_, err = ParseGitCommitVerification(string(c))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Could not parse result of verify")
	}
}

func Test_GetGnuPGHomePath(t *testing.T) {
	{
		os.Setenv("GNUPGHOME", "")
		p := GetGnuPGHomePath()
		assert.Equal(t, DefaultGnuPgHomePath, p)
	}
	{
		os.Setenv("GNUPGHOME", "/tmp/gpghome")
		p := GetGnuPGHomePath()
		assert.Equal(t, "/tmp/gpghome", p)
	}
}
